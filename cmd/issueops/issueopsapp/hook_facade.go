package issueopsapp

import (
	"path/filepath"

	"issueops/cmd/issueops/hookcli"
	"issueops/cmd/issueops/hookcli/hookcatalog"
	pathutil "issueops/cmd/issueops/pathutil"
	renderer "issueops/internal/adapter/hookprompt"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/projectdoc"
	app "issueops/internal/application/hookprompt"
)

func newHookConfig() hookcatalog.Config {
	target := pathutil.ResolveTarget("")
	service := app.CatalogService{
		DiscoverReport:        projectdoc.DiscoverProjectDocsReport,
		FormatCompact:         projectdoc.FormatProjectDocCatalog,
		RepoName:              hookRepoName,
		FormatUserView:        renderer.RenderProjectDocCatalogUserView,
		FormatCompactOmission: projectdoc.FormatProjectDocCatalogOmissions,
		FormatUserOmission:    renderer.RenderProjectDocCatalogOmissions,
	}
	return hookcatalog.Config{
		ResolveTarget: func(string) string { return target },
		BuildCatalog:  service.Build,
		PrintJSON:     printJSON,
		FormatContext: hostprotocol.FormatHookContext,
	}
}

// hookRepoName names the repository in the catalog notice. A linked worktree
// resolves to its source checkout so the notice shows the repository rather
// than the branch directory; unreadable Git metadata falls back to the
// directory name.
func hookRepoName(repo string) string {
	if root, err := install.ResolveStableNativeRoot(repo); err == nil {
		return filepath.Base(root)
	}
	abs, err := filepath.Abs(repo)
	if err != nil {
		return ""
	}
	return filepath.Base(abs)
}

func runHook(args []string) error {
	return hookcli.RunHook(args, newHookConfig())
}
