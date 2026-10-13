package tokenhub

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	"github.com/pkg/errors"
)

// ============================
// Task payload structs
// ============================

// ContentItem is the multimodal content element used by MiniMax-V2 / WAND series.
type ContentItem struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *MediaURL `json:"image_url,omitempty"`
	VideoURL *MediaURL `json:"video_url,omitempty"`
	AudioURL *MediaURL `json:"audio_url,omitempty"`
	Role     string    `json:"role,omitempty"`
}

// KlingContent is the contents element for Kling series.
type KlingContent struct {
	Type string    `json:"type"` // prompt / first_frame / last_frame / element / base_video / ...
	Text string    `json:"text,omitempty"`
	URL  *MediaURL `json:"url,omitempty"`
}

type KlingSettings struct {
	Resolution  string `json:"resolution,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	Audio       string `json:"audio,omitempty"`
}

type KlingRequest struct {
	Model    string         `json:"model"`
	Prompt   string         `json:"prompt,omitempty"`
	Contents []KlingContent `json:"contents,omitempty"`
	Settings *KlingSettings `json:"settings,omitempty"`
}

type ViduRequest struct {
	Model        string      `json:"model"`
	Prompt       string      `json:"prompt,omitempty"`
	Images       []string    `json:"images,omitempty"`
	Duration     int         `json:"duration,omitempty"`
	AspectRatio  string      `json:"aspect_ratio,omitempty"`
	Resolution   string      `json:"resolution,omitempty"`
	Audio        *bool       `json:"audio,omitempty"`
	AudioType    string      `json:"audio_type,omitempty"`
	MovementAmpl string      `json:"movement_amplitude,omitempty"`
	Seed         int64       `json:"seed,omitempty"`
	CallbackURL  string      `json:"callback_url,omitempty"`
}

type PixVerseRequest struct {
	Model       string `json:"model"`
	Prompt      string `json:"prompt,omitempty"`
	Duration    int    `json:"duration,omitempty"`
	Quality     string `json:"quality,omitempty"`
	AspectRatio string `json:"aspect_ratio,omitempty"`
	ImgID       string `json:"img_id,omitempty"`
	Seed        int64  `json:"seed,omitempty"`
}

type HyVideoRequest struct {
	Model        string `json:"model"`
	Prompt       string `json:"prompt,omitempty"`
	Image        string `json:"image,omitempty"` // base64, takes priority over image_url
	ImageURL     string `json:"image_url,omitempty"`
	Duration     int    `json:"duration,omitempty"`
	AspectRatio  string `json:"aspect_ratio,omitempty"`
	Resolution   string `json:"resolution,omitempty"`
	Footnote     string `json:"footnote,omitempty"`
}

type MiniMaxV1VideoRequest struct {
	Model            string `json:"model"`
	Prompt           string `json:"prompt,omitempty"`
	FirstFrameImage  string `json:"first_frame_image,omitempty"`
	Duration         int    `json:"duration,omitempty"`
	Resolution       string `json:"resolution,omitempty"`
	PromptOptimizer  *bool  `json:"prompt_optimizer,omitempty"`
	AIGCWatermark    bool   `json:"aigc_watermark,omitempty"`
}

type MiniMaxV2VideoRequest struct {
	Model      string        `json:"model"`
	Content    []ContentItem `json:"content"`
	Resolution string        `json:"resolution,omitempty"`
	Duration   int           `json:"duration,omitempty"`
	Ratio      string        `json:"ratio,omitempty"`
}

type WandVideoRequest struct {
	Model      string        `json:"model"`
	Content    []ContentItem `json:"content"`
	Resolution string        `json:"resolution,omitempty"`
	Ratio      string        `json:"ratio,omitempty"`
	Duration   int           `json:"duration,omitempty"`
}

type YTVideoRequest struct {
	Model string    `json:"model"`
	Image *MediaURL `json:"image,omitempty"`
	Prompt string   `json:"prompt,omitempty"`
}

type Hy3DRequest struct {
	Model           string           `json:"model"`
	Prompt          string           `json:"prompt,omitempty"`
	ImageBase64     string           `json:"image_base64,omitempty"`
	ImageURL        string           `json:"image_url,omitempty"`
	MultiViewImages []Hy3DViewImage  `json:"multi_view_images,omitempty"`
	EnablePBR       *bool            `json:"enable_pbr,omitempty"`
	FaceCount       int              `json:"face_count,omitempty"`
	GenerateType    string           `json:"generate_type,omitempty"`
	PolygonType     string           `json:"polygon_type,omitempty"`
	ResultFormat    string           `json:"result_format,omitempty"`
}

type Hy3DViewImage struct {
	ViewType     string `json:"view_type"`
	ViewImageURL string `json:"view_image_url"`
}

// ViduImageRequest is the body for vidu-image-q2 (async image).
type ViduImageRequest struct {
	Model       string   `json:"model"`
	Prompt      string   `json:"prompt,omitempty"`
	Images      []string `json:"images,omitempty"`
	AspectRatio string   `json:"aspect_ratio,omitempty"`
	Resolution  string   `json:"resolution,omitempty"`
	Seed        int64    `json:"seed,omitempty"`
	CallbackURL string   `json:"callback_url,omitempty"`
}

// ============================
// Upstream response structs
// ============================

type TaskEnvelope struct {
	ID        string          `json:"id,omitempty"`
	TaskID    string          `json:"task_id,omitempty"`
	State     string          `json:"state,omitempty"`
	Status    string          `json:"status,omitempty"`
	Error     *TokenhubError  `json:"error,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Resp      json.RawMessage `json:"Resp,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
}

