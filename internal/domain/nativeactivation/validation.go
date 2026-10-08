package nativeactivation

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	activationcontract "issueops/internal/contract/nativeactivation"
)

func ValidateRequest(request activationcontract.Request) error {
	if strings.TrimSpace(request.StateRoot) == "" || strings.TrimSpace(request.IssueOpsRoot) == "" || strings.TrimSpace(request.TargetBinary) == "" ||
		request.StateRoot != strings.TrimSpace(request.StateRoot) || request.IssueOpsRoot != strings.TrimSpace(request.IssueOpsRoot) || request.TargetBinary != strings.TrimSpace(request.TargetBinary) {
		return fmt.Errorf("native activation state root, harness root, and target binary are required")
	}
	if request.TransitionID != "" && !ValidTransitionID(request.TransitionID) {
		return fmt.Errorf("native activation transition ID is invalid")
	}
	return nil
}

func ValidTransitionID(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 16 && value == strings.ToLower(value)
}

func ValidSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.TrimSpace(value) && value == strings.ToLower(value)
}

func ValidTimestamp(value string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && parsed.UTC().Format(time.RFC3339Nano) == value
}

func ReadbackOrder(catalogSHA256 string, evidence []activationcontract.Evidence) ([]int, error) {
	if !ValidSHA256(catalogSHA256) {
		return nil, fmt.Errorf("native activation readback digest is invalid")
	}
	expected := map[string]bool{
		"codex\x00mcp": true, "codex\x00hooks": true,
		"claude\x00mcp": true, "claude\x00hooks": true,
		"omo\x00mcp": true, "omo\x00hooks": true,
		"omp\x00mcp": true, "omp\x00hooks": true,
		"agy\x00mcp": true,
	}
	paths := map[string]bool{}
	for _, item := range evidence {
		key := item.Host + "\x00" + item.Surface
		if item.Host != strings.TrimSpace(item.Host) || item.Surface != strings.TrimSpace(item.Surface) ||
			item.Path != strings.TrimSpace(item.Path) || !expected[key] || item.Path == "" || paths[item.Path] ||
			!ValidSHA256(item.SemanticSHA256) || !ValidSHA256(item.SHA256) {
			return nil, fmt.Errorf("native activation requires one valid readback for each first-party MCP/hook surface")
		}
		delete(expected, key)
		paths[item.Path] = true
	}
	if len(expected) != 0 || len(evidence) != 9 {
		return nil, fmt.Errorf("native activation requires exactly nine first-party MCP/hook readbacks")
	}
	order := make([]int, len(evidence))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(left, right int) bool {
		a, b := evidence[order[left]], evidence[order[right]]
		return a.Host+"\x00"+a.Surface < b.Host+"\x00"+b.Surface
	})
	return order, nil
}
