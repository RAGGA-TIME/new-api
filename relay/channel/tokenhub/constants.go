package tokenhub

const (
	ChannelName = "TokenHub"

	// API paths (relative to channel base URL).
	// Docs: https://cloud.tencent.com/document/product/1823/130079
	PathChatCompletions = "/v1/chat/completions"
	PathCompletions     = "/v1/completions"
	PathEmbeddings      = "/v1/embeddings"
	PathResponses       = "/v1/responses"
	PathModels          = "/v1/models"

	// Image generation (sync)
	PathHyImageV3Generation    = "/v1/wand/hunyuan-image/v3-generation"
	PathHyImageV35Generation   = "/v1/wand/hunyuan-image/v35-generation"
	PathSeedreamImageGenerator = "/v1/wand/si-image/generation"
	// NOTE: WAND Vega image / Kling image endpoints follow the documented
	// per-series path conventions (/v1/wand/<series>/<action>); adjust here if
	// upstream changes them.
	PathWandImageGeneration    = "/v1/wand/vega-image/generation"
	PathKlingTextToImage       = "/v1/wand/kling/text-to-image"
	PathKlingImageToImage      = "/v1/wand/kling/image-to-image"

	// TTS (sync)
	PathMiniMaxSyncTTS = "/v1/wand/minimax-tts/sync_tts"
)

// ============================
// Model lists
// ============================

// ModelList covers text/multimodal-understanding language models available on
// TokenHub (OpenAI Chat Completions compatible).
var ModelList = []string{
	"hy4-preview",
	"hy3",
	"hy-mt2-pro",
	"hy-mt2-plus",
	"hy-mt2-lite",
	"hunyuan-role-latest",
	"hy-role",
	"deepseek-v4.1-flash",
	"deepseek-v4-flash-0731",
	"deepseek-v4-pro-0813",
	"deepseek-v4-flash",
	"deepseek-v4-pro",
	"glm-5.3",
	"glm-5.3-flash",
	"glm-5.3-flashx",
	"glm-5.2",
	"glm-5.1",
	"glm-5v-turbo",
	"glm-5-turbo",
	"glm-5",
	"kimi-k3",
	"kimi-k2.6",
	"kimi-k2.7-code",
	"kimi-k2.7-code-highspeed",
	"kimi-k2.8-preview",
	"minimax-m3",
	"minimax-m2.7",
	"qwen3.5-flash",
	"qwen3.5-plus",
	"mimo-v2.6-pro",
	"mimo-v2.6-flash",
	"mimo-v2.5-pro",
	"step-5-preview",
}

// ImageModelList covers sync image generation models.
var ImageModelList = []string{
	"hy-image-v3",
	"hy-image-v3.5-preview",
	"wand-vega-image-lite",
	"wand-vega-image-flash",
	"wand-vega-image-pro",
	"seedream-image-v5.0-pro",
	"seedream-image-v5.0-lite",
	"kling-image-v3",
	"kling-image-o1",
	"kling-image-v3-omni",
}

// AsyncImageModelList covers async image generation models (task based).
var AsyncImageModelList = []string{
	"vidu-image-q2",
}

// VideoModelList covers async video generation models.
var VideoModelList = []string{
	"hy-video-v1.5",
	"minimax-video-v2.3",
	"minimax-video-v2.3-fast",
	"minimax-video-h3",
	"minimax-video-h3-max",
	"kling-video-v3",
	"kling-video-v3-omni",
	"kling-video-v3-turbo",
	"kling-video-o1",
	"kling-video-v2.6",
	"kling-video-v2.5-turbo",
	"kling-video-motion-control-v3.0",
	"kling-video-motion-control-v2.6",
	"vidu-video-q4-preview",
	"vidu-video-q3",
	"vidu-video-q3-pro",
	"vidu-video-q3-turbo",
	"vidu-video-q2",
	"vidu-video-q2-pro",
	"vidu-video-q2-turbo",
	"pixverse-video-v6.0",
	"pixverse-video-c1",
	"pixverse-video-v5.6",
	"wand-vega-video-1.0-lite",
	"yt-video-2.0",
	"yt-video-humanactor",
}

// ThreeDModelList covers async 3D generation models.
var ThreeDModelList = []string{
	"hy-3d-3.0",
	"hy-3d-3.1",
}

// TTSModelList covers sync speech synthesis models.
var TTSModelList = []string{
	"minimax-speech-2.8-hd",
	"minimax-speech-2.8-turbo",
	"minimax-speech-2.6-hd",
	"minimax-speech-2.6-turbo",
	"minimax-speech-02-hd",
	"minimax-speech-02-turbo",
}

// ============================
// Model capability helpers
// ============================

func IsImageModel(model string) bool {
	for _, m := range ImageModelList {
		if m == model {
			return true
		}
	}
	return false
}

func IsAsyncImageModel(model string) bool {
	for _, m := range AsyncImageModelList {
		if m == model {
			return true
		}
	}
	return false
}

func IsVideoModel(model string) bool {
	for _, m := range VideoModelList {
		if m == model {
			return true
		}
	}
	return false
}

func Is3DModel(model string) bool {
	for _, m := range ThreeDModelList {
		if m == model {
			return true
		}
	}
	return false
}

func IsTTSModel(model string) bool {
	for _, m := range TTSModelList {
		if m == model {
			return true
		}
	}
	return false
}

// AllModels returns every TokenHub model across all capabilities
// (chat / image / async image / video / 3D / TTS).
func AllModels() []string {
	all := make([]string, 0, len(ModelList)+len(ImageModelList)+len(AsyncImageModelList)+len(VideoModelList)+len(ThreeDModelList)+len(TTSModelList))
	seen := make(map[string]struct{}, len(all))
	for _, list := range [][]string{ModelList, ImageModelList, AsyncImageModelList, VideoModelList, ThreeDModelList, TTSModelList} {
		for _, m := range list {
			if _, ok := seen[m]; ok {
				continue
			}
			seen[m] = struct{}{}
			all = append(all, m)
		}
	}
	return all
}
