package issueopsapp

import (
	"os"
	"path/filepath"

	"issueops/cmd/issueops/hookcli"
	"issueops/cmd/issueops/hookcli/hookcatalog"
	pathutil "issueops/cmd/issueops/pathutil"
	renderer "issueops/internal/adapter/hookprompt"
	"issueops/internal/adapter/hostprotocol"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/projectdoc"
	app "issueops/internal/application/hookprompt"
	hookcontract "issueops/internal/contract/hookprompt"
)

// hookWorkTreeRoot returns the nearest ancestor of dir holding a .git entry
// (a checkout or a linked worktree), or dir itself outside Git. A session
// started in a subdirectory then gets the same catalog as one started at the
// root: before, it found no .issueops there and injected nothing, and its
// system prefix differed from root sessions, so prompt caches did not match.
// It walks the filesystem instead of running git to keep the hook fast.
func hookWorkTreeRoot(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return dir
	}
	for current := abs; ; current = filepath.Dir(current) {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current
		}
		if filepath.Dir(current) == current {
			return dir
		}
	}
}

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
		BuildCatalog:  func(repo string) hookcontract.ProjectDocCatalogContext { return service.Build(hookWorkTreeRoot(repo)) },
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
