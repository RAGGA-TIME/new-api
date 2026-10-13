package tokenhub

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// parseTokenhubTaskResult parses the raw polling response of any TokenHub
// task family (hy video, minimax v1/v2, kling, vidu, pixverse, wand, yt,
// hy-3d, vidu-image). The shapes differ per series, so this parser tries each
// known structure loosely.
func parseTokenhubTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	trimmed := bytes.TrimSpace(respBody)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty task result")
	}

	var raw map[string]any
	if err := common.Unmarshal(trimmed, &raw); err != nil {
		return nil, fmt.Errorf("unmarshal task result failed: %w", err)
	}

	// 1. Kling: {"code": 0, "data": [{id, status, outputs:[{url}], message}]}
	if ti := parseKlingTaskResult(raw); ti != nil {
		return ti, nil
	}

	// 2. PixVerse: {"ErrCode": 0, "Resp": {id, status, url}}
	if ti := parsePixVerseTaskResult(raw); ti != nil {
		return ti, nil
	}

	// 3. MiniMax V2: {"task": {id, status, content:{url}, error}}
	if ti := parseMiniMaxV2TaskResult(raw); ti != nil {
		return ti, nil
	}

	// 4. Generic flat envelope: {id|task_id, status|state, ...}
	ti := parseFlatTaskResult(raw)
	if ti == nil {
		return nil, fmt.Errorf("unrecognized task result body: %s", string(trimmed))
	}
	return ti, nil
}

func parseKlingTaskResult(raw map[string]any) *relaycommon.TaskInfo {
	codeVal, hasCode := raw["code"]
	dataRaw, hasData := raw["data"]
	if !hasCode || !hasData {
		return nil
	}
	code, ok := codeVal.(float64)
	if !ok {
		return nil
	}
	dataArr, ok := dataRaw.([]any)
	if !ok || len(dataArr) == 0 {
		if code != 0 {
			return &relaycommon.TaskInfo{
				Status:   model.TaskStatusFailure,
				Code:     int(code),
				Reason:   fmt.Sprintf("%v", raw["message"]),
				Progress: "100%",
			}
		}
		return nil
	}
	taskMap, ok := dataArr[0].(map[string]any)
	if !ok {
		return nil
	}
	status, _ := taskMap["status"].(string)
	ti := &relaycommon.TaskInfo{Code: 0}
	switch strings.ToLower(status) {
	case "succeeded", "success", "succeed":
		ti.Status = model.TaskStatusSuccess
		ti.Progress = "100%"
		ti.Url = firstStringFromOutputs(taskMap)
	case "failed":
		ti.Status = model.TaskStatusFailure
		ti.Progress = "100%"
		ti.Reason = stringFromMap(taskMap, "message")
		if ti.Reason == "" {
			ti.Reason = stringFromMap(raw, "message")
		}
	default: // processing / submitted / ...
		ti.Status = model.TaskStatusInProgress
		ti.Progress = "50%"
	}
	return ti
}

func firstStringFromOutputs(taskMap map[string]any) string {
	outputs, ok := taskMap["outputs"].([]any)
	if !ok {
		return ""
	}
	for _, o := range outputs {
		om, ok := o.(map[string]any)
		if !ok {
			continue
		}
		if u := stringFromMap(om, "url"); u != "" {
			return u
		}
	}
	return ""
}

func parsePixVerseTaskResult(raw map[string]any) *relaycommon.TaskInfo {
	respRaw, ok := raw["Resp"]
	if !ok {
		return nil
	}
	resp, ok := respRaw.(map[string]any)
	if !ok {
		return nil
	}
	statusVal, ok := resp["status"].(float64)
	if !ok {
		return nil
	}
	status := int(statusVal)
	ti := &relaycommon.TaskInfo{Code: 0}
	switch status {
	case 1:
		ti.Status = model.TaskStatusSuccess
		ti.Progress = "100%"
		ti.Url = stringFromMap(resp, "url")
	case 5:
		ti.Status = model.TaskStatusInProgress
		ti.Progress = "50%"
	default: // 6 deleted / 7 moderation failed / 8 failed
		ti.Status = model.TaskStatusFailure
		ti.Progress = "100%"
		ti.Reason = fmt.Sprintf("pixverse task status %d: %s", status, stringFromMap(raw, "ErrMsg"))
	}
	return ti
}

func parseMiniMaxV2TaskResult(raw map[string]any) *relaycommon.TaskInfo {
	taskRaw, ok := raw["task"]
	if !ok {
		return nil
	}
	task, ok := taskRaw.(map[string]any)
	if !ok {
		return nil
	}
	status, _ := task["status"].(string)
	if status == "" {
		return nil
	}
	ti := &relaycommon.TaskInfo{Code: 0}
	switch strings.ToLower(status) {
	case "succeeded", "success":
		ti.Status = model.TaskStatusSuccess
		ti.Progress = "100%"
		if content, ok := task["content"].(map[string]any); ok {
			ti.Url = stringFromMap(content, "url")
		}
	case "failed":
		ti.Status = model.TaskStatusFailure
		ti.Progress = "100%"
		ti.Reason = errorMessageFromMap(task["error"])
		if ti.Reason == "" {
			ti.Reason = "tokenhub minimax task failed"
		}
	case "cancelled":
		ti.Status = model.TaskStatusFailure
		ti.Progress = "100%"
		ti.Reason = "task cancelled"
	default: // queued / running
		ti.Status = model.TaskStatusInProgress
		ti.Progress = "50%"
	}
	return ti
}

