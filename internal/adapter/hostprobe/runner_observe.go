package hostprobe

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type hostStreamObservation struct {
	SessionStartObserved bool   `json:"session_start_observed"`
	PreToolUseObserved   bool   `json:"pre_tool_use_observed"`
	Model                string `json:"-"`
	AmbientToolCount     int    `json:"-"`
	MCPCallCount         int    `json:"mcp_call_count"`
	ResponseSHA256       string `json:"response_sha256"`
	ExitCode             int    `json:"exit_code"`
	DurationMS           int64  `json:"duration_ms"`
}

type semanticToolResponse struct {
	Content           []semanticToolContent `json:"content"`
	StructuredContent any                   `json:"structured_content,omitempty"`
	IsError           bool                  `json:"is_error"`
}

type semanticToolContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func observeHostStream(data []byte) (hostStreamObservation, error) {
	if len(data) == 0 || len(data) > MaxOutputBytes {
		return hostStreamObservation{}, fmt.Errorf("host_stream_invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	observation := hostStreamObservation{}
	results := make([]any, 0, 1)
	claudeProbeCalls := map[string]bool{}
	events := 0
	for {
		var event any
		if err := decoder.Decode(&event); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return hostStreamObservation{}, fmt.Errorf("host_stream_invalid")
		}
		events++
		observeStructuredModel(event, &observation)
		if err := observeHookEvent(event, &observation); err != nil {
			return hostStreamObservation{}, err
		}
		if err := observeToolCalls(event, claudeProbeCalls, &observation); err != nil {
			return hostStreamObservation{}, err
		}
		for _, result := range observedClaudeProbeResults(event, claudeProbeCalls) {
			observation.MCPCallCount++
			results = append(results, result)
		}
		if result, ok := observedMCPResult(event); ok {
			observation.MCPCallCount++
			results = append(results, result)
		}
	}
	if events == 0 {
		return hostStreamObservation{}, fmt.Errorf("host_stream_invalid")
	}
	if len(results) > 0 {
		digest, err := semanticResponseDigest(results)
		if err != nil {
			return hostStreamObservation{}, fmt.Errorf("host_stream_invalid")
		}
		observation.ResponseSHA256 = digest
	}
	return observation, nil
}

func observeStructuredModel(value any, observation *hostStreamObservation) {
	event, ok := value.(map[string]any)
	if !ok || observation.Model != "" {
		return
	}
	if model, ok := event["model"].(string); ok && strings.TrimSpace(model) != "" {
		observation.Model = boundedVersion(model)
		return
	}
	if message, ok := event["message"].(map[string]any); ok {
		if model, ok := message["model"].(string); ok && strings.TrimSpace(model) != "" {
			observation.Model = boundedVersion(model)
		}
	}
}

func observeToolCalls(value any, claudeCalls map[string]bool, observation *hostStreamObservation) error {
	event, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	if event["type"] == "item.completed" {
		item, _ := event["item"].(map[string]any)
		typeName, _ := item["type"].(string)
		switch typeName {
		case "reasoning", "agent_message", "todo_list":
		case "error":
			if item["message"] != "`--dangerously-bypass-hook-trust` is enabled. Enabled hooks may run without review for this invocation." {
				return fmt.Errorf("host_stream_invalid")
			}
		case "":
			return fmt.Errorf("host_stream_invalid")
		default:
			observation.AmbientToolCount++
		}
	}
	if event["type"] != "assistant" {
		return nil
	}
	message, _ := event["message"].(map[string]any)
	content, _ := message["content"].([]any)
	for _, rawBlock := range content {
		block, _ := rawBlock.(map[string]any)
		if block["type"] != "tool_use" {
			continue
		}
		name, _ := block["name"].(string)
		id, _ := block["id"].(string)
		if id == "" || name == "" {
			return fmt.Errorf("host_stream_invalid")
		}
		if _, exists := claudeCalls[id]; exists {
			return fmt.Errorf("host_stream_invalid")
		}
		claudeCalls[id] = strings.Contains(name, "issueops_probe")
		observation.AmbientToolCount++
	}
	return nil
}

func semanticResponseDigest(results []any) (string, error) {
	if len(results) == 0 {
		return "", fmt.Errorf("semantic_response_missing")
	}
	projection := make([]semanticToolResponse, 0, len(results))
	for _, result := range results {
		projected, err := projectSemanticToolResponse(result)
		if err != nil {
			return "", err
		}
		projection = append(projection, projected)
	}
	canonical, err := json.Marshal(projection)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return hex.EncodeToString(digest[:]), nil
}

func projectSemanticToolResponse(value any) (semanticToolResponse, error) {
	object, ok := value.(map[string]any)
	if !ok {
		return semanticToolResponse{}, fmt.Errorf("semantic_response_invalid")
	}
	rawContent, ok := object["content"]
	if !ok {
		return semanticToolResponse{}, fmt.Errorf("semantic_response_invalid")
	}
	content, err := projectSemanticToolContent(rawContent)
	if err != nil {
		return semanticToolResponse{}, err
	}
	projected := semanticToolResponse{Content: content}
	if structured, exists := object["structuredContent"]; exists {
		projected.StructuredContent = structured
	} else if structured, exists := object["structured_content"]; exists {
		projected.StructuredContent = structured
	}
	if rawError, exists := object["isError"]; exists {
		isError, ok := rawError.(bool)
		if !ok {
			return semanticToolResponse{}, fmt.Errorf("semantic_response_invalid")
		}
		projected.IsError = isError
	} else if rawError, exists := object["is_error"]; exists {
		isError, ok := rawError.(bool)
		if !ok {
			return semanticToolResponse{}, fmt.Errorf("semantic_response_invalid")
		}
		projected.IsError = isError
	}
	if projected.IsError {
		return semanticToolResponse{}, fmt.Errorf("semantic_response_tool_error")
	}
	return projected, nil
}

func projectSemanticToolContent(value any) ([]semanticToolContent, error) {
	switch content := value.(type) {
	case string:
		return []semanticToolContent{{Type: "text", Text: content}}, nil
	case []any:
		projected := make([]semanticToolContent, 0, len(content))
		for _, rawBlock := range content {
			block, ok := rawBlock.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("semantic_response_invalid")
			}
			typeName, _ := block["type"].(string)
			text, textOK := block["text"].(string)
			if typeName == "" && textOK {
				typeName = "text"
			}
			if typeName != "text" || !textOK {
				return nil, fmt.Errorf("semantic_response_invalid")
			}
			projected = append(projected, semanticToolContent{Type: "text", Text: text})
		}
		return projected, nil
	default:
		return nil, fmt.Errorf("semantic_response_invalid")
	}
}

