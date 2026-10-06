package statecli

func Run(deps Dependencies, args []string) error {
	return runState(deps, args)
}
