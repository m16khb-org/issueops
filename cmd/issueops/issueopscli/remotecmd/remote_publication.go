package remotecmd

import (
	"context"
	"flag"
	"fmt"

	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
)

func (command Command) runRemotePublication(args []string, deps Deps) error {
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
	result, err := command.Operations.CreatePublication(context.Background(), command.Operations.IssueOpsStateRoot(), remoteapp.PublicationInput{
		Request: issueopscontract.RemotePullRequestRequest{ID: *id, Provider: *providerOverride, Title: *title, Body: *body, Head: *head, Base: *base, Labels: labels, Assignees: assignees, ExpectedGeneration: *expectedGeneration, CWD: *cwd, Confirm: *confirm,
			Actor: issueopscontract.NativeActor{Host: *host, SessionID: *sessionID, AgentID: *agentID, SessionProcess: &issueopscontract.NativeProcessReceipt{PID: *sessionPID, StartedAt: *sessionStartedAt, Executable: *sessionExecutable}}},
		BodyFile: *bodyFile, Template: *template, ScoreFile: *scoreFile, Fields: fields,
	}, deps.Publication.Create, deps.observeNativeProcessAncestry)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	if result.URL != "" {
		fmt.Printf("created: %s\n", result.URL)
	} else {
		fmt.Println(result.Preview)
	}
	return nil
}