func observedClaudeProbeResults(value any, calls map[string]bool) []any {
	event, ok := value.(map[string]any)
	if !ok || event["type"] != "user" {
		return nil
	}
	message, _ := event["message"].(map[string]any)
	content, _ := message["content"].([]any)
	results := make([]any, 0, 1)
	for _, rawBlock := range content {
		block, _ := rawBlock.(map[string]any)
		id, _ := block["tool_use_id"].(string)
		if block["type"] != "tool_result" {
			continue
		}
		isProbe, exists := calls[id]
		if !exists {
			continue
		}
		delete(calls, id)
		if !isProbe {
			continue
		}
		if result, exists := event["tool_use_result"]; exists && result != nil {
			if content, ok := result.([]any); ok {
				projected := map[string]any{"content": content}
				if isError, exists := block["is_error"]; exists {
					projected["is_error"] = isError
				}
				result = projected
			}
			results = append(results, result)
		} else {
			results = append(results, block)
		}
	}
	return results
}

func observeHookEvent(value any, observation *hostStreamObservation) error {
	event, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	var name string
	switch event["type"] {
	case "hook.completed":
		hook, _ := event["hook"].(map[string]any)
		name, _ = hook["event_name"].(string)
	case "system":
		if event["subtype"] == "hook_response" {
			name, _ = event["hook_event"].(string)
		}
	}
	switch strings.ToLower(strings.ReplaceAll(name, "_", "")) {
	case "sessionstart":
		if event["type"] == "system" && (event["exit_code"] != json.Number("0") || event["outcome"] != "success") {
			return fmt.Errorf("hook_observation_invalid")
		}
		observation.SessionStartObserved = true
	case "pretooluse":
		observation.PreToolUseObserved = true
	}
	return nil
}

