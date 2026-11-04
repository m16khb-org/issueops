package issueopsbodysync

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/issueopsbodysync"
)

// TargetSnapshot is the cycle's linked issue URL and verified PR/MR artifact URL and kind that a sync target is resolved against.
type TargetSnapshot struct {
	IssueURL     string
	ArtifactURL  string
	ArtifactKind string
}

// ResolveTarget maps a sync command to the artifact kind and URL to edit. An issue command targets the cycle issue, or a child when a different URL is given. A PR command must match the cycle's verified artifact and returns its PR or MR kind. It errors when the needed link or artifact is missing or the kind is unsupported.
func ResolveTarget(snapshot TargetSnapshot, command contract.Command) (kind, url string, err error) {
	requested := strings.TrimSpace(command.URL)
	switch command.Kind {
	case contract.KindIssue:
		parent := strings.TrimSpace(snapshot.IssueURL)
		if parent == "" {
			return "", "", fmt.Errorf("cannot sync an issue body before the cycle has a linked issue")
		}
		if requested == "" || sameArtifactURL(requested, parent) {
			return contract.KindIssue, parent, nil
		}
		return contract.KindChild, requested, nil
	case contract.KindPR:
		artifact := strings.TrimSpace(snapshot.ArtifactURL)
		if artifact == "" {
			return "", "", fmt.Errorf("cannot sync a PR/MR body before the cycle has a verified remote artifact")
		}
		if requested != "" && !sameArtifactURL(requested, artifact) {
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

// IsPublicationKind reports whether kind is a PR or an MR.
func IsPublicationKind(kind string) bool {
	return kind == contract.KindPR || kind == contract.KindMR
}

func sameArtifactURL(left, right string) bool {
	normalize := func(raw string) string {
		trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
		return strings.Replace(trimmed, "/-/work_items/", "/-/issues/", 1)
	}
	return normalize(left) == normalize(right)
}

// ValidateGeneration rejects a sync made without a prepared execution lease or with a lease generation that differs from the expected one; an expected generation of 0 is always stale.
func ValidateGeneration(prepared bool, current, expected uint64) error {
	if !prepared {
		return fmt.Errorf("cannot sync a PR/MR body without an execution lease")
	}
	if expected == 0 || current != expected {
		return fmt.Errorf("stale lease generation: current=%d expected=%d", current, expected)
	}
	return nil
}

// PostCompletion reports a completed cycle whose lease was released by
// `execution complete`. Its issue and PR/MR bodies are edited from the main
// session without a lease, like merge-pr and reflect-completion: the
// completion sealed the head, not the bodies. QA: asked to add a section to a
// completed cycle's PR body, an agent reseeded the cycle (completion pushed to
// history, phase back to implement) only to get a lease for the edit.
func PostCompletion(phase, leaseStatus string, completed bool) bool {
	// Literal values: this domain package must not import contract/issueops.
	return phase == "done" && leaseStatus == "released" && completed
}

// RejectClosedPublication rejects rewriting a PR or MR body unless its observed state is open, opened or locked; an empty state is also rejected. Other kinds always pass.
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