// ============================
// TaskAdaptor implementation
// ============================

// TaskAdaptor implements the async generation surface of TokenHub:
// video, 3D assets and async image tasks.
//
// Docs:
//   - video overview: https://cloud.tencent.com/document/product/1823/135738
//   - 3D: https://cloud.tencent.com/document/product/1823/137181
//   - async image: https://cloud.tencent.com/document/product/1823/135746
type TaskAdaptor struct {
	taskcommon.BaseBilling
	baseURL    string
	apiKey     string
	submitPath string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if a.baseURL == "" {
		a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	}
	path := a.submitPath
	if path == "" {
		path = taskSubmitPath(info.UpstreamModelName)
	}
	return a.baseURL + path, nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	images := collectImages(&req)
	a.submitPath = taskSubmitPathFor(info.UpstreamModelName, len(images) > 0, len(images))

	body, err := buildTaskSubmitBody(&req, info, images)
	if err != nil {
		return nil, errors.Wrap(err, "convert request payload failed")
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	upstreamID, extractErr := extractSubmitTaskID(responseBody, info.UpstreamModelName)
	if extractErr != nil {
		taskErr = service.TaskErrorWrapper(extractErr, "invalid_response", http.StatusInternalServerError)
		return
	}

	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName
	if ux := jsonAnyFromBytes(responseBody); ux != nil {
		ov.SetMetadata("upstream", ux)
	}
	c.JSON(http.StatusOK, ov)
	return upstreamID, append([]byte(nil), responseBody...), nil
}

// ============================
// Endpoint resolution
// ============================

// taskSubmitPath resolves the async submit endpoint per model family.
func taskSubmitPath(model string) string {
	switch {
	case strings.HasPrefix(model, "hy-3d-"):
		return "/v1/api/3d/submit"
	case strings.HasPrefix(model, "yt-video-"):
		return "/v1/api/video/submit"
	case strings.HasPrefix(model, "hy-video-"):
		return "/v1/wand/hunyuan-video/generation"
	case model == "minimax-video-v2.3" || model == "minimax-video-v2.3-fast":
		return "/v1/wand/minimax-video/generation"
	case strings.HasPrefix(model, "minimax-video-"):
		return "/v1/wand/minimax-video-v2/generation"
	case strings.HasPrefix(model, "kling-video-"):
		return "/v1/wand/kling/text-to-video" // precise path is resolved per request shape in BuildRequestBody
	case strings.HasPrefix(model, "vidu-video-"):
		return "/v1/wand/vidu/text-to-video" // precise path is resolved per request shape in BuildRequestBody
	case strings.HasPrefix(model, "pixverse-video-"):
		return "/v1/wand/pixverse/text-to-video" // precise path is resolved per request shape in BuildRequestBody
	case strings.HasPrefix(model, "wand-vega-video"):
		return "/v1/wand/vega-videos/tasks"
	case strings.HasPrefix(model, "vidu-image-"):
		return "/v1/wand/vidu-image/generation"
	default:
		return "/v1/wand/hunyuan-video/generation"
	}
}

// taskSubmitPathFor resolves submit endpoints that depend on request shape
// (kling / vidu / pixverse split text2video vs image2video).
func taskSubmitPathFor(model string, hasImage bool, imageCount int) string {
	switch {
	case strings.HasPrefix(model, "kling-video-"):
		switch {
		case strings.Contains(model, "omni") || strings.Contains(model, "-o1"):
			return "/v1/wand/kling/omni-video"
		case strings.Contains(model, "motion-control"):
			return "/v1/wand/kling/motion-control"
		case hasImage:
			return "/v1/wand/kling/image-to-video"
		default:
			return "/v1/wand/kling/text-to-video"
		}
	case strings.HasPrefix(model, "vidu-video-"):
		switch {
		case imageCount >= 2:
			// Two images = start/end frame; more images or explicit reference -> reference-to-video.
			return "/v1/wand/vidu/start-end-to-video"
		case imageCount == 1:
			return "/v1/wand/vidu/image-to-video"
		default:
			return "/v1/wand/vidu/text-to-video"
		}
	case strings.HasPrefix(model, "vidu-image-"):
		return "/v1/wand/vidu-image/generation"
	case strings.HasPrefix(model, "pixverse-video-"):
		if hasImage {
			return "/v1/wand/pixverse/image-to-video"
		}
		return "/v1/wand/pixverse/text-to-video"
	default:
		return taskSubmitPath(model)
	}
}

// taskQueryPath resolves the polling endpoint per model family.
func taskQueryPath(model string) string {
	switch {
	case strings.HasPrefix(model, "hy-3d-"):
		return "/v1/api/3d/query"
	case strings.HasPrefix(model, "yt-video-"):
		return "/v1/api/video/query"
	case strings.HasPrefix(model, "hy-video-"):
		return "/v1/wand/hunyuan-video/tasks/"
	case model == "minimax-video-v2.3" || model == "minimax-video-v2.3-fast":
		return "/v1/wand/minimax-video/tasks/"
	case strings.HasPrefix(model, "minimax-video-"):
		return "/v1/wand/minimax-video-v2/tasks/"
	case strings.HasPrefix(model, "kling-video-"):
		return "/v1/wand/kling/tasks/"
	case strings.HasPrefix(model, "vidu-video-"):
		return "/v1/wand/vidu/tasks/"
	case strings.HasPrefix(model, "pixverse-video-"):
		return "/v1/wand/pixverse/tasks/"
	case strings.HasPrefix(model, "wand-vega-video"):
		return "/v1/wand/vega-videos/tasks/"
	case strings.HasPrefix(model, "vidu-image-"):
		return "/v1/wand/vidu-image/tasks/"
	default:
		return "/v1/wand/hunyuan-video/tasks/"
	}
}

// ============================
// Request body conversion
// ============================

func buildTaskSubmitBody(req *relaycommon.TaskSubmitReq, info *relaycommon.RelayInfo, images []string) (any, error) {
	model := info.UpstreamModelName
	hasImage := len(images) > 0
	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" && req.Metadata != nil {
		if res, ok := req.Metadata["resolution"].(string); ok {
			resolution = res
		}
	}
	duration := req.Duration
	if duration <= 0 && req.Seconds != "" {
		if v, err := strconv.Atoi(req.Seconds); err == nil {
			duration = v
		}
	}
	ratio := aspectRatioFromSize(req.Size)

	switch {
	case strings.HasPrefix(model, "hy-3d-"):
		body := &Hy3DRequest{Model: model}
		if req.Prompt != "" {
			body.Prompt = req.Prompt
		}
		for _, img := range images {
			if strings.HasPrefix(img, "data:image") {
				body.ImageBase64 = img
			} else {
				body.ImageURL = img
			}
			break // 3D accepts a single primary image
		}
		apply3DMetadata(body, req.Metadata)
		return body, nil

	case strings.HasPrefix(model, "yt-video-"):
		body := &YTVideoRequest{Model: model, Prompt: req.Prompt}
		if hasImage {
			body.Image = &MediaURL{URL: images[0]}
		}
		return body, nil

	case strings.HasPrefix(model, "hy-video-"):
		body := &HyVideoRequest{
			Model:      model,
			Prompt:     req.Prompt,
			Duration:   duration,
			Resolution: resolution,
		}
		if ratio != "" {
			body.AspectRatio = ratio
		}
		if hasImage {
			first := images[0]
			if strings.HasPrefix(first, "data:image") {
				body.Image = first
			} else {
				body.ImageURL = first
			}
		}
		return body, nil

	case model == "minimax-video-v2.3" || model == "minimax-video-v2.3-fast":
		body := &MiniMaxV1VideoRequest{
			Model:      model,
			Prompt:     req.Prompt,
			Duration:   duration,
			Resolution: resolution,
		}
		if hasImage {
			body.FirstFrameImage = images[0]
		}
		return body, nil

	case strings.HasPrefix(model, "minimax-video-"):
		content := make([]ContentItem, 0, len(images)+1)
		if req.Prompt != "" {
			content = append(content, ContentItem{Type: "text", Text: req.Prompt})
		}
		for i, img := range images {
			item := ContentItem{Type: "image_url", ImageURL: &MediaURL{URL: img}}
			if i == 0 {
				item.Role = "first_frame"
			} else if i == 1 {
				item.Role = "last_frame"
			} else {
				item.Role = "reference_image"
			}
			content = append(content, item)
		}
		return &MiniMaxV2VideoRequest{
			Model:      model,
			Content:    content,
			Resolution: resolution,
			Duration:   duration,
		}, nil

	case strings.HasPrefix(model, "kling-video-"):
		body := &KlingRequest{Model: model, Prompt: req.Prompt}
		if hasImage || strings.Contains(model, "omni") || strings.Contains(model, "-o1") || strings.Contains(model, "motion-control") {
			contents := make([]KlingContent, 0, len(images)+1)
			if req.Prompt != "" {
				contents = append(contents, KlingContent{Type: "prompt", Text: req.Prompt})
			}
			for i, img := range images {
				contentType := "first_frame"
				if i == 1 {
					contentType = "last_frame"
				} else if i > 1 {
					contentType = "refer_image"
				}
				contents = append(contents, KlingContent{Type: contentType, URL: &MediaURL{URL: img}})
			}
			body.Contents = contents
			body.Prompt = ""
		}
		body.Settings = &KlingSettings{
			Resolution:  resolution,
			Duration:    duration,
			AspectRatio: ratio,
		}
		applyKlingMetadata(body, req.Metadata)
		return body, nil

	case strings.HasPrefix(model, "vidu-video-"):
		return &ViduRequest{
			Model:        model,
			Prompt:       req.Prompt,
			Images:       images,
			Duration:     duration,
			AspectRatio:  ratio,
			Resolution:   resolution,
		}, nil

	case strings.HasPrefix(model, "pixverse-video-"):
		body := &PixVerseRequest{
			Model:       model,
			Prompt:      req.Prompt,
			Duration:    duration,
			AspectRatio: ratio,
		}
		if body.Duration <= 0 {
			body.Duration = 5
		}
		if resolution != "" {
			body.Quality = normalizePixverseQuality(resolution)
		} else {
			body.Quality = "540p"
		}
		if hasImage {
			body.ImgID = images[0]
		}
		return body, nil

	case strings.HasPrefix(model, "wand-vega-video"):
		content := make([]ContentItem, 0, len(images)+1)
		if req.Prompt != "" {
			content = append(content, ContentItem{Type: "text", Text: req.Prompt})
		}
		for i, img := range images {
			item := ContentItem{Type: "image_url", ImageURL: &MediaURL{URL: img}}
			if i == 0 {
				item.Role = "first_frame"
			} else if i == 1 {
				item.Role = "last_frame"
			} else {
				item.Role = "reference_image"
			}
			content = append(content, item)
		}
		return &WandVideoRequest{
			Model:      model,
			Content:    content,
			Resolution: resolution,
			Ratio:      ratio,
			Duration:   duration,
		}, nil

	case strings.HasPrefix(model, "vidu-image-"):
		return &ViduImageRequest{
			Model:       model,
			Prompt:      req.Prompt,
			Images:      images,
			AspectRatio: ratio,
			Resolution:  resolution,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported tokenhub task model: %s", model)
	}
}

func collectImages(req *relaycommon.TaskSubmitReq) []string {
	var images []string
	if strings.TrimSpace(req.Image) != "" {
		images = append(images, strings.TrimSpace(req.Image))
	}
	for _, img := range req.Images {
		if strings.TrimSpace(img) != "" {
			images = append(images, strings.TrimSpace(img))
		}
	}
	if len(images) == 0 && req.Metadata != nil {
		if inputRef, ok := req.Metadata["input_reference"].(string); ok && inputRef != "" {
			images = append(images, inputRef)
		}
	}
	return images
}

func aspectRatioFromSize(size string) string {
	size = strings.TrimSpace(size)
	if size == "" {
		return ""
	}
	parts := strings.SplitN(strings.ToLower(size), "x", 2)
	if len(parts) != 2 {
		return ""
	}
	w, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	h, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || w <= 0 || h <= 0 {
		return ""
	}
	// Normalize to common aspect ratio strings accepted by TokenHub.
	ratios := []struct {
		label string
		num   float64
	}{
		{"21:9", 21.0 / 9.0}, {"16:9", 16.0 / 9.0}, {"4:3", 4.0 / 3.0},
		{"3:4", 3.0 / 4.0}, {"1:1", 1.0}, {"3:2", 3.0 / 2.0},
		{"2:3", 2.0 / 3.0}, {"9:16", 9.0 / 16.0},
	}
	r := float64(w) / float64(h)
	best, bestDiff := "", 1e9
	for _, cand := range ratios {
		diff := r - cand.num
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			best = cand.label
		}
	}
	if bestDiff/(r) < 0.05 {
		return best
	}
	return ""
}

