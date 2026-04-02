package tencent

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// ============================
// Request / Response structures
// ============================

// SubmitVideoRequest 提交视频生成任务请求
// https://cloud.tencent.com/document/api/1616/126160
type SubmitVideoRequest struct {
	Prompt     string `json:"Prompt"`
	ImageUrl   string `json:"ImageUrl,omitempty"`
	Resolution string `json:"Resolution,omitempty"` // 分辨率 720p/1080p
}

// SubmitVideoResponse 提交视频生成任务响应
type SubmitVideoResponse struct {
	JobId     string `json:"JobId"`
	RequestId string `json:"RequestId"`
	Code      int    `json:"Code,omitempty"`
	Message   string `json:"Message,omitempty"`
}

// TencentError 腾讯云 API 错误响应
type TencentError struct {
	Code    string `json:"Code"`    // 错误码，如 "FailedOperation.JobNotExist"
	Message string `json:"Message"` // 错误信息
}

// QueryVideoResponse 查询视频生成任务响应
// https://cloud.tencent.com/document/api/1616/126161
type QueryVideoResponse struct {
	Error          *TencentError `json:"Error,omitempty"`  // API 错误（优先检查）
	Status         string        `json:"Status"`           // WAIT | RUN | FAIL | DONE
	ResultVideoUrl string        `json:"ResultVideoUrl"`   // 视频下载链接
	ErrorCode      string        `json:"ErrorCode"`        // 任务级错误码（空字符串表示无错误）
	ErrorMessage   string        `json:"ErrorMessage"`     // 任务级错误描述
	RequestId      string        `json:"RequestId"`
}

