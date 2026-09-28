package remotecmd

import (
	"context"
	"flag"
	"fmt"

	remoteapp "issueops/internal/application/issueopsremote"
)

func runRemoteCreateIssue(ctx context.Context, args []string, deps Deps) error {
	fs := flag.NewFlagSet("issueops remote create-issue", flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	title := fs.String("title", "", "issue title")
	body := fs.String("body", "", "issue body (markdown)")
	bodyFile := fs.String("body-file", "", "issue body markdown file")
	template := fs.String("template", "", "template kind")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	scoreFile := fs.String("score-file", "", "IssueOps remote score result JSON")
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
	result, err := remoteDeps.CreateIssue(ctx, remoteDeps.IssueOpsStateRoot(), remoteapp.IssueCreateCommand{ID: *id, Provider: *providerOverride, Title: *title, Body: *body, BodyFile: *bodyFile, Template: *template, ScoreFile: *scoreFile, Fields: fields, Labels: labels, Assignees: assignees, Confirm: *confirm}, deps.verifyLive)
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
