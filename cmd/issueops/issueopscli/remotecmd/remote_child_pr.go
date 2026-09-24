package remotecmd

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"issueops/internal/domain/artifactreadability"
	artifacttemplate "issueops/internal/domain/artifacttemplate"
	issueopsremote "issueops/internal/domain/issueopsremote"
	port "issueops/internal/port"
	"os"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

func runRemoteCreateChild(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote create-child", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	title := fs.String("title", "", "child title")
	body := fs.String("body", "", "child body (markdown)")
	bodyFile := fs.String("body-file", "", "child body markdown file")
	template := fs.String("template", "", "template kind")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	scoreFile := fs.String("score-file", "", "IssueOps remote score result JSON")
	host := fs.String("host", "", "native holder host")
	sessionID := fs.String("session-id", "", "native holder session id")
	agentID := fs.String("agent-id", "", "optional native holder agent id")
	cwd := fs.String("cwd", "", "canonical holder worktree cwd")
	confirm := fs.Bool("confirm", false, "execute creation; without this, dry-run preview only")
	var labels repeatedFlag
	var assignees repeatedFlag
	var fields repeatedFlag
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&assignees, "assignee", "assignee username (repeatable)")
	fs.Var(&fields, "field", "template field key=value (canonical or documented alias; repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	labels, assignees = normalizeRemoteCreateMetadata(labels, assignees)
	record, err := remoteDeps.ReadIssueOps(remoteDeps.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if strings.TrimSpace(record.IssueURL) == "" {
		err := fmt.Errorf("cannot create child before linked parent issue")
		return deps.printErrorResult(*jsonOut, err)
	}
	// 우산 브랜치 게이트는 provider 호출 이전에 선다. 자식이 만들어진 뒤에는
	// 위상을 되돌릴 수 없고, 부모에 원격 artifact가 없는 채로 자식만 생기면
	// 정리 경로가 순환 차단된다(#129).
	if reason := remoteDeps.UmbrellaBranchGateReason(record); reason != "" {
		return deps.printErrorResult(*jsonOut, fmt.Errorf("%s", reason))
	}
	if err := validateCreateChildInputs(*title, labels, assignees); err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	providerName := firstNonEmptyMain(*providerOverride, remoteDeps.ResolveRecordProvider(record))
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		return deps.printErrorResult(*jsonOut, err)
	}
	prov, err := Resolve(providerName)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	resolved, err := resolveTemplateBody(resolveTemplateBodyRequest{
		Kind:      artifacttemplate.IssueOpsArtifactChild,
		Template:  *template,
		Provider:  providerName,
		Title:     *title,
		Body:      *body,
		BodyFile:  *bodyFile,
		Fields:    fields,
		ScoreFile: *scoreFile,
		Confirm:   *confirm,
	})
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	finalBody := resolved.Body
	if err := rejectSecretLikeRemoteCreateInputs("child create", *title, finalBody, labels, assignees); err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	var actor issueopscontract.IssueOpsActor
	if *confirm && record.Execution != nil {
		ancestry, observeErr := deps.observeNativeProcessAncestry()
		if observeErr != nil {
			return deps.printErrorResult(*jsonOut, observeErr)
		}
		actor = issueopscontract.IssueOpsActor{
			Host: *host, SessionID: *sessionID, AgentID: *agentID, CWD: *cwd, NativeProcessAncestry: ancestry,
		}
		if validateErr := remoteDeps.ValidateIssueOpsMutationActor(remoteDeps.IssueOpsStateRoot(), record.ID, actor); validateErr != nil {
			return deps.printErrorResult(*jsonOut, validateErr)
		}
	}
	result, err := remoteDeps.CreateRemoteChild(port.IssueProviderCreateChildRequest{
		Repo:           record.Repo,
		ParentIssueURL: record.IssueURL,
		Title:          *title,
		Body:           finalBody,
		Labels:         labels,
		Assignees:      assignees,
		Confirm:        *confirm,
	}, prov)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *confirm {
		if !result.HierarchyVerified || strings.TrimSpace(result.ChildURL) == "" {
			err := fmt.Errorf("provider did not verify child hierarchy")
			return deps.printErrorResult(*jsonOut, err)
		}
		if _, err := remoteDeps.LinkIssueOpsChildWithActor(remoteDeps.IssueOpsStateRoot(), record.ID, result.ChildURL, *title, actor); err != nil {
			return deps.printErrorResult(*jsonOut, err)
		}
	}
	if *jsonOut {
		return deps.printJSON(createChildResponse{IssueProviderCreateChildResult: result, Readability: resolved.Readability})
	}
	if result.ChildURL != "" {
		fmt.Printf("created child: %s\n", result.ChildURL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func runRemoteCreatePR(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote create-pr", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	title := fs.String("title", "", "PR title")
	body := fs.String("body", "", "PR body (markdown)")
	bodyFile := fs.String("body-file", "", "PR body markdown file")
	template := fs.String("template", "", "template kind")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	scoreFile := fs.String("score-file", "", "IssueOps remote score result JSON")
	head := fs.String("head", "", "source branch")
	base := fs.String("base", "", "target branch")
	expectedGeneration := fs.Uint64("expected-generation", 0, "current execution lease generation")
	host := fs.String("host", "", "native owner host")
	sessionID := fs.String("session-id", "", "native owner session id")
	agentID := fs.String("agent-id", "", "native owner agent id")
	sessionPID := fs.Int("session-pid", 0, "native owner process id")
	sessionStartedAt := fs.String("session-started-at", "", "native owner process start identity")
	sessionExecutable := fs.String("session-executable", "", "native owner executable identity")
	cwd := fs.String("cwd", "", "canonical owner worker cwd")
	confirm := fs.Bool("confirm", false, "execute creation; without this, dry-run preview only")
	var labels repeatedFlag
	var assignees repeatedFlag
	var fields repeatedFlag
	fs.Var(&labels, "label", "label to apply (repeatable)")
	fs.Var(&assignees, "assignee", "assignee username (repeatable)")
	fs.Var(&fields, "field", "template field key=value (canonical or documented alias; repeatable)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	labels, assignees = normalizeRemoteCreateMetadata(labels, assignees)
	record, err := remoteDeps.ReadIssueOps(remoteDeps.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	providerName := firstNonEmptyMain(*providerOverride, remoteDeps.ResolveRecordProvider(record))
	if providerName == "" {
		err := fmt.Errorf("cannot determine provider from IssueOps record; ensure issue_url is set")
		return deps.printErrorResult(*jsonOut, err)
	}
	headBranch := firstNonEmptyMain(*head, record.Branch)
	baseBranch := *base
	if baseBranch == "" && record.BranchPrepare != nil {
		baseBranch = record.BranchPrepare.BaseBranch
	}
	resolved, err := resolveTemplateBody(resolveTemplateBodyRequest{
		Kind:      artifacttemplate.IssueOpsArtifactPR,
		Template:  *template,
		Provider:  providerName,
		Title:     *title,
		Body:      *body,
		BodyFile:  *bodyFile,
		Fields:    fields,
		ScoreFile: *scoreFile,
		Confirm:   *confirm,
	})
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	finalBody := resolved.Body
	if err := rejectSecretLikeRemoteCreateInputs("pr create", *title, finalBody, labels, assignees); err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if err := validateConfirmRemoteCreate(*confirm, labels, assignees); err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	actor, err := deps.remoteNativeActor(*host, *sessionID, *agentID, *sessionPID, *sessionStartedAt, *sessionExecutable, *confirm)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	result, err := remoteDeps.CreateRemotePullRequestWithHandler(context.Background(), remoteDeps.IssueOpsStateRoot(), issueopscontract.RemotePullRequestRequest{
		ID: record.ID, Provider: providerName, Title: *title, Body: finalBody,
		Head: headBranch, Base: baseBranch, Labels: labels, Assignees: assignees,
		ExpectedGeneration: *expectedGeneration,
		Actor:              actor,
		CWD:                *cwd, Confirm: *confirm,
	}, deps.Publication.Create)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(createPRResponse{IssueProviderCreatePullRequestResult: result, Readability: resolved.Readability})
	}
	if result.URL != "" {
		fmt.Printf("created: %s\n", result.URL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func (deps Deps) remoteNativeActor(host, sessionID, agentID string, sessionPID int, sessionStartedAt, sessionExecutable string, observe bool) (issueopscontract.NativeActor, error) {
	actor := issueopscontract.NativeActor{
		Host: host, SessionID: sessionID, AgentID: agentID,
		SessionProcess: &issueopscontract.NativeProcessReceipt{PID: sessionPID, StartedAt: sessionStartedAt, Executable: sessionExecutable},
	}
	if !observe {
		return actor, nil
	}
	ancestry, err := deps.observeNativeProcessAncestry()
	if err != nil {
		return issueopscontract.NativeActor{}, err
	}
	actor.ProcessAncestry = ancestry
	return actor, nil
}

func (deps Deps) observeNativeProcessAncestry() ([]issueopscontract.NativeProcessReceipt, error) {
	observe := deps.ObserveProcessAncestry
	if observe == nil {
		observe = remoteDeps.ObserveNativeProcessAncestry
	}
	ancestry, err := observe(os.Getpid())
	if err != nil {
		return nil, fmt.Errorf("observe native process ancestry: %w", err)
	}
	if len(ancestry) == 0 {
		return nil, fmt.Errorf("observe native process ancestry: no process receipts returned")
	}
	return ancestry, nil
}

func validateCreateChildInputs(title string, labels, assignees []string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("child title is required")
	}
	if len(labels) == 0 {
		return fmt.Errorf("at least one child label is required")
	}
	if len(assignees) == 0 {
		return fmt.Errorf("at least one child assignee is required")
	}
	return nil
}

type resolveTemplateBodyRequest struct {
	Kind      artifacttemplate.IssueOpsArtifactKind
	Template  string
	Provider  string
	Title     string
	Body      string
	BodyFile  string
	Fields    []string
	ScoreFile string
	Confirm   bool
}

// resolvedTemplateBody is the body a create command publishes and the
// readability report that judged it.
type resolvedTemplateBody struct {
	Body        string
	Readability artifactreadability.Report
}

// resolveTemplateBody renders the body against its contract and always runs
// the readability check. A preview reports the findings; a confirm refuses
// any critical finding before anything is sealed or sent to the provider.
// create-issue has four templates, so its confirm must name one; create-child
// and create-pr fall back to their single template.
func resolveTemplateBody(req resolveTemplateBodyRequest) (resolvedTemplateBody, error) {
	body, err := readBodyInput(req.Body, req.BodyFile)
	if err != nil {
		return resolvedTemplateBody{}, err
	}
	template := strings.TrimSpace(req.Template)
	if template == "" && req.Confirm && req.Kind == artifacttemplate.IssueOpsArtifactIssue {
		return resolvedTemplateBody{}, fmt.Errorf("--template is required with --confirm: pass bug, feature, proposal, or implementation_task")
	}
	fields, err := artifacttemplate.ParseFieldAssignments(req.Fields)
	if err != nil {
		return resolvedTemplateBody{}, err
	}
	scoreSummary, err := readScoreSummaryFile(req.ScoreFile)
	if err != nil {
		return resolvedTemplateBody{}, err
	}
	input := artifacttemplate.IssueOpsTemplateInput{
		Kind:         req.Kind,
		Template:     artifacttemplate.IssueOpsTemplateKind(template),
		Provider:     req.Provider,
		Title:        req.Title,
		Body:         body,
		Fields:       fields,
		ScoreSummary: scoreSummary,
	}
	result := artifacttemplate.Render(input)
	if body == "" && len(fields) == 0 {
		// Nothing was authored: judge the empty body instead of publishing an
		// empty skeleton. The skeleton belongs to render-template.
		result.Body = ""
	}
	var inputErrors []string
	for _, code := range result.Validation.Critical {
		if !readabilityDeferredCodes[code] {
			inputErrors = append(inputErrors, code)
		}
	}
	if len(inputErrors) > 0 {
		return resolvedTemplateBody{}, fmt.Errorf("template validation failed: %s", strings.Join(inputErrors, ","))
	}
	resolved := resolvedTemplateBody{
		Body: result.Body,
		Readability: artifactreadability.Check(artifactreadability.Input{
			Kind:     artifactreadability.KindFor(result.Kind),
			Template: result.Template,
			Title:    req.Title,
			Body:     result.Body,
			Fields:   fields,
		}),
	}
	if req.Confirm && !resolved.Readability.OK {
		return resolved, artifactreadability.RefusalError(resolved.Readability)
	}
	return resolved, nil
}

// readBodyInput reads the body from --body or --body-file.
func readBodyInput(body, bodyFile string) (string, error) {
	body = strings.TrimSpace(body)
	bodyFile = strings.TrimSpace(bodyFile)
	if body != "" && bodyFile != "" {
		return "", fmt.Errorf("body and body-file are mutually exclusive")
	}
	if bodyFile == "" {
		return body, nil
	}
	b, err := os.ReadFile(bodyFile)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func validateConfirmRemoteCreate(confirm bool, labels, assignees []string) error {
	if !confirm {
		return nil
	}
	labels = issueopsremote.CleanValues(labels)
	assignees = issueopsremote.CleanValues(assignees)
	if len(labels) == 0 {
		return fmt.Errorf("at least one label is required with --confirm")
	}
	if len(assignees) == 0 {
		return fmt.Errorf("at least one assignee is required with --confirm")
	}
	return nil
}

func readScoreSummaryFile(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var result issueopsremote.IssueOpsRemoteScoringResult
	if err := json.Unmarshal(b, &result); err != nil {
		return strings.TrimSpace(string(b)), nil
	}
	parts := []string{fmt.Sprintf("threshold %.2f", result.Threshold)}
	if len(result.SelectedRelatedIssues) > 0 {
		parts = append(parts, "선택 관련 이슈: "+joinScoredItems(result.SelectedRelatedIssues))
	}
	if len(result.RejectedRelatedIssues) > 0 {
		parts = append(parts, "거절 관련 이슈: "+joinScoredItems(result.RejectedRelatedIssues))
	}
	if len(result.SelectedLabels) > 0 {
		parts = append(parts, "선택 라벨: "+joinScoredItems(result.SelectedLabels))
	}
	if len(result.RejectedLabels) > 0 {
		parts = append(parts, "거절 라벨: "+joinScoredItems(result.RejectedLabels))
	}
	return strings.Join(parts, "\n"), nil
}

func joinScoredItems(items []issueopsremote.IssueOpsRemoteScoredItem) string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		name := firstNonEmptyMain(item.Name, item.ID, item.Title, item.URL)
		if name == "" {
			name = "unknown"
		}
		out = append(out, fmt.Sprintf("%s(%.2f)", name, item.Score))
	}
	return strings.Join(out, ", ")
}

func runRemoteSyncGraph(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote sync-graph", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	confirm := fs.Bool("confirm", false, "execute sync; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	record, err := remoteDeps.ReadIssueOps(remoteDeps.IssueOpsStateRoot(), *id)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if !*confirm {
		links := len(record.IssueLinks)
		if *jsonOut {
			return deps.printJSON(map[string]any{
				"ok":         true,
				"synced":     false,
				"dry_run":    true,
				"link_count": links,
				"message":    fmt.Sprintf("[dry-run] would sync %d issue graph links to remote issue %s", links, record.IssueURL),
			})
		}
		if links == 0 {
			fmt.Println("no issue graph links to sync")
			return nil
		}
		fmt.Printf("[dry-run] would sync %d issue graph links to remote issue %s\n", links, record.IssueURL)
		return nil
	}
	result, err := remoteDeps.SyncRemoteIssueGraph(record)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	fmt.Printf("synced: %v links\n", result["link_count"])
	return nil
}
