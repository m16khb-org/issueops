package projectcli

import (
	"flag"
	"fmt"
	"os"

	"issueops/cmd/issueops/cliflag"
	"issueops/cmd/issueops/jsonout"
	app "issueops/internal/application/lintdiagnose"
	lintdiagnosecontract "issueops/internal/contract/lintdiagnose"
)

func runProjectLintDiagnose(service app.Service, args []string) error {
	fs := flag.NewFlagSet("project lint-diagnose", flag.ContinueOnError)
	repo := fs.String("repo", ".", "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")

	if err := cliflag.Parse(fs, args); err != nil {
		return err
	}

	commandArgv := fs.Args()
	if len(commandArgv) == 0 {
		return fmt.Errorf("missing command to run. Usage: issueops project lint-diagnose [flags] -- <command_to_run...>")
	}

	result, err := service.Diagnose(lintdiagnosecontract.Request{
		RepoRoot:    *repo,
		CommandArgv: commandArgv,
	})
	if err != nil {
		return err
	}

	if *jsonOut {
		return printJSON(result)
	}

	if result.Failed {
		fmt.Fprintf(os.Stderr, "\n--- Host-Agent Diagnostic Prompt ---\n%s\n------------------------------------\n", result.Prompt)
		// The prompt is the whole report; Reported keeps the root command from
		// adding an error line, and the root maps ExitCodeError to the exit status.
		return jsonout.Reported(ExitCodeError{Code: result.ExitCode})
	}
	fmt.Println("Command completed successfully. No failure detected.")
	return nil
}

// ExitCodeError reports that the diagnosed command failed with exit status Code
// and that the process must end with the same status.
type ExitCodeError struct{ Code int }

func (e ExitCodeError) Error() string { return fmt.Sprintf("command exited %d", e.Code) }
