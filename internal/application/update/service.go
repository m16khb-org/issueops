// Package update is the application service behind `issueops update`: it turns
// update options into installer arguments and cleans up stale MCP proxy
// processes. Process and filesystem effects stay behind the installer and
// MCPProxyEffects ports.
package update

// installer owns the host process and filesystem boundary for a binary update.
type installer interface {
	Install(root string, args []string) error
}

// Options are the user-selected switches of an update run. Each set switch is
// forwarded to the installer as the matching command-line flag.
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

// Service runs binary updates through its Installer.
type Service struct {
	Installer installer
}

// Run translates options into installer flags and invokes the installer for
// options.Root. It returns the installer's error unchanged.
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
	return service.Installer.Install(options.Root, args)
}
