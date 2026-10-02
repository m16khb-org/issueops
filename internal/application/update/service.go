package update

// Installer owns the host process and filesystem boundary for a binary update.
type Installer interface {
	Install(root string, args []string) error
}

type Options struct {
	Root         string
	ProjectLocal bool
	DryRun       bool
	PathMode     string
	Interactive  bool
	JSON         bool
	SkipBuild    bool
	MCPTransport string
}

type Service struct {
	Installer     Installer
	RefreshDaemon func() error
}

func (service Service) Run(options Options) error {
	args := make([]string, 0, 7)
	if options.ProjectLocal {
		args = append(args, "--project-local")
	}
	if options.DryRun {
		args = append(args, "--dry-run")
	}
	if options.PathMode != "" {
		args = append(args, "--path-mode="+options.PathMode)
	}
	if options.Interactive {
		args = append(args, "--interactive")
	}
	if options.JSON {
		args = append(args, "--json")
	}
	if options.SkipBuild {
		args = append(args, "--skip-build")
	}
	if options.MCPTransport != "" {
		args = append(args, "--mcp-transport="+options.MCPTransport)
	}
	if err := service.Installer.Install(options.Root, args); err != nil {
		return err
	}
	if options.DryRun {
		return nil
	}
	return service.RefreshDaemon()
}
