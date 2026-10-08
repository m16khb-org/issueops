package issueopsapp

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"

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
	stdout  io.Writer
}

func runMCPAuthorize(args []string) error {
	return mcpAuthorizeCommand{
		issuer: newAuthorityService(), observe: issueopscore.ObserveNativeProcessAncestry,
		pid: os.Getpid, stdout: os.Stdout,
	}.Run(args)
}

// Run issues a pre-lease caller capability from CLI-observed native ancestry.
// Only the managed credential path and expiry are printed, never the token.
func (command mcpAuthorizeCommand) Run(args []string) error {
	fs := flag.NewFlagSet("issueops mcp authorize", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	workspaceRoot := fs.String("workspace-root", "", "absolute repository or workspace root to authorize")
	host := fs.String("host", "", "native host: codex, claude, omo, or omp")
	sessionID := fs.String("session-id", "", "native session id")
	agentID := fs.String("agent-id", "", "optional native agent id")
	sessionPID := fs.Int("session-pid", 0, "native session process id")
	startedAt := fs.String("session-started-at", "", "native session process start identity")
	executable := fs.String("session-executable", "", "native session executable identity")
	cwd := fs.String("cwd", "", "optional caller cwd; must be inside --workspace-root")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected mcp authorize argument %q", fs.Arg(0))
	}
	for name, value := range map[string]string{"workspace-root": *workspaceRoot, "host": *host, "session-id": *sessionID, "session-started-at": *startedAt, "session-executable": *executable} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("mcp authorize requires --%s", name)
		}
	}
	if *sessionPID <= 0 {
		return fmt.Errorf("mcp authorize requires --session-pid")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if strings.TrimSpace(*cwd) != "" {
		if _, err := (authorityoutbound.ScopeResolver{}).Resolve(ctx, strings.TrimSpace(*workspaceRoot), strings.TrimSpace(*cwd)); err != nil {
			return fmt.Errorf("mcp authorize: %w", err)
		}
	}
	ancestry, err := command.observe(command.pid())
	if err != nil {
		return fmt.Errorf("observe native process ancestry: %w", err)
	}
	receipt, err := command.issuer.Issue(ctx, authoritycontract.IssueRequest{
		WorkspaceRoot: strings.TrimSpace(*workspaceRoot),
		Actor: model.NativeActor{
			Host: *host, SessionID: *sessionID, AgentID: *agentID,
			SessionProcess:  &model.NativeProcessReceipt{PID: *sessionPID, StartedAt: *startedAt, Executable: *executable},
			ProcessAncestry: ancestry,
		},
	})
	if *jsonOut {
		encoder := json.NewEncoder(command.stdout)
		encoder.SetIndent("", "  ")
		if printErr := encoder.Encode(receipt); printErr != nil {
			return printErr
		}
		return err
	}
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(command.stdout, "authority_file: %s\nexpires_at: %s\n", receipt.AuthorityFile, receipt.ExpiresAt)
	return err
}
