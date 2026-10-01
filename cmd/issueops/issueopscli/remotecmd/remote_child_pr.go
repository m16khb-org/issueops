package remotecmd

import (
	"context"
	"flag"
	"fmt"
	"os"

	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
)

func (command Command) runRemoteCreateChild(args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote create-child", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	operationID := fs.String("operation-id", "", "32 lowercase hex ID; generate once with Python secrets.token_hex(16), save it, and reuse it for retries; a fresh ID explicitly creates another child")
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
	result, err := command.Operations.CreateChild(context.Background(), command.Operations.IssueOpsStateRoot(), remoteapp.ChildCreateCommand{
		ID: *id, OperationID: *operationID, Provider: *providerOverride, Title: *title, Body: *body, BodyFile: *bodyFile, Template: *template, ScoreFile: *scoreFile, Fields: fields, Labels: labels, Assignees: assignees, Confirm: *confirm,
		Actor: issueopscontract.IssueOpsActor{Host: *host, SessionID: *sessionID, AgentID: *agentID, CWD: *cwd},
	}, deps.observeNativeProcessAncestry)
	if err != nil {
		return deps.printChildErrorResult(*jsonOut, result, err)
	}

	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.ChildURL != "" {
		fmt.Printf("created child: %s\n", result.ChildURL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}

func (deps Deps) observeNativeProcessAncestry() ([]issueopscontract.NativeProcessReceipt, error) {
	observe := deps.ObserveProcessAncestry
	ancestry, err := observe(os.Getpid())
	if err != nil {
		return nil, fmt.Errorf("observe native process ancestry: %w", err)
	}
	if len(ancestry) == 0 {
		return nil, fmt.Errorf("observe native process ancestry: no process receipts returned")
	}
	return ancestry, nil
}

func (deps Deps) printChildErrorResult(jsonOut bool, result remoteapp.ChildCreateResult, err error) error {
	if result.OperationID == "" && result.ChildURL == "" {
		return deps.printErrorResult(jsonOut, err)
	}
	result.OK = false
	if jsonOut {
		if printErr := deps.printJSON(struct {
			remoteapp.ChildCreateResult
			Error string `json:"error"`
		}{result, err.Error()}); printErr != nil {
			return printErr
		}
	}
	return err
}
