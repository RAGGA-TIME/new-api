package tokenhub

import "strings"

// ============================
// Image generation DTOs
// ============================

// HyImageV3Request is the body for hy-image-v3 (sync).
// Docs: https://cloud.tencent.com/document/product/1823/135745
type HyImageV3Request struct {
	Model         string   `json:"model"`
	Prompt        string   `json:"prompt"`
	Images        []string `json:"images,omitempty"`
	Size          string   `json:"size,omitempty"`
	Seed          int64    `json:"seed,omitempty"`
	Footnote      string   `json:"footnote,omitempty"`
	Revise        *bool    `json:"revise,omitempty"`
}

type HyImageV3Response struct {
	ID            string `json:"id"`
	Created       int64  `json:"created"`
	Data          []HyImageDataItem `json:"data"`
	RequestID     string `json:"request_id"`
	TokenhubUsage *TokenhubUsage `json:"tokenhub_usage,omitempty"`
}

type HyImageDataItem struct {
	URL           string `json:"url,omitempty"`
	B64Json       string `json:"b64_json,omitempty"`
	RevisedPrompt string `json:"revised_prompt,omitempty"`
}

// TokenhubUsage appears on most TokenHub generation responses.
type TokenhubUsage struct {
	TotalTokens int `json:"total_tokens"`
}

// HyImageV35Request is the Chat/Messages-style body for hy-image-v3.5-preview (sync).
type HyImageV35Request struct {
	Model             string             `json:"model"`
	Messages          []HyImageV35Message `json:"messages"`
	Size              string             `json:"size,omitempty"`
	Seed              int64              `json:"seed,omitempty"`
	GenerateMaxPixels int                `json:"generate_max_pixels,omitempty"`
	ResizeMaxPixels   int                `json:"resize_max_pixels,omitempty"`
	Session           string             `json:"session,omitempty"`
	Footnote          string             `json:"footnote,omitempty"`
}

type HyImageV35Message struct {
	Role    string             `json:"role"`
	Content []HyImageV35Content `json:"content,omitempty"`
}

type HyImageV35Content struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *MediaURL    `json:"image_url,omitempty"`
}

// MediaURL is the {"url": "..."} wrapper used across TokenHub multimodal payloads.
type MediaURL struct {
	URL string `json:"url"`
}

// HyImageV35Response mirrors the chunk-like response of hy-image-v3.5-preview.
type HyImageV35Response struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Choices []struct {
		Index int `json:"index"`
		Delta struct {
			Type  string `json:"type,omitempty"`
			Image *struct {
				URL    string `json:"url"`
				Width  int    `json:"width,omitempty"`
				Height int    `json:"height,omitempty"`
			} `json:"image,omitempty"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *TokenhubError `json:"error,omitempty"`
	TokenhubUsage *TokenhubUsage `json:"tokenhub_usage,omitempty"`
	RequestID     string         `json:"request_id"`
}

// SeedreamImageRequest is the body for seedream-image-* (sync, OpenAI-images-like).
type SeedreamImageRequest struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	Images         []string `json:"images,omitempty"`
	Size           string   `json:"size,omitempty"`
	ResponseFormat string   `json:"response_format,omitempty"`
	OutputFormat   string   `json:"output_format,omitempty"`
	Watermark      *bool    `json:"watermark,omitempty"`
	Seed           int64    `json:"seed,omitempty"`
}

// GenericImageRequest is the body used for WAND Vega / Kling image models.
// Bodies follow the per-series OpenAI-like convention.
type GenericImageRequest struct {
	Model          string   `json:"model"`
	Prompt         string   `json:"prompt"`
	Images         []string `json:"images,omitempty"`
	Size           string   `json:"size,omitempty"`
	ResponseFormat string   `json:"response_format,omitempty"`
	Seed           int64    `json:"seed,omitempty"`
}

// GenericImageResponse covers seedream/wand/kling style responses
// ({"data": [{"url": ...}], "tokenhub_usage": ...}).
type GenericImageResponse struct {
	Created       int64             `json:"created"`
	CreatedAt     int64             `json:"created_at"`
	Data          []HyImageDataItem `json:"data"`
	Error         *TokenhubError    `json:"error,omitempty"`
	RequestID     string            `json:"request_id"`
	TokenhubUsage *TokenhubUsage    `json:"tokenhub_usage,omitempty"`
}

// ============================
// TTS DTOs
// ============================

// Docs: https://cloud.tencent.com/document/product/1823/135796
type SyncTTSRequest struct {
	Model          string          `json:"model"`
	Text           string          `json:"text"`
	VoiceSetting   *TTSVoiceSetting `json:"voice_setting,omitempty"`
	AudioSetting   *TTSAudioSetting `json:"audio_setting,omitempty"`
	OutputFormat   string          `json:"output_format,omitempty"`
	SubtitleEnable bool            `json:"subtitle_enable,omitempty"`
	AIGCWatermark  bool            `json:"aigc_watermark,omitempty"`
}

type TTSVoiceSetting struct {
	VoiceID string  `json:"voice_id"`
	Speed   float64 `json:"speed,omitempty"`
	Vol     float64 `json:"vol,omitempty"`
	Pitch   int     `json:"pitch,omitempty"`
	Emotion string  `json:"emotion,omitempty"`
}

type TTSAudioSetting struct {
	SampleRate int    `json:"sample_rate,omitempty"`
	Bitrate    int    `json:"bitrate,omitempty"`
	Format     string `json:"format,omitempty"`
	Channel    int    `json:"channel,omitempty"`
}

type SyncTTSResponse struct {
	Data *struct {
		Audio       string `json:"audio"`
		Status      int    `json:"status"`
		SubtitleFile string `json:"subtitle_file,omitempty"`
	} `json:"data"`
	ExtraInfo *struct {
		AudioLength     int    `json:"audio_length"`
		AudioSampleRate int    `json:"audio_sample_rate"`
		AudioSize       int    `json:"audio_size"`
		UsageCharacters int    `json:"usage_characters"`
		AudioFormat     string `json:"audio_format"`
		AudioChannel    int    `json:"audio_channel"`
	} `json:"extra_info"`
	BaseResp *BaseResp `json:"base_resp,omitempty"`
	Usage    *struct {
		TotalToken int `json:"total_token"`
	} `json:"usage,omitempty"`
	TraceID string `json:"trace_id"`
}

type BaseResp struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// TokenhubError is the unified error object across TokenHub endpoints.
type TokenhubError struct {
	Type      string `json:"type,omitempty"`
	Code      any    `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func (e *TokenhubError) ErrorMessage() string {
	if e == nil {
		return ""
	}
	msg := e.Message
	if msg == "" {
		msg = "tokenhub upstream error"
	}
	if e.Code != nil {
		if s, ok := e.Code.(string); ok && s != "" {
			return s + ": " + msg
		}
	}
	return msg
}

// NormalizeImageSize converts OpenAI "1024x1024" style size to TokenHub size.
func NormalizeImageSize(size string) string {
	size = strings.ToLower(strings.TrimSpace(size))
	if size == "auto" {
		return ""
	}
	return size
}
