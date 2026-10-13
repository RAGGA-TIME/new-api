package tokenhub

import (
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// ============================
// Image response handling
// ============================

func handleImageResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("failed to read tokenhub response: %w", readErr), types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("tokenhub image error, status: %d, body: %s", resp.StatusCode, string(body)), types.ErrorCodeBadResponse, resp.StatusCode)
	}

	// Unified failure shape: {"error": {...}}
	var unified GenericImageResponse
	if err := common.Unmarshal(body, &unified); err != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("failed to unmarshal tokenhub image response: %w, body: %s", err, string(body)), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if unified.Error != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("%s", unified.Error.ErrorMessage()), types.ErrorCodeBadResponse, http.StatusBadRequest)
	}

	switch {
	case info.UpstreamModelName == "hy-image-v3.5-preview":
		return convertHyImageV35Response(c, body, info)
	default:
		// hy-image-v3 / seedream / wand / kling all reply with data[].url|b64_json.
		return convertGenericImageResponse(c, &unified, info)
	}
}

func convertGenericImageResponse(c *gin.Context, unified *GenericImageResponse, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if len(unified.Data) == 0 {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("tokenhub image response contains no data"), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	imageResp := dto.ImageResponse{
		Created: unified.Created,
		Data:    make([]dto.ImageData, 0, len(unified.Data)),
	}
	for _, item := range unified.Data {
		data := dto.ImageData{
			Url:           item.URL,
			B64Json:       item.B64Json,
			RevisedPrompt: item.RevisedPrompt,
		}
		imageResp.Data = append(imageResp.Data, data)
	}
	usage := &dto.Usage{
		PromptTokens:     info.GetEstimatePromptTokens(),
		CompletionTokens: 0,
		TotalTokens:      int(unified.TokenhubUsage.TotalTokens),
	}
	c.JSON(http.StatusOK, imageResp)
	return usage, nil
}

func convertHyImageV35Response(c *gin.Context, body []byte, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	var v35 HyImageV35Response
	if err := common.Unmarshal(body, &v35); err != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("failed to unmarshal hy-image-v3.5 response: %w, body: %s", err, string(body)), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}
	if v35.Error != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("%s", v35.Error.ErrorMessage()), types.ErrorCodeBadResponse, http.StatusBadRequest)
	}

	imageResp := dto.ImageResponse{
		Created: v35.Created,
		Data:    make([]dto.ImageData, 0, 1),
	}
	for _, choice := range v35.Choices {
		if choice.Delta.Image != nil && choice.Delta.Image.URL != "" {
			imageResp.Data = append(imageResp.Data, dto.ImageData{Url: choice.Delta.Image.URL})
		}
	}
	if len(imageResp.Data) == 0 {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("hy-image-v3.5 response contains no image url, body: %s", string(body)), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}
	usage := &dto.Usage{
		PromptTokens:     info.GetEstimatePromptTokens(),
		CompletionTokens: 0,
		TotalTokens:      int(v35.TokenhubUsage.TotalTokens),
	}
	c.JSON(http.StatusOK, imageResp)
	return usage, nil
}

// ============================
// TTS response handling
// ============================

func handleTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	body, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("failed to read tokenhub tts response: %w", readErr), types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	defer resp.Body.Close()

	var ttsResp SyncTTSResponse
	if unmarshalErr := common.Unmarshal(body, &ttsResp); unmarshalErr != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("failed to unmarshal tokenhub tts response: %w, body: %s", unmarshalErr, string(body)), types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	if ttsResp.BaseResp != nil && ttsResp.BaseResp.StatusCode != 0 {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("tokenhub tts error: %d - %s", ttsResp.BaseResp.StatusCode, ttsResp.BaseResp.StatusMsg), types.ErrorCodeBadResponse, http.StatusBadRequest)
	}
	if ttsResp.Data == nil || ttsResp.Data.Audio == "" {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("no audio data in tokenhub tts response"), types.ErrorCodeBadResponse, http.StatusBadRequest)
	}

	audioHex := strings.TrimSpace(ttsResp.Data.Audio)
	audioData, decodeErr := hex.DecodeString(audioHex)
	if decodeErr != nil {
		return nil, types.NewErrorWithStatusCode(fmt.Errorf("failed to decode tokenhub hex audio data: %w", decodeErr), types.ErrorCodeBadResponse, http.StatusInternalServerError)
	}

	contentType := "audio/mpeg"
	if ttsResp.ExtraInfo != nil && ttsResp.ExtraInfo.AudioFormat != "" {
		contentType = audioFormatContentType(ttsResp.ExtraInfo.AudioFormat)
	}
	c.Data(http.StatusOK, contentType, audioData)

	totalTokens := 0
	if ttsResp.Usage != nil {
		totalTokens = ttsResp.Usage.TotalToken
	} else if ttsResp.ExtraInfo != nil {
		totalTokens = ttsResp.ExtraInfo.UsageCharacters
	}
	usage = &dto.Usage{
		PromptTokens:     info.GetEstimatePromptTokens(),
		CompletionTokens: 0,
		TotalTokens:      totalTokens,
	}
	return usage, nil
}

func audioFormatContentType(format string) string {
	switch strings.ToLower(format) {
	case "wav":
		return "audio/wav"
	case "flac":
		return "audio/flac"
	case "opus":
		return "audio/opus"
	case "pcm", "pcmu_raw":
		return "audio/pcm"
	case "pcmu_wav":
		return "audio/wav"
	default:
		return "audio/mpeg"
	}
}
