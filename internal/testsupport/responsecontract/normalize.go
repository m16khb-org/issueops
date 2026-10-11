package responsecontract

import (
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
)

func NormalizeContractValue(value any, replacements map[string]string) any {
	switch v := value.(type) {
	case map[string]any:
		return normalizeContractMap(v, replacements)
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, NormalizeContractValue(child, replacements))
		}
		return out
	case string:
		return normalizeContractString(v, replacements)
	default:
		return v
	}
}

// contractMapKind는 맵 전체의 모양에서 결정되는, 키 "bytes"/"duration_ms" 치환 조건이다.
type contractMapKind struct {
	stateCheckpoint bool
	stateRecord     bool // dynamic state record 또는 history entry
	commandRun      bool
}

func normalizeContractMap(v map[string]any, replacements map[string]string) map[string]any {
	out := make(map[string]any, len(v))
	kind := contractMapKind{
		stateCheckpoint: looksLikeStateCheckpoint(v),
		stateRecord:     looksLikeDynamicStateRecord(v) || looksLikeDynamicStateHistoryEntry(v),
		commandRun:      looksLikeCommandRunResult(v),
	}
	for key, child := range v {
		if placeholder, ok := contractPlaceholder(key, child, kind); ok {
			out[key] = placeholder
			continue
		}
		out[key] = NormalizeContractValue(child, replacements)
	}
	return out
}

// contractPlaceholder returns the fixed replacement for a volatile map entry, if any.
func contractPlaceholder(key string, child any, kind contractMapKind) (any, bool) {
	if isDynamicTimeKey(key) {
		return "$TIMESTAMP", true
	}
	switch key {
	case "duration_ms":
		if kind.commandRun {
			return "$DURATION_MS", true
		}
	case "bytes":
		if kind.stateCheckpoint {
			return "$STATE_BYTES", true
		}
		if kind.stateRecord {
			return "$STATE_RECORD_BYTES", true
		}
	case "audit_log_id":
		return "$AUDIT_ID", true
	case "project_claude_skill", "project_codex_skill":
		// Repo-local host integrations are gitignored install artifacts. Their
		// presence is machine state, not committed response-contract behavior.
		return "$PROJECT_SKILL_PRESENCE", true
	case "id":
		return contractIDPlaceholder(child)
	case "pid":
		return "$PID", true
	case "head", "sha":
		return "$GIT_SHA", true
	case "score":
		if score, ok := child.(float64); ok {
			return math.Round(score*100) / 100, true
		}
	}
	return nil, false
}

func contractIDPlaceholder(child any) (any, bool) {
	s, ok := child.(string)
	if !ok {
		return nil, false
	}
	if strings.HasPrefix(s, "job-") {
		return "$WORKER_JOB_ID", true
	}
	if strings.HasPrefix(s, "io-") {
		return "$ISSUEOPS_ID", true
	}
	return nil, false
}

func looksLikeDynamicStateRecord(value map[string]any) bool {
	keyValue, ok := value["key"].(string)
	if !ok || !isDynamicStateRecordKey(keyValue) {
		return false
	}
	if _, ok := value["schema_version"]; !ok {
		return false
	}
	if _, ok := value["updated_at"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func isDynamicStateRecordKey(key string) bool {
	return strings.HasPrefix(key, "self-augment-lesson-") ||
		strings.HasPrefix(key, "self-verify-candidates-") ||
		key == "self-verify-baseline" ||
		key == "self-verify-candidate" ||
		key == "self-verify-promoted"
}

func looksLikeDynamicStateHistoryEntry(value map[string]any) bool {
	keyValue, ok := value["key"].(string)
	if !ok || !isDynamicStateRecordKey(keyValue) {
		return false
	}
	if _, ok := value["generated_at"]; !ok {
		return false
	}
	if _, ok := value["updated_at"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func looksLikeStateCheckpoint(value map[string]any) bool {
	if _, ok := value["state_dir"]; !ok {
		return false
	}
	if _, ok := value["path"]; !ok {
		return false
	}
	if _, ok := value["key"]; !ok {
		return false
	}
	if _, ok := value["bytes"]; !ok {
		return false
	}
	return true
}

func looksLikeCommandRunResult(value map[string]any) bool {
	if _, ok := value["executed"]; !ok {
		return false
	}
	if _, ok := value["exit_code"]; !ok {
		return false
	}
	if _, ok := value["started_at"]; !ok {
		return false
	}
	if _, ok := value["finished_at"]; !ok {
		return false
	}
	if _, ok := value["duration_ms"]; !ok {
		return false
	}
	if _, ok := value["policy"]; !ok {
		return false
	}
	return true
}

var gitSubjectPrefixRe = regexp.MustCompile(`^[0-9a-f]{7,40} `)

func normalizeContractString(value string, replacements map[string]string) string {
	keys := make([]string, 0, len(replacements))
	for from := range replacements {
		if from != "" {
			keys = append(keys, from)
		}
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, from := range keys {
		value = strings.ReplaceAll(value, from, replacements[from])
	}
	if gitSubjectPrefixRe.MatchString(value) {
		return "$GIT_SHA " + strings.TrimSpace(gitSubjectPrefixRe.ReplaceAllString(value, ""))
	}
	if strings.HasPrefix(value, "IssueOps remote reflect-devils-advocate failed: gh issue view failed:") &&
		(strings.Contains(value, "gh auth login") ||
			strings.Contains(value, "GitHub CLI") ||
			strings.Contains(value, "GH_TOKEN")) {
		return "IssueOps remote reflect-devils-advocate failed: gh issue view failed: $GH_AUTH_ERROR"
	}
	if isRFC3339Like(value) {
		return "$TIMESTAMP"
	}
	return value
}

func NormalizeMCPTextJSON(value any, replacements map[string]string) any {
	switch v := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(v))
		for key, child := range v {
			if key == "text" {
				if text, ok := child.(string); ok {
					var nested any
					if err := json.Unmarshal([]byte(text), &nested); err == nil {
						out["json"] = NormalizeContractValue(nested, replacements)
						continue
					}
				}
			}
			out[key] = NormalizeMCPTextJSON(child, replacements)
		}
		return out
	case []any:
		out := make([]any, 0, len(v))
		for _, child := range v {
			out = append(out, NormalizeMCPTextJSON(child, replacements))
		}
		return out
	default:
		return v
	}
}

func isDynamicTimeKey(key string) bool {
	switch key {
	case "updated_at", "generated_at", "cutoff", "started_at", "finished_at":
		return true
	default:
		return false
	}
}

func isRFC3339Like(value string) bool {
	if _, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	return false
}