func observedMCPResult(value any) (any, bool) {
	event, ok := value.(map[string]any)
	if !ok {
		return nil, false
	}
	typeName, _ := event["type"].(string)
	switch typeName {
	case "item.completed":
		item, _ := event["item"].(map[string]any)
		server, _ := item["server"].(string)
		status, _ := item["status"].(string)
		if item["type"] != "mcp_tool_call" || !strings.Contains(server, "issueops_probe") || status != "completed" {
			return nil, false
		}
		result, exists := item["result"]
		return result, exists && result != nil
	case "tool_result":
		tool, _ := event["tool"].(map[string]any)
		name, _ := tool["name"].(string)
		if !strings.Contains(name, "issueops_probe") {
			return nil, false
		}
		result, exists := event["result"]
		return result, exists && result != nil
	default:
		return nil, false
	}
}

func observeRecordedHookEvents(observationPath string) (hostStreamObservation, error) {
	observationPath = strings.TrimSpace(observationPath)
	if !filepath.IsAbs(observationPath) {
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	markerPath := observationPath + ".hooks"
	info, err := os.Lstat(markerPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return hostStreamObservation{}, nil
		}
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 {
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	data, err := readBoundedEvidenceFile(markerPath)
	if err != nil {
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	observation, err := decodeHookMarkerEvents(data)
	if err != nil {
		return hostStreamObservation{}, err
	}
	if err := os.Remove(markerPath); err != nil {
		return hostStreamObservation{}, fmt.Errorf("hook_observation_cleanup_failed")
	}
	return observation, nil
}

func decodeHookMarkerEvents(data []byte) (hostStreamObservation, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	observation := hostStreamObservation{}
	events := 0
	for {
		var marker struct {
			Event string `json:"event"`
			Model string `json:"model"`
		}
		if err := decoder.Decode(&marker); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
		}
		events++
		switch marker.Event {
		case "SessionStart":
			model := boundedVersion(marker.Model)
			if model != marker.Model || (observation.Model != "" && model != "" && observation.Model != model) {
				return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
			}
			observation.SessionStartObserved = true
			if model != "" {
				observation.Model = model
			}
		case "PreToolUse":
			observation.PreToolUseObserved = true
		default:
			return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
		}
	}
	if events == 0 {
		return hostStreamObservation{}, fmt.Errorf("hook_observation_invalid")
	}
	return observation, nil
}

func mergeHookObservation(observation *hostStreamObservation, recorded hostStreamObservation) error {
	if observation.Model != "" && recorded.Model != "" && observation.Model != recorded.Model {
		return fmt.Errorf("host_model_mismatch")
	}
	if observation.Model == "" {
		observation.Model = recorded.Model
	}
	observation.SessionStartObserved = observation.SessionStartObserved || recorded.SessionStartObserved
	observation.PreToolUseObserved = observation.PreToolUseObserved || recorded.PreToolUseObserved
	return nil
}

func validHostRuntimeObservation(observation hostStreamObservation) bool {
	return observation.SessionStartObserved && !observation.PreToolUseObserved &&
		observation.Model != "" && observation.AmbientToolCount == 1 && observation.MCPCallCount == 1 &&
		validSHA256(observation.ResponseSHA256)
}
