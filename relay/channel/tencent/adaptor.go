package tencent

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/relay/channel"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

type Adaptor struct {
	Sign      string
	AppID     int64
	Action    string
	Version   string
	Timestamp int64
	SecretId  string
	SecretKey string
}

func (a *Adaptor) ConvertGeminiRequest(*gin.Context, *relaycommon.RelayInfo, *dto.GeminiChatRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertClaudeRequest(*gin.Context, *relaycommon.RelayInfo, *dto.ClaudeRequest) (any, error) {
	//TODO implement me
	panic("implement me")
	return nil, nil
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	// 解析密钥
	apiKey := common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	appId, secretId, secretKey, err := parseTencentConfig(apiKey)
	if err != nil {
		return nil, err
	}
	a.AppID = appId
	a.SecretId = secretId
	a.SecretKey = secretKey

	tencentReq := &TextToImageLiteRequest{
		Prompt: request.Prompt,
	}

	// 处理 ResponseFormat -> RspImgType
	// OpenAI: "url" 或 "b64_json" -> 混元: "url" 或 "base64"
	if request.ResponseFormat == "url" {
		tencentReq.RspImgType = "url"
	} else {
		tencentReq.RspImgType = "base64" // 默认
	}

	// 从 Extra 字段读取额外参数
	if request.Extra != nil {
		if v, ok := request.Extra["negative_prompt"]; ok {
			var negativePrompt string
			if err := common.Unmarshal(v, &negativePrompt); err == nil {
				tencentReq.NegativePrompt = negativePrompt
			}
		}
		if v, ok := request.Extra["style"]; ok {
			var style string
			if err := common.Unmarshal(v, &style); err == nil {
				tencentReq.Style = style
			}
		}
		if v, ok := request.Extra["logo_param"]; ok {
			var logoParam LogoParam
			if err := common.Unmarshal(v, &logoParam); err == nil {
				tencentReq.LogoParam = &logoParam
			}
		}
	}

	// 计算签名
	a.Sign = a.getTencentSignForImage(tencentReq, secretId, secretKey)

	return tencentReq, nil
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.Timestamp = common.GetTimestamp()
	a.Version = "2023-09-01"

	// 根据 RelayMode 设置 Action
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations:
		a.Action = "TextToImageLite"
	default:
		a.Action = "ChatCompletions"
	}
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/", info.ChannelBaseUrl), nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, req *http.Header, info *relaycommon.RelayInfo) error {
	channel.SetupApiRequestHeader(info, c, req)
	req.Set("Authorization", a.Sign)
	req.Set("X-TC-Action", a.Action)
	req.Set("X-TC-Version", a.Version)
	req.Set("X-TC-Timestamp", strconv.FormatInt(a.Timestamp, 10))
	// 混元生图仅支持广州地域
	if info.RelayMode == relayconstant.RelayModeImagesGenerations {
		req.Set("X-TC-Region", "ap-guangzhou")
	}
	return nil
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	if request == nil {
		return nil, errors.New("request is nil")
	}
	apiKey := common.GetContextKeyString(c, constant.ContextKeyChannelKey)
	apiKey = strings.TrimPrefix(apiKey, "Bearer ")
	appId, secretId, secretKey, err := parseTencentConfig(apiKey)
	a.AppID = appId
	if err != nil {
		return nil, err
	}
	tencentRequest := requestOpenAI2Tencent(a, *request)
	// we have to calculate the sign here
	a.Sign = getTencentSign(*tencentRequest, a, secretId, secretKey)
	return tencentRequest, nil
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	return nil, nil
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	//TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	// TODO implement me
	return nil, errors.New("not implemented")
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations:
		usage, err = tencentImageHandler(c, resp, info)
	default:
		if info.IsStream {
			usage, err = tencentStreamHandler(c, info, resp)
		} else {
			usage, err = tencentHandler(c, info, resp)
		}
	}
	return
}

func (a *Adaptor) GetModelList() []string {
	return ModelList
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}
