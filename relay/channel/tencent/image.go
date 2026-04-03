package tencent

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
)

// getTencentSignForImage 计算图片生成 API 签名
func (a *Adaptor) getTencentSignForImage(req *TextToImageLiteRequest, secretId, secretKey string) string {
	host := "hunyuan.tencentcloudapi.com"
	service := "hunyuan"
	algorithm := "TC3-HMAC-SHA256"

	// 序列化请求体
	payload, _ := json.Marshal(req)

	// 步骤 1: 拼接规范请求串
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		"application/json", host, strings.ToLower(a.Action))
	signedHeaders := "content-type;host;x-tc-action"
	hashedRequestPayload := sha256hex(string(payload))
	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod, canonicalURI, canonicalQueryString,
		canonicalHeaders, signedHeaders, hashedRequestPayload)

	// 步骤 2: 拼接待签名字符串
	date := time.Unix(a.Timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%d\n%s\n%s",
		algorithm, a.Timestamp, credentialScope, hashedCanonicalRequest)

	// 步骤 3: 计算签名
	secretDate := hmacSha256(date, "TC3"+secretKey)
	secretService := hmacSha256(service, secretDate)
	secretSigning := hmacSha256("tc3_request", secretService)
	signature := hex.EncodeToString([]byte(hmacSha256(string2sign, secretSigning)))

	// 步骤 4: 拼接 Authorization
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm, secretId, credentialScope, signedHeaders, signature)

	return authorization
}

// tencentImageHandler 处理混元生图响应
func tencentImageHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*dto.Usage, *types.NewAPIError) {
	var wrapper TextToImageLiteResponseSB
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError)
	}
	service.CloseResponseBodyGracefully(resp)

	if err := common.Unmarshal(responseBody, &wrapper); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusInternalServerError)
	}

	// 检查错误
	if wrapper.Response.Error != nil {
		return nil, types.WithOpenAIError(types.OpenAIError{
			Message: wrapper.Response.Error.Message,
			Type:    wrapper.Response.Error.GetCodeString(),
			Code:    "tencent_image_error",
		}, resp.StatusCode)
	}

	// 转换为 OpenAI 格式
	openAIResp := &dto.ImageResponse{
		Created: info.StartTime.Unix(),
	}

	// ResultImage 可能是 URL 或 Base64
	if wrapper.Response.ResultImage != "" {
		if strings.HasPrefix(wrapper.Response.ResultImage, "http") {
			openAIResp.Data = append(openAIResp.Data, dto.ImageData{Url: wrapper.Response.ResultImage})
		} else {
			openAIResp.Data = append(openAIResp.Data, dto.ImageData{B64Json: wrapper.Response.ResultImage})
		}
	}

	jsonResponse, err := json.Marshal(openAIResp)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	c.Writer.Header().Set("Content-Type", "application/json")
	c.Writer.WriteHeader(resp.StatusCode)
	_, err = c.Writer.Write(jsonResponse)
	if err != nil {
		return nil, types.NewError(err, types.ErrorCodeBadResponseBody)
	}

	return &dto.Usage{}, nil
}