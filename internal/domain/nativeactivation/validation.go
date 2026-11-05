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

// ValidateRequest rejects a request whose state root, harness root or target binary is empty or has surrounding whitespace, or whose non-empty transition ID is malformed.
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

// ValidTransitionID reports whether value is a lowercase hex encoding of 16 bytes.
func ValidTransitionID(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 16 && value == strings.ToLower(value)
}

// ValidSHA256 reports whether value is a lowercase hex SHA-256 digest with no surrounding whitespace.
func ValidSHA256(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size && value == strings.TrimSpace(value) && value == strings.ToLower(value)
}

// ValidTimestamp reports whether value is an RFC 3339 timestamp with nanoseconds in its canonical UTC form.
func ValidTimestamp(value string) bool {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	return err == nil && parsed.UTC().Format(time.RFC3339Nano) == value
}

// ReadbackOrder validates the readback evidence and returns the indices that sort it by host and surface. The catalog digest must be valid and evidence must hold exactly one valid readback, with distinct paths, for each of the eleven first-party MCP and hook surfaces.
func ReadbackOrder(catalogSHA256 string, evidence []activationcontract.Evidence) ([]int, error) {
	if !ValidSHA256(catalogSHA256) {
		return nil, fmt.Errorf("native activation readback digest is invalid")
	}
	expected := expectedReadbackSurfaces()
	paths := map[string]bool{}
	for _, item := range evidence {
		key := item.Host + "\x00" + item.Surface
		if !validReadbackEvidence(item, key, expected, paths) {
			return nil, fmt.Errorf("native activation requires one valid readback for each first-party MCP/hook surface")
		}
		delete(expected, key)
		paths[item.Path] = true
	}
	if len(expected) != 0 || len(evidence) != 11 {
		return nil, fmt.Errorf("native activation requires exactly eleven first-party MCP/hook readbacks")
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

func expectedReadbackSurfaces() map[string]bool {
	return map[string]bool{
		"codex\x00mcp": true, "codex\x00hooks": true,
		"claude\x00mcp": true, "claude\x00hooks": true,
		"omo\x00mcp": true, "omo\x00hooks": true,
		"omp\x00mcp": true, "omp\x00hooks": true,
		"opencode\x00mcp": true, "opencode\x00hooks": true,
		"agy\x00mcp": true,
	}
}

func validReadbackEvidence(item activationcontract.Evidence, key string, expected, paths map[string]bool) bool {
	return item.Host == strings.TrimSpace(item.Host) && item.Surface == strings.TrimSpace(item.Surface) &&
		item.Path == strings.TrimSpace(item.Path) && expected[key] && item.Path != "" && !paths[item.Path] &&
		ValidSHA256(item.SemanticSHA256) && ValidSHA256(item.SHA256)
}
