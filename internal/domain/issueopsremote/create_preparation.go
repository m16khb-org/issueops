package issueopsremote

import (
	"encoding/hex"
	"fmt"
	"strings"
)

// CreateAuthority holds the cycle facts remote create checks before it may write: provider, whether the phase is pr, whether an artifact already exists, whether the call confirms, whether an execution lease exists, and the current and expected lease generation.
type CreateAuthority struct {
	Provider           string
	PhasePR            bool
	Artifact           bool
	Confirm            bool
	Execution          bool
	Generation         uint64
	ExpectedGeneration uint64
}

// ValidateCreateAuthority returns the normalized provider and artifact kind (pr for github, mr for gitlab). It errors when the provider is unsupported, the cycle already has an artifact, the phase is not pr, or, when confirming, there is no execution lease or the lease generation is stale.
func ValidateCreateAuthority(facts CreateAuthority) (string, string, error) {
	provider := strings.ToLower(strings.TrimSpace(facts.Provider))
	kind := "pr"
	if provider == "gitlab" {
		kind = "mr"
	} else if provider != "github" {
		return "", "", fmt.Errorf("remote provider must be github or gitlab")
	}
	// Probe: the joint message did not say which half failed.
	if facts.Artifact {
		// QA (crashed create, reconciled): the shared "requires phase pr"
		// prefix read as a phase problem; lead with the cause.
		return "", "", fmt.Errorf("this cycle already has its PR/MR (remote_artifact.url in issueops status), so remote create refuses a second; follow issueops next --id ID --json")
	}
	if !facts.PhasePR {
		return "", "", fmt.Errorf("the cycle is not in phase pr yet, so remote create refuses; follow issueops next --id ID --json until it routes to create-pr")
	}
	if facts.Confirm {
		if !facts.Execution {
			return "", "", fmt.Errorf("remote create requires IssueOps execution v1")
		}
		if facts.ExpectedGeneration == 0 || facts.Generation != facts.ExpectedGeneration {
			return "", "", fmt.Errorf("stale lease generation: current=%d expected=%d", facts.Generation, facts.ExpectedGeneration)
		}
	}
	return provider, kind, nil
}

// ValidateCreatePending refuses create while an earlier external intent is pending; execution reconcile must settle it first.
func ValidateCreatePending(pending bool) error {
	if pending {
		return fmt.Errorf("external intent is already pending; run execution reconcile")
	}
	return nil
}

// ValidateCreateReview requires a passing implementation review before create. missing carries the reason no review qualifies (id is used in the remediation hint), and a sealed cycle without a current fingerprint cannot prove the review is fresh.
func ValidateCreateReview(id, missing, currentFingerprint string, sealed bool) error {
	if missing != "" {
		return fmt.Errorf("remote create requires a pass implementation review (%s); record it with `issueops implementation-review record --id %s ...`", missing, id)
	}
	if currentFingerprint == "" && sealed {
		return fmt.Errorf("remote create cannot verify implementation review freshness (current_fingerprint unavailable)")
	}
	return nil
}

// CreateBranchAuthority is the sealed branch-preparation authority a create request must match: whether preparation ran, provider, workspace and base branches, linked issue URL, and code project key.
type CreateBranchAuthority struct {
	Prepared        bool
	Provider        string
	WorkspaceBranch string
	BaseBranch      string
	IssueURL        string
	CodeProjectKey  string
}

// CreateRequest is a PR/MR create request: provider, project key, head and base branches, title, body, labels, and assignees.
type CreateRequest struct {
	Provider   string
	ProjectKey string
	Head       string
	Base       string
	Title      string
	Body       string
	Labels     []string
	Assignees  []string
}

// PrepareCreateRequest normalizes request against authority and returns it with the project key resolved. It requires prior branch preparation, provider, head and base matching the authority, a title and a body within provider limits that are not secretLike, and non-empty labels and non-placeholder assignees.
func PrepareCreateRequest(authority CreateBranchAuthority, request CreateRequest, secretLike bool) (CreateRequest, error) {
	if !authority.Prepared {
		return CreateRequest{}, fmt.Errorf("remote create requires branch preparation")
	}
	request.ProjectKey = EffectiveProjectKey(authority.CodeProjectKey, authority.IssueURL, request.Provider)
	request.Head, request.Base = strings.TrimSpace(request.Head), strings.TrimSpace(request.Base)
	if request.ProjectKey == "" || request.Provider != strings.ToLower(strings.TrimSpace(authority.Provider)) || request.Head != authority.WorkspaceBranch || request.Base != strings.TrimSpace(authority.BaseBranch) {
		return CreateRequest{}, createAuthorityMismatch(authority, request)
	}
	request.Title, request.Body = strings.TrimSpace(request.Title), strings.TrimSpace(request.Body)
	if err := validateCreateContent(request, secretLike); err != nil {
		return CreateRequest{}, err
	}
	request.Labels, request.Assignees = CleanValues(request.Labels), CleanValues(request.Assignees)
	if len(request.Labels) == 0 || len(request.Assignees) == 0 {
		return CreateRequest{}, fmt.Errorf("remote create requires canonical labels and assignees")
	}
	if invalid := invalidAssignee(request.Assignees); invalid != "" {
		return CreateRequest{}, fmt.Errorf("remote create assignee must not be placeholder %q", invalid)
	}
	return request, nil
}

func validateCreateContent(request CreateRequest, secretLike bool) error {
	if request.Title == "" || len(request.Body) > 1<<20 {
		return fmt.Errorf("remote create title is required and body must not exceed 1048576 bytes")
	}
	if err := ValidateProviderTitleLength("PR/MR", request.Title); err != nil {
		return err
	}
	if err := ValidateProviderBodyLength(request.Provider, "PR/MR", request.Body); err != nil {
		return err
	}
	if secretLike {
		return fmt.Errorf("remote create title or body contains secret-like content")
	}
	return nil
}

// ValidateCreateHead requires head to be a full hex commit ID (40 or 64 characters).
func ValidateCreateHead(head string) error {
	if len(head) == 40 || len(head) == 64 {
		if _, err := hex.DecodeString(head); err == nil {
			return nil
		}
	}
	return fmt.Errorf("remote create requires a resolvable canonical worktree HEAD")
}

// createAuthorityMismatch names the fields that differ. QA: --base main
// against a cycle prepared on master read only "does not match execution
// workspace and linked issue authority".
func createAuthorityMismatch(authority CreateBranchAuthority, request CreateRequest) error {
	var fields []string
	if request.ProjectKey == "" {
		fields = append(fields, "project key unresolved")
	}
	if provider := strings.ToLower(strings.TrimSpace(authority.Provider)); request.Provider != provider {
		fields = append(fields, fmt.Sprintf("provider %q, prepared %q", request.Provider, provider))
	}
	if request.Head != authority.WorkspaceBranch {
		fields = append(fields, fmt.Sprintf("head %q, workspace branch %q", request.Head, authority.WorkspaceBranch))
	}
	if base := strings.TrimSpace(authority.BaseBranch); request.Base != base {
		fields = append(fields, fmt.Sprintf("base %q, prepared base %q (to change the base, see the issueops-prepare base-change reference)", request.Base, base))
	}
	return fmt.Errorf("remote create request does not match execution workspace and linked issue authority: %s", strings.Join(fields, "; "))
}
