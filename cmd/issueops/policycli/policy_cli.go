package policycli

import (
	"fmt"
	policydomain "issueops/internal/contract/policy"
	"os"
)

func (command Command) Run(args []string) error {
	if len(args) == 0 {
		policyUsage()
		return fmt.Errorf("missing policy subcommand")
	}
	switch args[0] {
	case "check":
		return command.Check(args[1:])
	case "fake-run":
		return command.FakeRun(args[1:])
	case "run":
		return command.RunReadOnly(args[1:])
	case "audit":
		return command.AuditPolicy(args[1:])
	default:
		policyUsage()
		return fmt.Errorf("unknown policy subcommand %q", args[0])
	}
}

func policyUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops policy check [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--write] [--network] [--shell --shell-reason TEXT] [--json] -- ARGV...
  issueops policy fake-run [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--write] [--network] [--shell --shell-reason TEXT] [--json] -- ARGV...
  issueops policy run --read-only [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--json] -- ARGV...
  issueops policy audit [--workspace-root PATH] [--cwd PATH] [--timeout=30s] [--env=NAME,NAME] [--write] [--network] [--shell --shell-reason TEXT] [--json] -- ARGV...
`)
}

func (command Command) Check(args []string) error {
	req, jsonOut, err := command.ParseFlags("policy check", args)
	if err != nil {
		return err
	}
	result := command.Policy.Evaluate(req)
	if jsonOut {
		return printJSON(result)
	}
	printPolicyEvaluation(result)
	return nil
}

func (command Command) FakeRun(args []string) error {
	req, jsonOut, err := command.ParseFlags("policy fake-run", args)
	if err != nil {
		return err
	}
	result := command.Policy.FakeRun(req)
	if jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		printPolicyEvaluation(result.Policy)
		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Fprint(os.Stderr, result.Stderr)
		}
	}
	if !result.Policy.Allowed {
		return policydomain.PolicyDeniedError{Reasons: result.Policy.DenyReasons}
	}
	return nil
}

func (command Command) RunReadOnly(args []string) error {
	req, jsonOut, readOnly, err := command.ParseRunFlags(args)
	if err != nil {
		return err
	}
	if !readOnly {
		return fmt.Errorf("policy run currently requires --read-only")
	}
	result := command.Policy.RunReadOnly(req)
	if jsonOut {
		if err := printJSON(result); err != nil {
			return err
		}
	} else {
		printPolicyEvaluation(result.Policy)
		if result.Stdout != "" {
			fmt.Print(result.Stdout)
		}
		if result.Stderr != "" {
			fmt.Fprint(os.Stderr, result.Stderr)
		}
	}
	if !result.Policy.Allowed {
		return policydomain.PolicyDeniedError{Reasons: result.Policy.DenyReasons}
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("command exited %d", result.ExitCode)
	}
	return nil
}

func (command Command) AuditPolicy(args []string) error {
	req, jsonOut, err := command.ParseFlags("policy audit", args)
	if err != nil {
		return err
	}
	result, err := command.Audit.Audit(req)
	if jsonOut {
		if printErr := printJSON(result); printErr != nil {
			return printErr
		}
	} else {
		printPolicyEvaluation(result.Policy)
		fmt.Printf("audit log: %s\n", result.LogPath)
	}
	return err
}

func printPolicyEvaluation(result policydomain.CommandPolicyEvaluation) {
	if result.Allowed {
		fmt.Printf("policy allowed: %s\n", result.AuditLogID)
		return
	}
	fmt.Printf("policy denied: %s\n", result.AuditLogID)
	for _, reason := range result.DenyReasons {
		fmt.Printf("- %s\n", reason)
	}
}