// ResponseWrapper 腾讯云 API 响应包装结构
type ResponseWrapper struct {
	Response any `json:"Response"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	Action    string
	Version   string
	Timestamp int64
	SecretId  string
	SecretKey string
	baseURL   string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.Action = "SubmitHunyuanToVideoJob"
	a.Version = "2024-05-23"
	a.Timestamp = common.GetTimestamp()
	a.baseURL = info.ChannelBaseUrl

	// 解析密钥: AppID|SecretId|SecretKey
	parts := strings.Split(info.ApiKey, "|")
	if len(parts) >= 3 {
		a.SecretId = parts[1]
		a.SecretKey = parts[2]
	} else if len(parts) == 2 {
		// 兼容 SecretId|SecretKey 格式
		a.SecretId = parts[0]
		a.SecretKey = parts[1]
	}
}

// CheckConcurrency 检查是否超过并发限制
// 返回 true 表示可以提交，false 表示需要等待
// 混元视频并发数限制为 1
func (a *TaskAdaptor) CheckConcurrency(info *relaycommon.RelayInfo) bool {
	runningCount := model.GetHunyuanRunningTaskCount(info.ChannelId)
	return runningCount < 1 // 并发数限制为 1
}

// EstimateBilling 估算计费参数
// 混元视频：720p 为刊例价，1080p 在 720p 价格上打 5 折
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	v, ok := c.Get("task_request")
	if !ok {
		return nil
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil
	}

	// 解析分辨率（优先 Resolution，其次 Size）
	resolution := parseHunyuanResolution(req.Resolution, req.Size, req.Metadata)
	resRatio := hunyuanResolutionRatio(resolution)

	return map[string]float64{
		"resolution": resRatio,
	}
}

// parseHunyuanResolution 解析混元视频分辨率
// 支持格式: "720p", "1080p", "1280x720", "1920x1080" 等
// 优先级: Resolution > metadata.resolution > Size > 默认720p
func parseHunyuanResolution(resolution, size string, metadata map[string]interface{}) string {
	// 优先使用 Resolution 字段
	if resolution != "" {
		return normalizeResolution(resolution)
	}

	// 从 metadata 获取
	if metadata != nil {
		if res, ok := metadata["resolution"].(string); ok && res != "" {
			return normalizeResolution(res)
		}
	}

	// 从 size 字段获取（兼容旧参数）
	if size != "" {
		return normalizeResolution(size)
	}

	// 默认 720p
	return "720p"
}

// normalizeResolution 标准化分辨率字符串
func normalizeResolution(size string) string {
	size = strings.ToLower(strings.TrimSpace(size))

	// 直接匹配
	switch size {
	case "720p", "1280x720", "720":
		return "720p"
	case "1080p", "1920x1080", "1080":
		return "1080p"
	}

	// 尝试解析宽x高格式
	if strings.Contains(size, "x") {
		parts := strings.Split(size, "x")
		if len(parts) == 2 {
			width := 0
			if _, err := fmt.Sscanf(parts[0], "%d", &width); err == nil {
				if width >= 1920 {
					return "1080p"
				}
			}
		}
	}

	// 默认 720p
	return "720p"
}

// hunyuanResolutionRatio 获取分辨率价格比率
// 720p = 1.0 (刊例价), 1080p = 2.0 (2倍价格)
func hunyuanResolutionRatio(resolution string) float64 {
	switch resolution {
	case "1080p":
		return 2.0 // 1080p 是 720p 的 2 倍价格
	default:
		return 1.0 // 720p 为刊例价
	}
}

// ValidateRequestAndSetAction 解析请求体并设置默认 action
func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

// BuildRequestURL 构建请求 URL
func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	return a.baseURL, nil
}

// BuildRequestHeader 设置请求头，包括腾讯云签名
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", "vclm.tencentcloudapi.com")
	req.Header.Set("X-TC-Action", a.Action)
	req.Header.Set("X-TC-Version", a.Version)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(a.Timestamp, 10))
	req.Header.Set("X-TC-Region", "ap-guangzhou")

	// 计算签名
	authorization := a.getTencentSign(req, a.SecretId, a.SecretKey)
	req.Header.Set("Authorization", authorization)

	return nil
}

// BuildRequestBody 构建请求体
func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	v, exists := c.Get("task_request")
	if !exists {
		return nil, fmt.Errorf("request not found in context")
	}
	req, ok := v.(relaycommon.TaskSubmitReq)
	if !ok {
		return nil, fmt.Errorf("invalid request type in context")
	}

	// 解析分辨率
	resolution := parseHunyuanResolution(req.Resolution, req.Size, req.Metadata)

	// 调试日志：输出解析后的分辨率
	common.SysLog(fmt.Sprintf("HunyuanVideo BuildRequestBody: req.Resolution=%s, req.Size=%s, parsed resolution=%s",
		req.Resolution, req.Size, resolution))

	// 构建腾讯混元视频生成请求
	tencentReq := SubmitVideoRequest{
		Prompt:     req.Prompt,
		Resolution: resolution,
	}

	// 处理图片 URL（图生视频）
	if req.HasImage() {
		if strings.HasPrefix(req.Images[0], "http") {
			tencentReq.ImageUrl = req.Images[0]
		}
	}

	body, err := common.Marshal(tencentReq)
	if err != nil {
		return nil, err
	}

	// 调试日志：输出最终请求体
	common.SysLog(fmt.Sprintf("HunyuanVideo Request Body: %s", string(body)))

	return bytes.NewReader(body), nil
}

// DoRequest 执行请求
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse 处理响应，返回任务 ID
func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	// 解析腾讯云响应
	var wrapper ResponseWrapper
	if err := common.Unmarshal(responseBody, &wrapper); err != nil {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	// 提取 Response 字段
	respBytes, err := common.Marshal(wrapper.Response)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "marshal_response_failed", http.StatusInternalServerError)
		return
	}

	var submitResp SubmitVideoResponse
	if err := common.Unmarshal(respBytes, &submitResp); err != nil {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("body: %s", responseBody), "unmarshal_submit_response_failed", http.StatusInternalServerError)
		return
	}

	if submitResp.Code != 0 {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("%s", submitResp.Message), fmt.Sprintf("%d", submitResp.Code), http.StatusInternalServerError)
		return
	}

	// 返回 OpenAI 格式的视频任务响应
	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	c.JSON(http.StatusOK, ov)

	return submitResp.JobId, responseBody, nil
}

// FetchTask 查询任务状态
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid task_id")
	}

	// 解析密钥
	parts := strings.Split(key, "|")
	var secretId, secretKey string
	if len(parts) >= 3 {
		secretId = parts[1]
		secretKey = parts[2]
	} else if len(parts) == 2 {
		secretId = parts[0]
		secretKey = parts[1]
	} else {
		return nil, fmt.Errorf("invalid api key format")
	}

	// 构建查询请求
	queryReq := map[string]string{
		"JobId": taskID,
	}
	payloadBytes, err := common.Marshal(queryReq)
	if err != nil {
		return nil, fmt.Errorf("marshal query request failed: %w", err)
	}

	// 使用固定的 URL，确保正确
	apiUrl := "https://vclm.tencentcloudapi.com"
	req, err := http.NewRequest(http.MethodPost, apiUrl, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}

	// 设置请求头
	timestamp := common.GetTimestamp()
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", "vclm.tencentcloudapi.com")
	req.Header.Set("X-TC-Action", "DescribeHunyuanToVideoJob")
	req.Header.Set("X-TC-Version", "2024-05-23")
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Region", "ap-guangzhou")

	// 计算签名
	authorization := a.getTencentSignForQuery(req, secretId, secretKey, timestamp)
	req.Header.Set("Authorization", authorization)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

// ParseTaskResult 解析任务结果
func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var wrapper ResponseWrapper
	if err := common.Unmarshal(respBody, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal response wrapper failed: %w", err)
	}

	respBytes, err := common.Marshal(wrapper.Response)
	if err != nil {
		return nil, fmt.Errorf("marshal response failed: %w", err)
	}

	var queryResp QueryVideoResponse
	if err := common.Unmarshal(respBytes, &queryResp); err != nil {
		return nil, fmt.Errorf("unmarshal query response failed: %w", err)
	}

	taskResult := &relaycommon.TaskInfo{}

	// 优先检查 API 级错误（Response.Error）
	if queryResp.Error != nil {
		taskResult.Status = model.TaskStatusFailure
		taskResult.Code = 500
		taskResult.Reason = fmt.Sprintf("%s: %s", queryResp.Error.Code, queryResp.Error.Message)
		taskResult.Progress = "100%"
		return taskResult, nil
	}

	// 检查任务级错误（空字符串表示无错误）
	if queryResp.ErrorCode != "" || queryResp.ErrorMessage != "" {
		taskResult.Status = model.TaskStatusFailure
		taskResult.Code = 500
		taskResult.Reason = queryResp.ErrorMessage
		taskResult.Progress = "100%"
		return taskResult, nil
	}

	// 解析状态: WAIT、RUN、FAIL、DONE
	switch queryResp.Status {
	case "RUN":
		taskResult.Status = model.TaskStatusInProgress
		taskResult.Progress = "50%"
	case "DONE":
		taskResult.Status = model.TaskStatusSuccess
		taskResult.Progress = "100%"
		taskResult.Url = queryResp.ResultVideoUrl
	case "FAIL":
		taskResult.Status = model.TaskStatusFailure
		taskResult.Reason = queryResp.ErrorMessage
		taskResult.Progress = "100%"
	case "WAIT":
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	default:
		taskResult.Status = model.TaskStatusQueued
		taskResult.Progress = "10%"
	}

	return taskResult, nil
}

// GetModelList 返回模型列表
func (a *TaskAdaptor) GetModelList() []string {
	return []string{"HY-Video"}
}

// GetChannelName 返回渠道名称
func (a *TaskAdaptor) GetChannelName() string {
	return "HunyuanVideo"
}

// ConvertToOpenAIVideo 转换为 OpenAI 视频格式响应
func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	var wrapper ResponseWrapper
	if err := common.Unmarshal(originTask.Data, &wrapper); err != nil {
		return nil, fmt.Errorf("unmarshal origin task data failed: %w", err)
	}

	respBytes, err := common.Marshal(wrapper.Response)
	if err != nil {
		return nil, fmt.Errorf("marshal response failed: %w", err)
	}

	var queryResp QueryVideoResponse
	if err := common.Unmarshal(respBytes, &queryResp); err != nil {
		return nil, fmt.Errorf("unmarshal query response failed: %w", err)
	}

	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.SetMetadata("url", queryResp.ResultVideoUrl)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt

	// 处理错误信息
	if queryResp.Error != nil {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Code:    queryResp.Error.Code,
			Message: queryResp.Error.Message,
		}
	} else if queryResp.ErrorCode != "" || queryResp.ErrorMessage != "" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Code:    queryResp.ErrorCode,
			Message: queryResp.ErrorMessage,
		}
	}

	return common.Marshal(openAIVideo)
}

// ============================
// Tencent Signature
// ============================

// sha256hex 计算 SHA256 哈希并返回十六进制字符串
func sha256hex(s string) string {
	b := sha256.Sum256([]byte(s))
	return hex.EncodeToString(b[:])
}

// hmacSha256 计算 HMAC-SHA256
func hmacSha256(key, data string) string {
	hashed := hmac.New(sha256.New, []byte(key))
	hashed.Write([]byte(data))
	return string(hashed.Sum(nil))
}

// getTencentSign 计算腾讯云 TC3-HMAC-SHA256 签名
func (a *TaskAdaptor) getTencentSign(req *http.Request, secretId, secretKey string) string {
	host := "vclm.tencentcloudapi.com"
	service := "vclm"
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	action := a.Action

	contentType := "application/json; charset=utf-8"
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		contentType, host, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"

	// 读取请求体 - 从 GetBody 获取副本，不消耗原始请求体
	var bodyBytes []byte
	if req.GetBody != nil {
		bodyReader, err := req.GetBody()
		if err == nil && bodyReader != nil {
			bodyBytes, _ = io.ReadAll(bodyReader)
			_ = bodyReader.Close()
		}
	}
	payload := string(bodyBytes)
	hashedRequestPayload := sha256hex(payload)

	// 重要：恢复请求体，确保后续请求发送时可以正常读取
	if len(bodyBytes) > 0 {
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload)

	// 构建待签名字符串
	algorithm := "TC3-HMAC-SHA256"
	requestTimestamp := strconv.FormatInt(a.Timestamp, 10)
	t := time.Unix(a.Timestamp, 0).UTC()
	date := t.Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		requestTimestamp,
		credentialScope,
		hashedCanonicalRequest)

	// 计算签名 - 注意: hmacSha256(key, data) 密钥在前，数据在后
	// 官方: hmacsha256(data, key) 数据在前，密钥在后
	// 所以调用时参数顺序需要交换
	secretDate := hmacSha256("TC3"+secretKey, date)
	secretService := hmacSha256(secretDate, service)
	secretSigning := hmacSha256(secretService, "tc3_request")
	signature := hex.EncodeToString([]byte(hmacSha256(secretSigning, string2sign)))

	// 构建 Authorization 头
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		secretId,
		credentialScope,
		signedHeaders,
		signature)

	return authorization
}

// getTencentSignForQuery 查询任务时的签名计算
func (a *TaskAdaptor) getTencentSignForQuery(req *http.Request, secretId, secretKey string, timestamp int64) string {
	host := "vclm.tencentcloudapi.com"
	service := "vclm"
	httpRequestMethod := "POST"
	canonicalURI := "/"
	canonicalQueryString := ""
	action := "DescribeHunyuanToVideoJob"
	canonicalHeaders := fmt.Sprintf("content-type:%s\nhost:%s\nx-tc-action:%s\n",
		"application/json; charset=utf-8", host, strings.ToLower(action))
	signedHeaders := "content-type;host;x-tc-action"

	// 读取请求体
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
		req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
	}
	hashedRequestPayload := sha256hex(string(bodyBytes))

	canonicalRequest := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s",
		httpRequestMethod,
		canonicalURI,
		canonicalQueryString,
		canonicalHeaders,
		signedHeaders,
		hashedRequestPayload)

	// 构建待签名字符串
	algorithm := "TC3-HMAC-SHA256"
	requestTimestamp := strconv.FormatInt(timestamp, 10)
	t := time.Unix(timestamp, 0).UTC()
	date := t.Format("2006-01-02")
	credentialScope := fmt.Sprintf("%s/%s/tc3_request", date, service)
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	string2sign := fmt.Sprintf("%s\n%s\n%s\n%s",
		algorithm,
		requestTimestamp,
		credentialScope,
		hashedCanonicalRequest)

	// 计算签名 - 注意: hmacSha256(key, data) 密钥在前，数据在后
	// 官方: hmacsha256(data, key) 数据在前，密钥在后
	// 所以调用时参数顺序需要交换
	secretDate := hmacSha256("TC3"+secretKey, date)
	secretService := hmacSha256(secretDate, service)
	secretSigning := hmacSha256(secretService, "tc3_request")
	signature := hex.EncodeToString([]byte(hmacSha256(secretSigning, string2sign)))

	// 构建 Authorization 头
	authorization := fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		algorithm,
		secretId,
		credentialScope,
		signedHeaders,
		signature)

	return authorization
}