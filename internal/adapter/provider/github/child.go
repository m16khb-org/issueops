package github

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"issueops/internal/adapter/provider/providerutil"
	"issueops/internal/port"
)

// githubChildTarget은 부모 이슈 URL에서 뽑아낸 저장소와 부모 번호다.
type githubChildTarget struct {
	owner        string
	repoName     string
	parentNumber string
}

func (t githubChildTarget) subIssuesPath() string {
	return "repos/" + t.owner + "/" + t.repoName + "/issues/" + t.parentNumber + "/sub_issues"
}

func (Provider) CreateChild(req port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, fmt.Errorf("child title is required")
	}
	owner, repoName, parentNumber, err := parseGitHubIssueURL(req.ParentIssueURL)
	if err != nil {
		return port.IssueProviderCreateChildResult{OK: false, Provider: "github"}, err
	}
	target := githubChildTarget{owner: owner, repoName: repoName, parentNumber: parentNumber}
	createArgs := githubChildCreateArgs(req, title, target)
	if !req.Confirm {
		return port.IssueProviderCreateChildResult{OK: true, Provider: "github", Preview: githubChildCreatePreview(createArgs, target)}, nil
	}
	help, err := providerutil.RunBoundedReadbackContext(context.Background(), req.Repo, "gh", "issue", "create", "--help")
	if err != nil {
		return port.IssueProviderCreateChildResult{Provider: "github"}, &port.IssueProviderCreateError{Invoked: false, Err: err}
	}
	usedParentFlag, knownCreateHelp := ghCreateHelpCapabilities(string(help))
	if !usedParentFlag && !knownCreateHelp {
		return port.IssueProviderCreateChildResult{Provider: "github"}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("cannot determine gh issue create parent capability")}
	}
	if usedParentFlag {
		createArgs = append(createArgs, "--parent", parentNumber)
	}
	child, err := runGhJSON(createArgs, req.Repo, "issue")
	if err != nil {
		return port.IssueProviderCreateChildResult{Provider: "github", ChildURL: strings.TrimSpace(child.URL)}, err
	}
	return verifyCreatedGitHubChild(req, target, strings.TrimSpace(child.URL), usedParentFlag)
}

// githubChildCreateArgs는 `gh issue create`의 --parent 이전 인자를 만든다.
func githubChildCreateArgs(req port.IssueProviderCreateChildRequest, title string, target githubChildTarget) []string {
	createArgs := []string{"issue", "create", "--title", title}
	if body := strings.TrimSpace(req.Body); body != "" {
		createArgs = append(createArgs, "--body", body)
	}
	for _, label := range req.Labels {
		createArgs = append(createArgs, "--label", label)
	}
	for _, assignee := range req.Assignees {
		createArgs = append(createArgs, "--assignee", assignee)
	}
	return append(createArgs, "--repo", target.owner+"/"+target.repoName)
}

func githubChildCreatePreview(createArgs []string, target githubChildTarget) string {
	preferred := append(append([]string{}, createArgs...), "--parent", target.parentNumber)
	return strings.Join([]string{
		providerutil.DryRunPreview("gh", "issue", "create", "--help"),
		"if parent flag is supported: " + providerutil.DryRunPreview("gh", preferred...),
		providerutil.DryRunPreview("gh", "api", target.subIssuesPath()),
		"if parent flag is absent from valid help: " + providerutil.DryRunPreview("gh", createArgs...),
		providerutil.DryRunPreview("gh", "api", "-X", "POST", target.subIssuesPath(), "-f", "sub_issue_id={child_database_id}"),
	}, "; ") + "; choose one create path before mutation; never retry a failed create"
}

// ghCreateHelpCapabilities는 `gh issue create --help` 출력의 플래그 줄에서
// --parent 지원 여부와 --title 존재(=도움말 형식을 알아봤는지)를 읽는다.
func ghCreateHelpCapabilities(help string) (usedParentFlag, knownCreateHelp bool) {
	for line := range strings.SplitSeq(help, "\n") {
		fields := strings.Fields(line)
		if len(fields) > 0 && strings.HasPrefix(fields[0], "-") {
			for _, field := range fields {
				if field == "--parent" {
					usedParentFlag = true
				}
				if field == "--title" {
					knownCreateHelp = true
				}
			}
		}
	}
	return usedParentFlag, knownCreateHelp
}

// githubChildFollowUpFailure는 자식이 만들어진 뒤 후속 단계가 실패한 결과를 만든다.
func githubChildFollowUpFailure(childURL string, err error) (port.IssueProviderCreateChildResult, error) {
	return port.IssueProviderCreateChildResult{OK: false, Provider: "github", ChildURL: childURL}, githubCreatedChildError(childURL, err)
}

