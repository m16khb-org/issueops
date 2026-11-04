package statecli

import (
	"flag"
	"fmt"
	"os"

	"issueops/cmd/issueops/cliflag"
)

// Run 함수는 `issueops state`의 하위 명령을 args[0]으로 골라 실행한다. 하위 명령이
// 없으면 usage를 출력하고 오류를 돌려주며, 도움말 인자는 flag.ErrHelp를 돌려준다.
func Run(deps Dependencies, args []string) error {
	if len(args) == 0 {
		stateUsage()
		return fmt.Errorf("missing state subcommand")
	}
	switch args[0] {
	case "write":
		return runStateWrite(deps, args[1:])
	case "read":
		return runStateRead(deps, args[1:])
	case "list":
		return runStateList(deps, args[1:])
	case "prune":
		return runStatePrune(deps, args[1:])
	case "doctor":
		return runStateDoctor(deps, args[1:])
	case "maintain":
		return runStateMaintain(deps, args[1:])
	case "-h", "--help", "help":
		stateUsage()
		return flag.ErrHelp
	default:
		return cliflag.UnknownSubcommand("state", args[0], "write", "read", "list", "prune", "doctor", "maintain")
	}
}

func stateUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  issueops state write --key KEY (--value TEXT|--input FILE|--stdin) [--json]
  issueops state read --key KEY [--json]
  issueops state list [--json]
  issueops state prune --max-age DURATION [--confirm] [--json]
  issueops state doctor [--json]
  issueops state maintain [--json]
`)
}

// flagProvided reports whether the parsed flag set saw name, in any syntax the
// flag package accepts (-name, --name, -name=v, --name=v), even with an empty value.
func flagProvided(fs *flag.FlagSet, name string) bool {
	provided := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == name {
			provided = true
		}
	})
	return provided
}