// parseFlatTaskResult handles hy video, minimax v1, vidu, wand, yt, hy-3d and
// vidu-image responses (all flat: id/task_id + status/state + result urls).
func parseFlatTaskResult(raw map[string]any) *relaycommon.TaskInfo {
	status := strings.ToLower(stringOr(stringValue(raw["status"]), stringValue(raw["state"])))
	if status == "" {
		return nil
	}

	ti := &relaycommon.TaskInfo{Code: 0}
	switch status {
	case "succeeded", "success", "completed", "done":
		ti.Status = model.TaskStatusSuccess
		ti.Progress = "100%"
		ti.Url = extractResultURLFromMap(raw)
	case "failed", "failure", "fail":
		ti.Status = model.TaskStatusFailure
		ti.Progress = "100%"
		ti.Reason = taskFailureReason(raw)
	case "queued", "queueing", "created", "preparing", "submitted":
		ti.Status = model.TaskStatusQueued
		ti.Progress = "10%"
	default: // running / processing / in_progress ...
		ti.Status = model.TaskStatusInProgress
		ti.Progress = "50%"
	}
	return ti
}

// extractResultURLFromMap searches the known URL locations across series:
//   - hy video:      videos[0].url
//   - minimax v1:    content.url
//   - wand video:    content.video_url
//   - yt video:      data.url
//   - hy-3d:         data[0].url
//   - vidu:          creations[0].url
func extractResultURLFromMap(raw map[string]any) string {
	// videos[].url (hy video)
	if arr, ok := raw["videos"].([]any); ok {
		for _, v := range arr {
			if m, ok := v.(map[string]any); ok {
				if u := stringFromMap(m, "url"); u != "" {
					return u
				}
			}
		}
	}
	// content.url / content.video_url (minimax v1 / wand)
	if content, ok := raw["content"].(map[string]any); ok {
		if u := stringFromMap(content, "url"); u != "" {
			return u
		}
		if u := stringFromMap(content, "video_url"); u != "" {
			return u
		}
	}
	// creations[].url (vidu video / vidu image)
	if arr, ok := raw["creations"].([]any); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]any); ok {
			if u := stringFromMap(m, "url"); u != "" {
				return u
			}
		}
	}
	// data.url (yt video) or data[0].url (hy-3d)
	switch data := raw["data"].(type) {
	case map[string]any:
		if u := stringFromMap(data, "url"); u != "" {
			return u
		}
	case []any:
		for _, v := range data {
			if m, ok := v.(map[string]any); ok {
				if u := stringFromMap(m, "url"); u != "" {
					return u
				}
			}
		}
	}
	return ""
}

// extractResultURLFromTaskData performs the same loose URL extraction on
// stored task data (used by ConvertToOpenAIVideo / ConvertToOpenAIAsyncImage).
func extractResultURLFromTaskData(data []byte) string {
	if len(bytes.TrimSpace(data)) == 0 {
		return ""
	}
	var raw map[string]any
	if err := common.Unmarshal(data, &raw); err != nil {
		return ""
	}
	if u := extractResultURLFromMap(raw); u != "" {
		return u
	}
	// MiniMax V2 nests everything under "task".
	if task, ok := raw["task"].(map[string]any); ok {
		return extractResultURLFromMap(task)
	}
	// Kling nests under data[0].outputs[].url.
	if arr, ok := raw["data"].([]any); ok && len(arr) > 0 {
		if m, ok := arr[0].(map[string]any); ok {
			return firstStringFromOutputs(m)
		}
	}
	return ""
}

func taskFailureReason(raw map[string]any) string {
	if r := errorMessageFromMap(raw["error"]); r != "" {
		return r
	}
	// minimax v1: base_resp.status_msg
	if base, ok := raw["base_resp"].(map[string]any); ok {
		if msg := stringFromMap(base, "status_msg"); msg != "" && msg != "success" {
			return msg
		}
	}
	// vidu style: top-level message
	if msg := stringFromMap(raw, "message"); msg != "" {
		return msg
	}
	// pixverse style
	if msg := stringFromMap(raw, "ErrMsg"); msg != "" {
		return msg
	}
	return "tokenhub task failed"
}

func errorMessageFromMap(v any) string {
	m, ok := v.(map[string]any)
	if !ok {
		return ""
	}
	msg := stringFromMap(m, "message")
	if msg == "" {
		msg = stringFromMap(m, "msg")
	}
	code := stringFromMap(m, "code")
	if code != "" && msg != "" {
		return code + ": " + msg
	}
	return msg
}

func stringFromMap(m map[string]any, key string) string {
	v, ok := m[key].(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(v)
}

func stringValue(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func stringOr(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}