func normalizePixverseQuality(resolution string) string {
	switch strings.ToLower(strings.TrimSpace(resolution)) {
	case "360p", "540p", "720p", "1080p":
		return strings.ToLower(strings.TrimSpace(resolution))
	case "4k", "2160p", "2k", "1440p":
		return "1080p"
	default:
		return "540p"
	}
}

func apply3DMetadata(body *Hy3DRequest, metadata map[string]any) {
	if metadata == nil {
		return
	}
	taskcommon.UnmarshalMetadata(metadata, body)
}

func applyKlingMetadata(body *KlingRequest, metadata map[string]any) {
	if metadata == nil {
		return
	}
	if audio, ok := metadata["audio"].(string); ok && audio != "" {
		body.Settings.Audio = audio
	}
}

// ============================
// Submit response parsing
// ============================

func extractSubmitTaskID(body []byte, model string) (string, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", fmt.Errorf("empty submit response")
	}

	switch {
	case strings.HasPrefix(model, "kling-video-"):
		var env struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
			Data    struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if err := common.Unmarshal(trimmed, &env); err != nil {
			return "", errors.Wrapf(err, "body: %s", string(trimmed))
		}
		if env.Code != 0 {
			return "", fmt.Errorf("tokenhub kling submit error: %d %s", env.Code, env.Message)
		}
		if env.Data.ID == "" {
			return "", fmt.Errorf("kling task id is empty, body: %s", string(trimmed))
		}
		return env.Data.ID, nil

	case strings.HasPrefix(model, "pixverse-video-"):
		var env struct {
			ErrCode int    `json:"ErrCode"`
			ErrMsg  string `json:"ErrMsg"`
			Resp    struct {
				VideoID string `json:"video_id"`
			} `json:"Resp"`
		}
		if err := common.Unmarshal(trimmed, &env); err != nil {
			return "", errors.Wrapf(err, "body: %s", string(trimmed))
		}
		if env.ErrCode != 0 {
			return "", fmt.Errorf("tokenhub pixverse submit error: %d %s", env.ErrCode, env.ErrMsg)
		}
		if env.Resp.VideoID == "" {
			return "", fmt.Errorf("pixverse video_id is empty, body: %s", string(trimmed))
		}
		return env.Resp.VideoID, nil

	default:
		// hy / minimax / vidu / wand / yt / hy-3d all reply with id or task_id.
		var env TaskEnvelope
		if err := common.Unmarshal(trimmed, &env); err != nil {
			return "", errors.Wrapf(err, "body: %s", string(trimmed))
		}
		if env.Error != nil {
			return "", fmt.Errorf("tokenhub submit error: %s", env.Error.ErrorMessage())
		}
		id := env.ID
		if id == "" {
			id = env.TaskID
		}
		if id == "" {
			return "", fmt.Errorf("task id is empty, body: %s", string(trimmed))
		}
		return id, nil
	}
}

