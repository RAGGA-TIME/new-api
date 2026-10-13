package tokenhub

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
)

// Adaptor implements the TokenHub (腾讯云大模型服务平台) sync relay surface:
// chat completions, embeddings, responses, sync image generation and TTS.
// Async generation (video / 3D / async image) is implemented by TaskAdaptor.
//
// Docs: https://cloud.tencent.com/document/product/1823/130079
type Adaptor struct{}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	openaiRequest, err := service.GeminiToOpenAIRequest(request, info)
	if err != nil {
		return nil, err
	}
	return a.ConvertOpenAIRequest(c, info, openaiRequest)
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	aiRequest, err := service.ClaudeToOpenAIRequest(*request, info)
	if err != nil {
		return nil, err
	}
	if info.SupportStreamOptions && info.IsStream {
		aiRequest.StreamOptions = &dto.StreamOptions{
			IncludeUsage: true,
		}
	}
	return a.ConvertOpenAIRequest(c, info, aiRequest)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	if info.RelayMode != relayconstant.RelayModeAudioSpeech {
		return nil, errors.New("unsupported audio relay mode")
	}
	if !IsTTSModel(info.UpstreamModelName) {
		return nil, fmt.Errorf("model %s is not a TokenHub TTS model", info.UpstreamModelName)
	}

	outputFormat := request.ResponseFormat
	// TokenHub MiniMax TTS only supports non-streaming sync synthesis; audio
	// always comes back hex-encoded in JSON. Force hex and decode for clients.
	if outputFormat == "" {
		outputFormat = "mp3"
	}

	speed := lo.FromPtrOr(request.Speed, 1.0)
	ttsReq := SyncTTSRequest{
		Model: info.UpstreamModelName,
		Text:  request.Input,
		VoiceSetting: &TTSVoiceSetting{
			VoiceID: request.Voice,
			Speed:   speed,
		},
		AudioSetting: &TTSAudioSetting{
			Format: outputFormat,
		},
		OutputFormat: "hex",
	}

	// Merge vendor-specific extras supplied via metadata.
	if len(request.Metadata) > 0 {
		if err := common.Unmarshal(request.Metadata, &ttsReq); err != nil {
			return nil, fmt.Errorf("error unmarshalling metadata to tokenhub tts request: %w", err)
		}
	}
	// These fields are gateway-level and must not be overridden by metadata.
	ttsReq.Model = info.UpstreamModelName
	ttsReq.OutputFormat = "hex"
	if ttsReq.VoiceSetting == nil {
		ttsReq.VoiceSetting = &TTSVoiceSetting{VoiceID: request.Voice, Speed: speed}
	} else if ttsReq.VoiceSetting.VoiceID == "" {
		ttsReq.VoiceSetting.VoiceID = request.Voice
	}
	if ttsReq.AudioSetting != nil && ttsReq.AudioSetting.Format == "" {
		ttsReq.AudioSetting.Format = outputFormat
	}

	data, err := common.Marshal(ttsReq)
	if err != nil {
		return nil, fmt.Errorf("error marshalling tokenhub tts request: %w", err)
	}
	return bytes.NewReader(data), nil
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	if info.RelayMode != relayconstant.RelayModeImagesGenerations && info.RelayMode != relayconstant.RelayModeImagesEdits {
		return nil, fmt.Errorf("unsupported image relay mode: %d", info.RelayMode)
	}
	if IsAsyncImageModel(info.UpstreamModelName) {
		return nil, fmt.Errorf("model %s is asynchronous on TokenHub, use POST /v1/images/generations/async instead", info.UpstreamModelName)
	}
	if !IsImageModel(info.UpstreamModelName) {
		return nil, fmt.Errorf("model %s is not a TokenHub sync image model", info.UpstreamModelName)
	}

	// Collect reference images (OpenAI image edits / gpt-image style inputs).
	images := imageRequestImageList(&request)

	size := NormalizeImageSize(request.Size)
	responseFormat := request.ResponseFormat
	if responseFormat == "" {
		responseFormat = "url"
	}

	switch {
	case info.UpstreamModelName == "hy-image-v3":
		req := &HyImageV3Request{
			Model:  info.UpstreamModelName,
			Prompt: request.Prompt,
			Images: images,
			Size:   size,
		}
		if request.Extra != nil {
			if v, ok := request.Extra["seed"]; ok {
				var seed int64
				if err := common.Unmarshal(v, &seed); err == nil {
					req.Seed = seed
				}
			}
			if v, ok := request.Extra["footnote"]; ok {
				var footnote string
				if err := common.Unmarshal(v, &footnote); err == nil {
					req.Footnote = footnote
				}
			}
			if v, ok := request.Extra["revise"]; ok {
				var revise bool
				if err := common.Unmarshal(v, &revise); err == nil {
					req.Revise = &revise
				}
			}
		}
		return req, nil

	case info.UpstreamModelName == "hy-image-v3.5-preview":
		content := make([]HyImageV35Content, 0, len(images)+1)
		if request.Prompt != "" {
			content = append(content, HyImageV35Content{Type: "text", Text: request.Prompt})
		}
		for _, img := range images {
			content = append(content, HyImageV35Content{Type: "image_url", ImageURL: &MediaURL{URL: img}})
		}
		req := &HyImageV35Request{
			Model: info.UpstreamModelName,
			Messages: []HyImageV35Message{
				{Role: "user", Content: content},
			},
			Size: size,
		}
		if request.Extra != nil {
			if v, ok := request.Extra["seed"]; ok {
				var seed int64
				if err := common.Unmarshal(v, &seed); err == nil {
					req.Seed = seed
				}
			}
			if v, ok := request.Extra["footnote"]; ok {
				var footnote string
				if err := common.Unmarshal(v, &footnote); err == nil {
					req.Footnote = footnote
				}
			}
			if v, ok := request.Extra["session"]; ok {
				var session string
				if err := common.Unmarshal(v, &session); err == nil {
					req.Session = session
				}
			}
		}
		return req, nil

	case strings.HasPrefix(info.UpstreamModelName, "seedream-image-"):
		req := &SeedreamImageRequest{
			Model:          info.UpstreamModelName,
			Prompt:         request.Prompt,
			Images:         images,
			Size:           size,
			ResponseFormat: responseFormat,
		}
		if request.Extra != nil {
			if v, ok := request.Extra["watermark"]; ok {
				var watermark bool
				if err := common.Unmarshal(v, &watermark); err == nil {
					req.Watermark = &watermark
				}
			}
			if v, ok := request.Extra["output_format"]; ok {
				var outputFormat string
				if err := common.Unmarshal(v, &outputFormat); err == nil {
					req.OutputFormat = outputFormat
				}
			}
		}
		return req, nil

	default:
		// WAND Vega image & Kling image models share the generic OpenAI-like body.
		return &GenericImageRequest{
			Model:          info.UpstreamModelName,
			Prompt:         request.Prompt,
			Images:         images,
			Size:           size,
			ResponseFormat: responseFormat,
		}, nil
	}
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := strings.TrimRight(info.ChannelBaseUrl, "/")
	if base == "" {
		return "", errors.New("tokenhub base url is empty")
	}
	switch info.RelayMode {
	case relayconstant.RelayModeChatCompletions:
		return base + PathChatCompletions, nil
	case relayconstant.RelayModeCompletions:
		return base + PathCompletions, nil
	case relayconstant.RelayModeEmbeddings:
		return base + PathEmbeddings, nil
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		return base + PathResponses, nil
	case relayconstant.RelayModeAudioSpeech:
		return base + PathMiniMaxSyncTTS, nil
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		return base + syncImageRelayPath(info.UpstreamModelName, len(imageRequestImages(info))), nil
	default:
		return "", fmt.Errorf("unsupported relay mode: %d", info.RelayMode)
	}
}

