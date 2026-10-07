package benchmarkcmd

import (
	"errors"
	"flag"
	"fmt"
	"issueops/cmd/issueops/jsonout"
	app "issueops/internal/application/issueopsbenchmark"
	"strings"
)

type Command struct{ Service *app.Service }

func (c Command) Run(args []string) error {
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		fmt.Println("Usage: issueops benchmark run|compare|gate|reliability|consensus [--json]")
		return nil
	}
	handlers := map[string]func([]string) error{"run": c.runBenchmarkRun, "compare": c.runBenchmarkCompare, "gate": c.runBenchmarkGate, "reliability": c.runBenchmarkReliability, "consensus": c.runBenchmarkConsensus}
	handler, ok := handlers[args[0]]
	if !ok {
		return fmt.Errorf("unknown issueops benchmark subcommand %q", args[0])
	}
	if c.Service == nil {
		return errors.New("issueops benchmark is not configured")
	}
	return handler(args[1:])
}

func parseFlags(fs *flag.FlagSet, args []string) (bool, error) {
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

var printJSON = jsonout.Print

type repeatedFlag []string

func (f *repeatedFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *repeatedFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}