// verifyCreatedGitHubChild는 생성된 자식의 조회, (필요 시) 부모 연결,
// 계층·라벨·담당자 검증을 순서대로 수행한다.
func verifyCreatedGitHubChild(req port.IssueProviderCreateChildRequest, target githubChildTarget, childURL string, usedParentFlag bool) (port.IssueProviderCreateChildResult, error) {
	_, _, childNumber, err := parseGitHubIssueURL(childURL)
	if err != nil {
		return githubChildFollowUpFailure(childURL, fmt.Errorf("parse created child issue URL: %w", err))
	}
	childIssue, err := runGhAPIJSON[githubIssue](req.Repo, []string{"repos/" + target.owner + "/" + target.repoName + "/issues/" + childNumber}, "issue lookup")
	if err != nil {
		return githubChildFollowUpFailure(childURL, err)
	}
	if childIssue.ID == 0 {
		return githubChildFollowUpFailure(childURL, fmt.Errorf("created child issue is missing database id"))
	}
	if !usedParentFlag {
		_, err = runGhAPIJSON[map[string]any](req.Repo, []string{"-X", "POST", target.subIssuesPath(), "-f", "sub_issue_id=" + strconv.FormatInt(childIssue.ID, 10)}, "sub-issue attach")
		if err != nil {
			return githubChildFollowUpFailure(childURL, err)
		}
	}
	children, err := runGhAPIJSON[[]githubIssue](req.Repo, []string{target.subIssuesPath()}, "sub-issue verification")
	if err != nil {
		return githubChildFollowUpFailure(childURL, err)
	}
	if !githubIssueListContains(children, childIssue.ID, childNumber) {
		return githubChildFollowUpFailure(childURL, fmt.Errorf("github sub-issue hierarchy verification failed"))
	}
	labels := githubLabelNames(childIssue.Labels)
	assignees := githubAssigneeLogins(childIssue.Assignees)
	if missing := providerutil.MissingStrings(req.Labels, labels); len(missing) > 0 {
		return githubChildFollowUpFailure(childURL, fmt.Errorf("github child issue missing labels: %s", strings.Join(missing, ", ")))
	}
	if missing := providerutil.MissingStrings(req.Assignees, assignees); len(missing) > 0 {
		return githubChildFollowUpFailure(childURL, fmt.Errorf("github child issue missing assignees: %s", strings.Join(missing, ", ")))
	}
	return port.IssueProviderCreateChildResult{
		OK:                true,
		Provider:          "github",
		ChildURL:          providerutil.FirstNonEmpty(childIssue.HTMLURL, childURL),
		ChildNumber:       childNumber,
		HierarchyVerified: true,
		Labels:            labels,
		Assignees:         assignees,
	}, nil
}

func (Provider) CloseChild(req port.IssueProviderCloseChildRequest) (port.IssueProviderCloseChildResult, error) {
	owner, repoName, parentNumber, err := parseGitHubIssueURL(req.ParentIssueURL)
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
	}
	childOwner, childRepo, childNumber, err := parseGitHubIssueURL(req.ChildURL)
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
	}
	if childOwner != owner || childRepo != repoName {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, fmt.Errorf("child issue url must match linked parent issue project")
	}
	target := githubChildTarget{owner: owner, repoName: repoName, parentNumber: parentNumber}
	if !req.Confirm {
		return previewCloseGitHubChild(req, target, childNumber), nil
	}
	children, err := runGhAPIJSON[[]githubIssue](req.Repo, []string{target.subIssuesPath()}, "sub-issue verification")
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
	}
	childIssue := githubIssueByNumber(children, childNumber)
	if childIssue.Number == 0 {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, fmt.Errorf("github sub-issue hierarchy verification failed")
	}
	alreadyClosed := strings.EqualFold(childIssue.State, "closed")
	if !alreadyClosed {
		if _, err := runGhAPIJSON[githubIssue](req.Repo, []string{"-X", "PATCH", "repos/" + owner + "/" + repoName + "/issues/" + childNumber, "-f", "state=closed", "-f", "state_reason=completed"}, "issue close"); err != nil {
			return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
		}
	}
	verified, err := runGhAPIJSON[githubIssue](req.Repo, []string{"repos/" + owner + "/" + repoName + "/issues/" + childNumber}, "issue close verification")
	if err != nil {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, err
	}
	if !strings.EqualFold(verified.State, "closed") {
		return port.IssueProviderCloseChildResult{OK: false, Provider: "github"}, fmt.Errorf("github child issue close verification failed: state=%s", verified.State)
	}
	return port.IssueProviderCloseChildResult{
		OK:                true,
		Provider:          "github",
		ChildURL:          providerutil.FirstNonEmpty(verified.HTMLURL, childIssue.HTMLURL, req.ChildURL),
		HierarchyVerified: true,
		Closed:            true,
		AlreadyClosed:     alreadyClosed,
		State:             verified.State,
	}, nil
}