// syncImageRelayPath resolves the sync image endpoint for a given model.
func syncImageRelayPath(model string, imageCount int) string {
	switch {
	case model == "hy-image-v3":
		return PathHyImageV3Generation
	case model == "hy-image-v3.5-preview":
		return PathHyImageV35Generation
	case strings.HasPrefix(model, "seedream-image-"):
		return PathSeedreamImageGenerator
	case strings.HasPrefix(model, "kling-image-"):
		if imageCount > 0 {
			return PathKlingImageToImage
		}
		return PathKlingTextToImage
	default:
		return PathWandImageGeneration
	}
}

// imageRequestImages extracts reference image inputs from the parsed request.
func imageRequestImages(info *relaycommon.RelayInfo) []string {
	req, ok := info.Request.(*dto.ImageRequest)
	if !ok {
		return nil
	}
	return imageRequestImageList(req)
}

// imageRequestImageList flattens the image field (string or string array)
// plus optional "images" extra field into a URL/base64 list.
func imageRequestImageList(req *dto.ImageRequest) []string {
	var images []string
	appendImage := func(raw json.RawMessage) {
		var single string
		if err := common.Unmarshal(raw, &single); err == nil {
			if strings.TrimSpace(single) != "" {
				images = append(images, strings.TrimSpace(single))
			}
			return
		}
		var list []string
		if err := common.Unmarshal(raw, &list); err == nil {
			for _, img := range list {
				if strings.TrimSpace(img) != "" {
					images = append(images, strings.TrimSpace(img))
				}
			}
		}
	}
	if len(req.Image) > 0 {
		appendImage(req.Image)
	}
	if raw, ok := req.Extra["images"]; ok {
		appendImage(raw)
	}
	return images
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	return request, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, errors.New("tokenhub does not support rerank")
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	return request, nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeAudioSpeech:
		return handleTTSResponse(c, resp, info)
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		return handleImageResponse(c, resp, info)
	default:
		openaiAdaptor := openai.Adaptor{}
		return openaiAdaptor.DoResponse(c, resp, info)
	}
}

func (a *Adaptor) GetModelList() []string {
	return AllModels()
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
