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
		return nil, fmt.Errorf("execution complete requires confirm (--confirm; it records the completion and releases the lease, so check execution status first)")
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
	if !validFullCommitSHA(requested) {
		// QA: a short SHA read as "does not match" though it named HEAD.
		return fmt.Errorf("final_head must match canonical worktree HEAD: pass the full 40-hex SHA (git rev-parse HEAD in the worktree), got %q", strings.TrimSpace(requested))
	}
	if !strings.EqualFold(strings.TrimSpace(requested), observed) {
		return fmt.Errorf("final_head must match canonical worktree HEAD (%s); commit and push first, then pass that SHA", observed)
	}
	return nil
}

// ValidateVerifiedSincePR refuses a final head that changed anything other than the
// verification report since pr_head was sealed. The pr gate sealed that head, so any other
// change was never cleaned, documented, reviewed, or verified.
func ValidateVerifiedSincePR(prHead string, changed []string) error {
	if len(changed) == 0 {
		return nil
	}
	shown := changed
	if len(shown) > 5 {
		shown = shown[:5]
	}
	return fmt.Errorf("final_head changes %d path(s) after the verified pr head %s besides the verification report (%s); refresh cleanup with issueops phase --to ai-slop-clean, re-record the reviews and verification issueops next reports for the new head, reseal with issueops phase --to pr, then complete",
		len(changed), shortSHA(prHead), strings.Join(shown, ", "))
}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) > 10 {
		return sha[:10]
	}
	return sha
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