// previewCloseGitHubChild는 dry-run 미리보기를 만든다.
//
// preview는 무엇을 할지 보여주는 것에 더해 지금 상태가 무엇인지도
// 관측한다. cleanup close-children이 부모 머지 증거 없이 정리해도
// 되는지 판정하려면 자식이 원격에서 이미 닫혔는지를 알아야 한다(#129).
//
// 읽기 전용이며 best-effort다. gh가 없거나 조회가 실패하면 상태를 비워
// 두고 성공을 돌려준다 — 여기서 오류를 내면 provider 없는 환경의
// dry-run이 통째로 깨지고, core는 어차피 미상을 통과 근거로 인정하지
// 않는다.
func previewCloseGitHubChild(req port.IssueProviderCloseChildRequest, target githubChildTarget, childNumber string) port.IssueProviderCloseChildResult {
	owner, repoName, parentNumber := target.owner, target.repoName, target.parentNumber
	preview := fmt.Sprintf("[dry-run] would execute: gh api repos/%s/%s/issues/%s/sub_issues; gh api -X PATCH repos/%s/%s/issues/%s -f state=closed -f state_reason=completed; gh api repos/%s/%s/issues/%s",
		owner, repoName, parentNumber, owner, repoName, childNumber, owner, repoName, childNumber)
	result := port.IssueProviderCloseChildResult{OK: true, Provider: "github", ChildURL: strings.TrimSpace(req.ChildURL), Preview: preview}
	if children, err := runGhAPIJSON[[]githubIssue](req.Repo, []string{target.subIssuesPath()}, "sub-issue observation"); err == nil {
		if childIssue := githubIssueByNumber(children, childNumber); childIssue.Number != 0 {
			result.HierarchyVerified = true
			result.State = childIssue.State
			result.AlreadyClosed = strings.EqualFold(childIssue.State, "closed")
			result.ChildURL = providerutil.FirstNonEmpty(childIssue.HTMLURL, result.ChildURL)
		}
	}
	return result
}

type githubIssue struct {
	ID        int64            `json:"id"`
	Number    int              `json:"number"`
	HTMLURL   string           `json:"html_url"`
	State     string           `json:"state"`
	Labels    []githubLabel    `json:"labels"`
	Assignees []githubAssignee `json:"assignees"`
}

type githubLabel struct {
	Name string `json:"name"`
}

type githubAssignee struct {
	Login string `json:"login"`
}

func githubCreatedChildError(childURL string, err error) error {
	childURL = strings.TrimSpace(childURL)
	if childURL != "" {
		err = fmt.Errorf("created child %s but follow-up failed: %w", childURL, err)
	}
	return &port.IssueProviderCreateError{Invoked: true, Err: err}
}

func parseGitHubIssueURL(raw string) (owner, repo, number string, err error) {
	raw = strings.TrimSpace(raw)
	parts := strings.Split(raw, "/")
	for i := 0; i+4 < len(parts); i++ {
		if parts[i] == "github.com" && parts[i+3] == "issues" {
			if parts[i+1] == "" || parts[i+2] == "" || parts[i+4] == "" {
				break
			}
			return parts[i+1], parts[i+2], parts[i+4], nil
		}
	}
	return "", "", "", fmt.Errorf("parent_issue_url must be a GitHub issue URL")
}

func githubIssueListContains(issues []githubIssue, id int64, number string) bool {
	for _, issue := range issues {
		if id != 0 && issue.ID == id {
			return true
		}
		if number != "" && strconv.Itoa(issue.Number) == number {
			return true
		}
	}
	return false
}

func githubIssueByNumber(issues []githubIssue, number string) githubIssue {
	for _, issue := range issues {
		if number != "" && strconv.Itoa(issue.Number) == number {
			return issue
		}
	}
	return githubIssue{}
}

func githubLabelNames(labels []githubLabel) []string {
	out := make([]string, 0, len(labels))
	for _, label := range labels {
		if strings.TrimSpace(label.Name) != "" {
			out = append(out, label.Name)
		}
	}
	return out
}

func githubAssigneeLogins(assignees []githubAssignee) []string {
	out := make([]string, 0, len(assignees))
	for _, assignee := range assignees {
		if strings.TrimSpace(assignee.Login) != "" {
			out = append(out, assignee.Login)
		}
	}
	return out
}