func jsonAnyFromBytes(data []byte) any {
	var raw any
	if err := common.Unmarshal(data, &raw); err != nil {
		return nil
	}
	return raw
}

// ============================
// Polling
// ============================

func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	modelName, _ := body["model"].(string)
	baseUrl = strings.TrimRight(baseUrl, "/")
	queryPath := taskQueryPath(modelName)

	var req *http.Request
	var err error
	if queryPath == "/v1/api/video/query" || queryPath == "/v1/api/3d/query" {
		// YT video / HY-3D: POST {model, id}
		payload := map[string]any{"id": taskID}
		if modelName != "" {
			payload["model"] = modelName
		}
		raw, mErr := common.Marshal(payload)
		if mErr != nil {
			return nil, mErr
		}
		req, err = http.NewRequest(http.MethodPost, baseUrl+queryPath, bytes.NewReader(raw))
	} else {
		uri := fmt.Sprintf("%s%s%s", baseUrl, queryPath, url.PathEscape(taskID))
		req, err = http.NewRequest(http.MethodGet, uri, nil)
	}
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	return parseTokenhubTaskResult(respBody)
}

func (a *TaskAdaptor) GetModelList() []string {
	return append(append(append(append([]string{}, VideoModelList...), ThreeDModelList...), AsyncImageModelList...), TTSModelList...)
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// ConvertToOpenAIVideo implements channel.OpenAIVideoConverter.
func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.TaskID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	openAIVideo.Model = originTask.Properties.OriginModelName

	resultURL := strings.TrimSpace(originTask.GetResultURL())
	if resultURL == "" {
		resultURL = extractResultURLFromTaskData(originTask.Data)
	}
	if resultURL != "" {
		openAIVideo.SetMetadata("url", resultURL)
	}
	if ux := jsonAnyFromBytes(originTask.Data); ux != nil {
		openAIVideo.SetMetadata("upstream", ux)
	}
	if originTask.FailReason != "" && originTask.Status == model.TaskStatusFailure {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: originTask.FailReason,
			Code:    "task_failed",
		}
	}
	return common.Marshal(openAIVideo)
}

// ConvertToOpenAIAsyncImage implements channel.OpenAIAsyncImageConverter.
func (a *TaskAdaptor) ConvertToOpenAIAsyncImage(originTask *model.Task) ([]byte, error) {
	out := map[string]any{
		"object":     "tokenhub.image.generation.task",
		"id":         originTask.TaskID,
		"task_id":    originTask.TaskID,
		"status":     originTask.Status.ToVideoStatus(),
		"progress":   originTask.Progress,
		"model":      originTask.Properties.OriginModelName,
		"created_at": originTask.CreatedAt,
		"updated_at": originTask.UpdatedAt,
	}
	imageURL := strings.TrimSpace(originTask.GetResultURL())
	if imageURL == "" {
		imageURL = extractResultURLFromTaskData(originTask.Data)
	}
	if imageURL != "" {
		out["url"] = imageURL
	}
	if ux := jsonAnyFromBytes(originTask.Data); ux != nil {
		out["upstream"] = ux
	}
	if originTask.FailReason != "" {
		out["fail_reason"] = originTask.FailReason
	}
	return common.Marshal(out)
}
