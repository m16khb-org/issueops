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

// mcpAuthorizeOptions holds the parsed `mcp authorize` flags.
type mcpAuthorizeOptions struct {
	workspaceRoot string
	host          string
	sessionID     string
	agentID       string
	sessionPID    int
	startedAt     string
	executable    string
	cwd           string
	jsonOut       bool
	args          []string
}

func parseMCPAuthorizeOptions(args []string) (mcpAuthorizeOptions, error) {
	fs := flag.NewFlagSet("issueops mcp authorize", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var opts mcpAuthorizeOptions
	fs.StringVar(&opts.workspaceRoot, "workspace-root", "", "absolute repository or workspace root to authorize")
	fs.StringVar(&opts.host, "host", "", "native host: codex, claude, omo, omp, or opencode")
	fs.StringVar(&opts.sessionID, "session-id", "", "native session id")
	fs.StringVar(&opts.agentID, "agent-id", "", "optional native agent id")
	fs.IntVar(&opts.sessionPID, "session-pid", 0, "native session process id")
	fs.StringVar(&opts.startedAt, "session-started-at", "", "native session process start identity")
	fs.StringVar(&opts.executable, "session-executable", "", "native session executable identity")
	fs.StringVar(&opts.cwd, "cwd", "", "optional caller cwd; must be inside --workspace-root")
	fs.BoolVar(&opts.jsonOut, "json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return mcpAuthorizeOptions{}, err
	}
	opts.args = fs.Args()
	return opts, nil
}

// fail reports a refusal. With --json every refusal is one
// {"ok":false,"error":...} document.
func (command mcpAuthorizeCommand) fail(jsonOut bool, err error) error {
	if !jsonOut {
		return err
	}
	if printErr := jsonout.PrintTo(command.stdout, map[string]any{"ok": false, "error": err.Error()}); printErr != nil {
		return printErr
	}
	return jsonout.Reported(err)
}

// sessionFlagFields lists the string session flags that must come together
// with --session-pid.
func (opts mcpAuthorizeOptions) sessionFlagFields() []struct{ name, value string } {
	return []struct{ name, value string }{
		{"host", opts.host}, {"session-id", opts.sessionID}, {"session-started-at", opts.startedAt}, {"session-executable", opts.executable},
	}
}

// missingSessionFlags names the session flags left unset, in flag order.
func (opts mcpAuthorizeOptions) missingSessionFlags() []string {
	var missing []string
	for _, field := range opts.sessionFlagFields() {
		if strings.TrimSpace(field.value) == "" {
			missing = append(missing, "--"+field.name)
		}
	}
	if opts.sessionPID <= 0 {
		missing = append(missing, "--session-pid")
	}
	return missing
}

// resolveSession fills the session flags. The five session flags come
// together or not at all. Without them the command uses the session
// `execution whoami` reports; the issuer still checks that receipt against
// the live ancestry.
func (command mcpAuthorizeCommand) resolveSession(opts *mcpAuthorizeOptions, ancestry []model.NativeProcessReceipt) error {
	missing := opts.missingSessionFlags()
	switch len(missing) {
	case 0:
		return nil
	case len(opts.sessionFlagFields()) + 1:
		getenv := command.getenv
		if getenv == nil {
			getenv = os.Getenv
		}
		host, sessionID, current, err := executioncmd.CurrentNativeSessionReceipt(getenv, command.pid(), ancestry)
		if err != nil {
			return fmt.Errorf("mcp authorize: resolve the current native session (or pass --host --session-id --session-pid --session-started-at --session-executable): %w", err)
		}
		opts.host, opts.sessionID = host, sessionID
		opts.sessionPID, opts.startedAt, opts.executable = current.PID, current.StartedAt, current.Executable
		return nil
	default:
		return fmt.Errorf("mcp authorize: the session flags --host --session-id --session-pid --session-started-at --session-executable go together; missing %s (or omit all five to use the current session)", strings.Join(missing, " "))
	}
}

// Run issues a pre-lease caller capability from CLI-observed native ancestry.
// Only the managed credential path and expiry are printed, never the token.
func (command mcpAuthorizeCommand) Run(args []string) error {
	opts, err := parseMCPAuthorizeOptions(args)
	if err != nil {
		return err
	}
	if len(opts.args) > 0 {
		return command.fail(opts.jsonOut, fmt.Errorf("unexpected mcp authorize argument %q", opts.args[0]))
	}
	if strings.TrimSpace(opts.workspaceRoot) == "" {
		return command.fail(opts.jsonOut, fmt.Errorf("mcp authorize requires --workspace-root"))
	}
	ancestry, err := command.observe(command.pid())
	if err != nil {
		return command.fail(opts.jsonOut, fmt.Errorf("observe native process ancestry: %w", err))
	}
	if err := command.resolveSession(&opts, ancestry); err != nil {
		return command.fail(opts.jsonOut, err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	workspaceRoot := strings.TrimSpace(opts.workspaceRoot)
	if cwd := strings.TrimSpace(opts.cwd); cwd != "" {
		if _, err := (authorityoutbound.ScopeResolver{}).Resolve(ctx, workspaceRoot, cwd); err != nil {
			return command.fail(opts.jsonOut, fmt.Errorf("mcp authorize: %w", err))
		}
	}
	receipt, err := command.issuer.Issue(ctx, authoritycontract.IssueRequest{
		WorkspaceRoot: workspaceRoot,
		Actor: model.NativeActor{
			Host: opts.host, SessionID: opts.sessionID, AgentID: opts.agentID,
			SessionProcess:  &model.NativeProcessReceipt{PID: opts.sessionPID, StartedAt: opts.startedAt, Executable: opts.executable},
			ProcessAncestry: ancestry,
		},
	})
	if err != nil {
		// The receipt has no error field; the envelope names the refusal.
		return command.fail(opts.jsonOut, err)
	}
	if opts.jsonOut {
		return jsonout.PrintTo(command.stdout, receipt)
	}
	_, err = fmt.Fprintf(command.stdout, "authority_file: %s\nexpires_at: %s\n", receipt.AuthorityFile, receipt.ExpiresAt)
	return err
}
