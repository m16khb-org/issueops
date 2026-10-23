package issueopsapp

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

	"issueops/cmd/issueops/cliflag"
	"issueops/cmd/issueops/issueopscli/executioncmd"
	"issueops/cmd/issueops/jsonout"
	issueopscore "issueops/internal/adapter/issueops"
	authorityoutbound "issueops/internal/adapter/outbound/authority"
	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
)

type authorityIssuer interface {
	Issue(context.Context, authoritycontract.IssueRequest) (authoritycontract.Receipt, error)
}

type mcpAuthorizeCommand struct {
	issuer  authorityIssuer
	observe func(int) ([]model.NativeProcessReceipt, error)
	pid     func() int
	getenv  func(string) string
	stdout  io.Writer
}

func runMCPAuthorize(args []string) error {
	return mcpAuthorizeCommand{
		issuer: newAuthorityService(), observe: issueopscore.ObserveNativeProcessAncestry,
		pid: os.Getpid, getenv: os.Getenv, stdout: os.Stdout,
	}.Run(args)
}

// Run issues a pre-lease caller capability from CLI-observed native ancestry.
// Only the managed credential path and expiry are printed, never the token.
func (command mcpAuthorizeCommand) Run(args []string) error {
	fs := flag.NewFlagSet("issueops mcp authorize", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	workspaceRoot := fs.String("workspace-root", "", "absolute repository or workspace root to authorize")
	host := fs.String("host", "", "native host: codex, claude, omo, omp, or opencode")
	sessionID := fs.String("session-id", "", "native session id")
	agentID := fs.String("agent-id", "", "optional native agent id")
	sessionPID := fs.Int("session-pid", 0, "native session process id")
	startedAt := fs.String("session-started-at", "", "native session process start identity")
	executable := fs.String("session-executable", "", "native session executable identity")
	cwd := fs.String("cwd", "", "optional caller cwd; must be inside --workspace-root")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}
	// With --json every refusal is one {"ok":false,"error":...} document.
	fail := func(err error) error {
		if !*jsonOut {
			return err
		}
		if printErr := jsonout.PrintTo(command.stdout, map[string]any{"ok": false, "error": err.Error()}); printErr != nil {
			return printErr
		}
		return jsonout.Reported(err)
	}
	if fs.NArg() > 0 {
		return fail(fmt.Errorf("unexpected mcp authorize argument %q", fs.Arg(0)))
	}
	if strings.TrimSpace(*workspaceRoot) == "" {
		return fail(fmt.Errorf("mcp authorize requires --workspace-root"))
	}
	ancestry, err := command.observe(command.pid())
	if err != nil {
		return fail(fmt.Errorf("observe native process ancestry: %w", err))
	}
	// The five session flags come together or not at all. Without them the
	// command uses the session `execution whoami` reports; the issuer still
	// checks that receipt against the live ancestry.
	sessionFlags := []struct{ name, value string }{
		{"host", *host}, {"session-id", *sessionID}, {"session-started-at", *startedAt}, {"session-executable", *executable},
	}
	given := 0
	for _, field := range sessionFlags {
		if strings.TrimSpace(field.value) != "" {
			given++
		}
	}
	if *sessionPID > 0 {
		given++
	}
	switch given {
	case 0:
		getenv := command.getenv
		if getenv == nil {
			getenv = os.Getenv
		}
		var current model.NativeProcessReceipt
		*host, *sessionID, current, err = executioncmd.CurrentNativeSessionReceipt(getenv, command.pid(), ancestry)
		if err != nil {
			return fail(fmt.Errorf("mcp authorize: resolve the current native session (or pass --host --session-id --session-pid --session-started-at --session-executable): %w", err))
		}
		*sessionPID, *startedAt, *executable = current.PID, current.StartedAt, current.Executable
	case len(sessionFlags) + 1:
	default:
		var missing []string
		for _, field := range sessionFlags {
			if strings.TrimSpace(field.value) == "" {
				missing = append(missing, "--"+field.name)
			}
		}
		if *sessionPID <= 0 {
			missing = append(missing, "--session-pid")
		}
		return fail(fmt.Errorf("mcp authorize: the session flags --host --session-id --session-pid --session-started-at --session-executable go together; missing %s (or omit all five to use the current session)", strings.Join(missing, " ")))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if strings.TrimSpace(*cwd) != "" {
		if _, err := (authorityoutbound.ScopeResolver{}).Resolve(ctx, strings.TrimSpace(*workspaceRoot), strings.TrimSpace(*cwd)); err != nil {
			return fail(fmt.Errorf("mcp authorize: %w", err))
		}
	}
	receipt, err := command.issuer.Issue(ctx, authoritycontract.IssueRequest{
		WorkspaceRoot: strings.TrimSpace(*workspaceRoot),
		Actor: model.NativeActor{
			Host: *host, SessionID: *sessionID, AgentID: *agentID,
			SessionProcess:  &model.NativeProcessReceipt{PID: *sessionPID, StartedAt: *startedAt, Executable: *executable},
			ProcessAncestry: ancestry,
		},
	})
	if err != nil {
		// The receipt has no error field; the envelope names the refusal.
		return fail(err)
	}
	if *jsonOut {
		return jsonout.PrintTo(command.stdout, receipt)
	}
	_, err = fmt.Fprintf(command.stdout, "authority_file: %s\nexpires_at: %s\n", receipt.AuthorityFile, receipt.ExpiresAt)
	return err
}
