package installcli

func (c Command) RunInstall(args []string) error {
	return c.runInstall(args)
}
