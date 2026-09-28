package issueopscompletion

import (
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"

	completioncontract "issueops/internal/contract/issueopscompletion"
)

func PrepareEvidence(confirm bool, values []string, remoteURL string) ([]string, error) {
	if !confirm {
		return nil, fmt.Errorf("execution complete requires confirm")
	}
	verification, err := normalizeVerification(values)
	if err != nil {
		return nil, err
	}
	if err := validateRemoteArtifactURL(remoteURL); err != nil {
		return nil, err
	}
	return verification, nil
}

func ValidatePrepared(prepared bool) error {
	if !prepared {
		return completioncontract.ErrExecutionNotPrepared
	}
	return nil
}

func ValidatePhase(phase string) error {
	if phase != "pr" {
		return fmt.Errorf("execution completion requires pr phase")
	}
	return nil
}

func ValidateFinalHead(requested, observed string) error {
	if !validFullCommitSHA(requested) || !strings.EqualFold(strings.TrimSpace(requested), observed) {
		return fmt.Errorf("final_head must match canonical worktree HEAD")
	}
	return nil
}

func normalizeVerification(values []string) ([]string, error) {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, fmt.Errorf("verification entries must be nonempty")
		}
		result = append(result, value)
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("execution completion requires verification evidence")
	}
	return result, nil
}

func validateRemoteArtifactURL(raw string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.Path == "" {
		return fmt.Errorf("execution completion requires an HTTPS draft PR or MR URL")
	}
	return nil
}

func validFullCommitSHA(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
