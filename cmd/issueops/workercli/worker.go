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
	workercontract "issueops/internal/contract/worker"
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
		return cliflag.UnknownSubcommand("worker", args[0], "enqueue", "run", "status", "list", "cleanup-stuck", "cancel")
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

// readOnlyOptions holds the parsed `worker run` flags and trailing argv.
type readOnlyOptions struct {
	kind          string
	payload       string
	workspaceRoot string
	cwd           string
	timeout       time.Duration
	envAllowlist  string
	jsonOut       bool
	argv          []string
}

func parseReadOnlyOptions(args []string) (readOnlyOptions, error) {
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
		return readOnlyOptions{}, err
	}
	if !*readOnly {
		return readOnlyOptions{}, fmt.Errorf("worker run currently requires --read-only")
	}
	if fs.NArg() == 0 {
		// Probe: an empty argv still recorded a failed job (argv_required).
		return readOnlyOptions{}, fmt.Errorf("worker run --read-only needs the command after the flags, e.g. issueops worker run --read-only --json -- git status --short")
	}
	return readOnlyOptions{
		kind: *kind, payload: *payload, workspaceRoot: *workspaceRoot, cwd: *cwd,
		timeout: *timeout, envAllowlist: *envAllowlist, jsonOut: *jsonOut, argv: fs.Args(),
	}, nil
}

func (command Command) readOnlyRequest(opts readOnlyOptions) policy.CommandPolicyRequest {
	root := opts.workspaceRoot
	if root == "" {
		root = command.ResolveTarget("")
	}
	workDir := opts.cwd
	if workDir == "" {
		workDir = root
	}
	return policy.CommandPolicyRequest{
		WorkspaceRoot: root,
		CWD:           workDir,
		Argv:          opts.argv,
		Timeout:       opts.timeout.String(),
		EnvAllowlist:  splitCSV(opts.envAllowlist),
	}
}

// printReadOnlyJob renders a successful job: the JSON document, or the
// summary line plus the captured stdout/stderr.
func printReadOnlyJob(job workercontract.Job, jsonOut bool) error {
	if jsonOut {
		return printJSON(job)
	}
	fmt.Printf("%s %s exit=%d\n", job.ID, job.Status, job.Result.ExitCode)
	if job.Result.Stdout != "" {
		fmt.Print(job.Result.Stdout)
	}
	if job.Result.Stderr != "" {
		fmt.Fprint(os.Stderr, job.Result.Stderr)
	}
	return nil
}

// readOnlyOutcome maps a printed job to the command's exit error. With
// --json the printed job already carries the refusal, so the error is only
// marked reported.
func readOnlyOutcome(job workercontract.Job, jsonOut bool) error {
	if job.Result == nil {
		return nil
	}
	var failure error
	switch {
	case !job.Result.Policy.Allowed:
		failure = policy.DeniedError{Reasons: job.Result.Policy.DenyReasons}
	case job.Result.ExitCode != 0:
		failure = fmt.Errorf("worker read-only command exited %d", job.Result.ExitCode)
	default:
		return nil
	}
	if jsonOut {
		return jsonout.Reported(failure)
	}
	return failure
}

func (command Command) RunReadOnly(args []string) error {
	opts, err := parseReadOnlyOptions(args)
	if err != nil {
		return err
	}
	job, err := command.Service.RunReadOnly(context.Background(), opts.kind, opts.payload, command.readOnlyRequest(opts))
	if err != nil {
		if opts.jsonOut {
			return jsonout.PrintResultError(os.Stdout, job, err)
		}
		return err
	}
	if printErr := printReadOnlyJob(job, opts.jsonOut); printErr != nil {
		return printErr
	}
	return readOnlyOutcome(job, opts.jsonOut)
}
