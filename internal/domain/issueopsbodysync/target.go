package issueopsbodysync

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/issueopsbodysync"
)

type TargetSnapshot struct {
	IssueURL     string
	ArtifactURL  string
	ArtifactKind string
}

func ResolveTarget(snapshot TargetSnapshot, command contract.Command) (kind, url string, err error) {
	requested := strings.TrimSpace(command.URL)
	switch command.Kind {
	case contract.KindIssue:
		parent := strings.TrimSpace(snapshot.IssueURL)
		if parent == "" {
			return "", "", fmt.Errorf("cannot sync an issue body before the cycle has a linked issue")
		}
		if requested == "" || SameArtifactURL(requested, parent) {
			return contract.KindIssue, parent, nil
		}
		return contract.KindChild, requested, nil
	case contract.KindPR:
		artifact := strings.TrimSpace(snapshot.ArtifactURL)
		if artifact == "" {
			return "", "", fmt.Errorf("cannot sync a PR/MR body before the cycle has a verified remote artifact")
		}
		if requested != "" && !SameArtifactURL(requested, artifact) {
			return "", "", fmt.Errorf("--url %s is not this cycle's verified artifact (%s)", requested, snapshot.ArtifactURL)
		}
		resolved := strings.TrimSpace(snapshot.ArtifactKind)
		if !IsPublicationKind(resolved) {
			return "", "", fmt.Errorf("verified remote artifact kind %q is not a PR or MR", snapshot.ArtifactKind)
		}
		return resolved, artifact, nil
	}
	return "", "", fmt.Errorf("unsupported body sync kind %q (want %s|%s)", command.Kind, contract.KindIssue, contract.KindPR)
}

func IsPublicationKind(kind string) bool {
	return kind == contract.KindPR || kind == contract.KindMR
}

func SameArtifactURL(left, right string) bool {
	normalize := func(raw string) string {
		trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
		return strings.Replace(trimmed, "/-/work_items/", "/-/issues/", 1)
	}
	return normalize(left) == normalize(right)
}

func ValidateGeneration(prepared bool, current, expected uint64) error {
	if !prepared {
		return fmt.Errorf("cannot sync a PR/MR body without an execution lease")
	}
	if expected == 0 || current != expected {
		return fmt.Errorf("stale lease generation: current=%d expected=%d", current, expected)
	}
	return nil
}

func RejectClosedPublication(kind, state string) error {
	if !IsPublicationKind(kind) {
		return nil
	}
	switch normalized := strings.ToLower(strings.TrimSpace(state)); normalized {
	case "open", "opened", "locked":
		return nil
	case "":
		return fmt.Errorf("refusing to rewrite a %s body without an observed artifact state", kind)
	default:
		return fmt.Errorf("refusing to rewrite the body of a %s artifact (state=%s)", kind, normalized)
	}
}
