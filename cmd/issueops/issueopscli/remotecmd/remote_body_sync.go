package remotecmd

import (
	"context"
	"flag"
	"fmt"
	"strings"

	remoteapp "issueops/internal/application/issueopsremote"
	issueopscontract "issueops/internal/contract/issueops"
	bodysynccontract "issueops/internal/contract/issueopsbodysync"
)

// runRemoteSyncIssue refreshes a linked issue's body, or the body of one of its
// provider-native children when --url names one.
func runRemoteSyncIssue(ctx context.Context, args []string, deps Deps) error {
	return runBodySyncCommand(ctx, "issueops remote sync-issue", bodysynccontract.KindIssue, args, deps)
}

// runRemoteSyncPR refreshes the body of the PR/MR this cycle published. It is
// fenced by the execution lease generation, like create-pr.
func runRemoteSyncPR(ctx context.Context, args []string, deps Deps) error {
	return runBodySyncCommand(ctx, "issueops remote sync-pr", bodysynccontract.KindPR, args, deps)
}

func runBodySyncCommand(ctx context.Context, name, kind string, args []string, deps Deps) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	id := fs.String("id", "", "IssueOps id")
	providerOverride := fs.String("provider", "", "remote provider override: github or gitlab")
	artifactURL := fs.String("url", "", "artifact URL; for sync-issue a provider-native child of the linked issue")
	body := fs.String("body", "", "replacement body markdown")
	bodyFile := fs.String("body-file", "", "replacement body markdown file")
	expectedBodySHA := fs.String("expected-body-sha256", "", "digest of the live body the replacement was built on; required with --confirm")
	acceptRemoteEdits := fs.Bool("accept-remote-edits", false, "acknowledge that the body was edited outside the harness and the replacement preserves those edits")
	host := fs.String("host", "", "native holder host")
	sessionID := fs.String("session-id", "", "native holder session id")
	agentID := fs.String("agent-id", "", "optional native holder agent id")
	cwd := fs.String("cwd", "", "canonical holder worktree cwd")
	confirm := fs.Bool("confirm", false, "write to the remote artifact; without this, dry-run preview only")
	jsonOut := fs.Bool("json", false, "print JSON")
	var expectedGeneration *uint64
	if kind == bodysynccontract.KindPR {
		expectedGeneration = fs.Uint64("expected-generation", 0, "expected execution lease generation")
	}
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	cmd := bodysynccontract.Command{
		ID:                 *id,
		Kind:               kind,
		URL:                *artifactURL,
		ProposedBody:       *body,
		ExpectedBodySHA256: *expectedBodySHA,
		AcceptRemoteEdits:  *acceptRemoteEdits,
		Confirm:            *confirm,
	}
	if expectedGeneration != nil {
		cmd.ExpectedGeneration = *expectedGeneration
	}
	_, result, err := remoteDeps.SyncRemoteBody(ctx, remoteDeps.IssueOpsStateRoot(), remoteapp.BodySyncInput{
		Provider: *providerOverride, BodyFile: *bodyFile, Command: cmd,
		Actor: issueopscontract.IssueOpsActor{Host: *host, SessionID: *sessionID, AgentID: *agentID, CWD: *cwd},
	}, deps.observeNativeProcessAncestry)
	if err != nil {
		return deps.printErrorResult(*jsonOut, err)
	}
	if *jsonOut {
		return deps.printJSON(result)
	}
	printBodySyncResult(result)
	return nil
}

func printBodySyncResult(result bodysynccontract.Result) {
	fmt.Printf("kind=%s url=%s drift=%s\n", result.Kind, result.URL, result.Drift)
	fmt.Printf("remote=%s merged=%s\n", result.RemoteBodySHA256, result.MergedBodySHA256)
	if len(result.PreservedSections) > 0 {
		fmt.Printf("preserved: %s\n", strings.Join(result.PreservedSections, ", "))
	}
	switch {
	case result.Updated:
		fmt.Println("body updated")
	case result.Drift == bodysynccontract.DriftInSync:
		fmt.Println("already in sync; nothing to write")
	default:
		fmt.Printf("preview only; re-run with --confirm --expected-body-sha256 %s\n", result.ExpectedBodySHA256)
	}
}
