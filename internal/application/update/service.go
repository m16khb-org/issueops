package update

// Installer owns the host process and filesystem boundary for a binary update.
type Installer interface {
	Install(root string, args []string) error
	RefreshDaemon() error
}

type Options struct {
	Root         string
	ProjectLocal bool
	DryRun       bool
	PathMode     string
	Interactive  bool
	JSON         bool
	SkipBuild    bool
}

type Service struct{ Installer Installer }

func (service Service) Run(options Options) error {
	args := make([]string, 0, 6)
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
	if err := service.Installer.Install(options.Root, args); err != nil {
		return err
	}
	if options.DryRun {
		return nil
	}
	return service.Installer.RefreshDaemon()
}
