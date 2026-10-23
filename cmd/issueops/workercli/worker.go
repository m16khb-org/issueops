package workercli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"issueops/cmd/issueops/cliflag"
	"issueops/cmd/issueops/jsonout"
	policy "issueops/internal/contract/policy"
)

func (command Command) Run(args []string) error {
	if len(args) == 0 {
		workerUsage()
		return fmt.Errorf("missing worker subcommand")
	}
	switch args[0] {
	case "enqueue":
		return command.RunEnqueue(args[1:])
	case "run":
		return command.RunReadOnly(args[1:])
	case "status":
		return command.RunStatus(args[1:])
	case "list":
		return command.RunList(args[1:])
	case "cleanup-stuck":
		return command.RunCleanupStuck(args[1:])
	case "cancel":
		return command.RunCancel(args[1:])
	case "-h", "--help", "help":
		workerUsage()
		return flag.ErrHelp
	default:
		workerUsage()
		return fmt.Errorf("unknown worker subcommand %q", args[0])
	}
}

func workerUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops worker enqueue --kind KIND [--payload TEXT] [--json]
  issueops worker run --read-only --kind KIND [--payload TEXT] [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--json] -- ARGV...
  issueops worker status --id ID [--json]
  issueops worker list [--json]
  issueops worker cleanup-stuck [--json]
  issueops worker cancel --id ID [--json]
`)
}

func (command Command) RunReadOnly(args []string) error {
	fs := flag.NewFlagSet("worker run", flag.ContinueOnError)
	kind := fs.String("kind", "read-only-command", "job kind")
	payload := fs.String("payload", "", "redacted job payload")
	readOnly := fs.Bool("read-only", false, "execute an argv-only read-only command")
	workspaceRoot := fs.String("workspace-root", "", "workspace root boundary")
	cwd := fs.String("cwd", "", "command working directory")
	timeout := fs.Duration("timeout", 30*time.Second, "maximum runtime")
	envAllowlist := fs.String("env", "", "comma-separated environment variable allowlist")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}
	if !*readOnly {
		return fmt.Errorf("worker run currently requires --read-only")
	}
	root := *workspaceRoot
	if root == "" {
		root = command.ResolveTarget("")
	}
	workDir := *cwd
	if workDir == "" {
		workDir = root
	}
	req := policy.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           workDir,
		Argv:          fs.Args(),
		Timeout:       timeout.String(),
		EnvAllowlist:  splitCSV(*envAllowlist),
	}
	job, err := command.Service.RunReadOnly(context.Background(), *kind, *payload, req)
	if *jsonOut {
		if err != nil {
			return jsonout.PrintResultError(os.Stdout, job, err)
		}
		if printErr := printJSON(job); printErr != nil {
			return printErr
		}
	}
	if err == nil && !*jsonOut {
		fmt.Printf("%s %s exit=%d\n", job.ID, job.Status, job.Result.ExitCode)
		if job.Result.Stdout != "" {
			fmt.Print(job.Result.Stdout)
		}
		if job.Result.Stderr != "" {
			fmt.Fprint(os.Stderr, job.Result.Stderr)
		}
	}
	if err != nil {
		return err
	}
	if job.Result != nil && !job.Result.Policy.Allowed {
		denied := error(policy.PolicyDeniedError{Reasons: job.Result.Policy.DenyReasons})
		if *jsonOut {
			return jsonout.Reported(denied)
		}
		return denied
	}
	if job.Result != nil && job.Result.ExitCode != 0 {
		exited := fmt.Errorf("worker read-only command exited %d", job.Result.ExitCode)
		if *jsonOut {
			// The printed job carries the exit code.
			return jsonout.Reported(exited)
		}
		return exited
	}
	return nil
}
