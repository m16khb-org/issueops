package github

import (
	"context"
	"encoding/json"
	"fmt"
	"issueops/internal/adapter/provider/providerutil"
	"issueops/internal/domain/issueopsremote"
	"issueops/internal/port"
	"strconv"
)

// FindChildCreateCandidates finds issues carrying the create marker; a child
// is an ordinary issue, so it shares FindIssueCreateCandidates.
func (p Provider) FindChildCreateCandidates(ctx context.Context, req port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error) {
	return p.FindIssueCreateCandidates(ctx, req)
}

// ReadChild reads the child issue and its parent's sub-issue list and returns
// a snapshot with TypeVerified (not a pull request) and HierarchyVerified
// (listed under the parent). It fails when the URLs do not belong together or
// the fetched identity does not match childURL.
func (p Provider) ReadChild(ctx context.Context, repo, parentURL, childURL string) (port.ChildSnapshot, error) {
	var result port.ChildSnapshot
	if err := issueopsremote.ValidateChildMatchesParent(parentURL, childURL); err != nil {
		return result, err
	}
	owner, name, parent, err := parseGitHubIssueURL(parentURL)
	if err != nil {
		return result, err
	}
	_, _, child, err := parseGitHubIssueURL(childURL)
	if err != nil {
		return result, err
	}
	issue, err := childReadAPI[githubChildSnapshot](ctx, repo, "repos/"+owner+"/"+name+"/issues/"+child)
	if err != nil {
		return result, err
	}
	if issue.ID == 0 || strconv.Itoa(issue.Number) != child || issue.HTMLURL != childURL {
		return result, fmt.Errorf("child lookup identity mismatch")
	}
	children, err := childReadAPI[[]githubIssue](ctx, repo, "repos/"+owner+"/"+name+"/issues/"+parent+"/sub_issues?per_page=100")
	if err != nil {
		return result, err
	}
	return port.ChildSnapshot{URL: issue.HTMLURL, Title: issue.Title, Body: issue.Body, TypeVerified: len(issue.PullRequest) == 0, HierarchyVerified: githubIssueListContains(children, issue.ID, child), Labels: githubLabelNames(issue.Labels), Assignees: githubAssigneeLogins(issue.Assignees)}, nil
}

type githubChildSnapshot struct {
	githubIssue
	Title       string          `json:"title"`
	Body        string          `json:"body"`
	PullRequest json.RawMessage `json:"pull_request"`
}

func childReadAPI[T any](ctx context.Context, repo, endpoint string) (T, error) {
	var result T
	out, err := providerutil.RunBoundedReadbackContext(ctx, repo, "gh", "api", endpoint)
	if err != nil {
		return result, err
	}
	err = json.Unmarshal(out, &result)
	return result, err
}

// AttachChild links childURL as a sub-issue of parentURL after checking the
// fetched issue identity. It never replaces an existing different parent.
func (p Provider) AttachChild(ctx context.Context, repo, parentURL, childURL string) error {
	if err := issueopsremote.ValidateChildMatchesParent(parentURL, childURL); err != nil {
		return err
	}
	owner, name, parent, err := parseGitHubIssueURL(parentURL)
	if err != nil {
		return err
	}
	_, _, child, err := parseGitHubIssueURL(childURL)
	if err != nil {
		return err
	}
	issue, err := childReadAPI[githubIssue](ctx, repo, "repos/"+owner+"/"+name+"/issues/"+child)
	if err != nil {
		return err
	}
	if issue.ID == 0 || issue.HTMLURL != childURL {
		return fmt.Errorf("child attach identity mismatch")
	}
	// replace_parent is deliberately omitted: an existing different parent is never moved.
	_, _, err = providerutil.RunBoundedMutationContext(ctx, repo, "gh", "api", "-X", "POST", "repos/"+owner+"/"+name+"/issues/"+parent+"/sub_issues", "-f", "sub_issue_id="+strconv.FormatInt(issue.ID, 10))
	return err
}
