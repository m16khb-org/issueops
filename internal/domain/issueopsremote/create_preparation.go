package remote

import (
	"encoding/hex"
	"fmt"
	"strings"
)

type CreateAuthority struct {
	Provider           string
	PhasePR            bool
	Artifact           bool
	Confirm            bool
	Execution          bool
	Generation         uint64
	ExpectedGeneration uint64
}

func ValidateCreateAuthority(facts CreateAuthority) (string, string, error) {
	provider := strings.ToLower(strings.TrimSpace(facts.Provider))
	kind := "pr"
	if provider == "gitlab" {
		kind = "mr"
	} else if provider != "github" {
		return "", "", fmt.Errorf("remote provider must be github or gitlab")
	}
	if !facts.PhasePR || facts.Artifact {
		return "", "", fmt.Errorf("remote create requires phase pr and no existing remote artifact")
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

func ValidateCreatePending(pending bool) error {
	if pending {
		return fmt.Errorf("external intent is already pending; run execution reconcile")
	}
	return nil
}

func ValidateCreateReview(id, missing, currentFingerprint string, sealed bool) error {
	if missing != "" {
		return fmt.Errorf("remote create requires a pass implementation review (%s); record it with `issueops implementation-review record --id %s ...`", missing, id)
	}
	if currentFingerprint == "" && sealed {
		return fmt.Errorf("remote create cannot verify implementation review freshness (current_fingerprint unavailable)")
	}
	return nil
}

type CreateBranchAuthority struct {
	Prepared        bool
	Provider        string
	WorkspaceBranch string
	BaseBranch      string
	IssueURL        string
	CodeProjectKey  string
}

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

func PrepareCreateRequest(authority CreateBranchAuthority, request CreateRequest, secretLike bool) (CreateRequest, error) {
	if !authority.Prepared {
		return CreateRequest{}, fmt.Errorf("remote create requires branch preparation")
	}
	request.ProjectKey = EffectiveProjectKey(authority.CodeProjectKey, authority.IssueURL, request.Provider)
	request.Head, request.Base = strings.TrimSpace(request.Head), strings.TrimSpace(request.Base)
	if request.ProjectKey == "" || request.Provider != strings.ToLower(strings.TrimSpace(authority.Provider)) || request.Head != authority.WorkspaceBranch || request.Base != strings.TrimSpace(authority.BaseBranch) {
		return CreateRequest{}, fmt.Errorf("remote create request does not match execution workspace and linked issue authority")
	}
	request.Title, request.Body = strings.TrimSpace(request.Title), strings.TrimSpace(request.Body)
	if request.Title == "" || len(request.Title) > 1024 || len(request.Body) > 1<<20 {
		return CreateRequest{}, fmt.Errorf("remote create title is required and body must not exceed 1048576 bytes")
	}
	if secretLike {
		return CreateRequest{}, fmt.Errorf("remote create title or body contains secret-like content")
	}
	request.Labels, request.Assignees = CleanValues(request.Labels), CleanValues(request.Assignees)
	if len(request.Labels) == 0 || len(request.Assignees) == 0 {
		return CreateRequest{}, fmt.Errorf("remote create requires canonical labels and assignees")
	}
	if invalid := InvalidAssignee(request.Assignees); invalid != "" {
		return CreateRequest{}, fmt.Errorf("remote create assignee must not be placeholder %q", invalid)
	}
	return request, nil
}

func ValidateCreateHead(head string) error {
	if len(head) == 40 || len(head) == 64 {
		if _, err := hex.DecodeString(head); err == nil {
			return nil
		}
	}
	return fmt.Errorf("remote create requires a resolvable canonical worktree HEAD")
}
