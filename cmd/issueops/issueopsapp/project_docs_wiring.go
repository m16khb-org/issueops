package issueopsapp

import (
	"os"
	"path/filepath"

	docsfiles "issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
	docsapp "issueops/internal/application/projectdocs"
)

func newProjectDocsService(defaultRoot string) docsapp.Service {
	return projectDocsServiceWith(newRepoRootResolver(defaultRoot))
}

func newScopedProjectDocsService(root, cwd string) docsapp.Service {
	return projectDocsServiceWith(newRepoRootResolverAt(root, cwd, nil))
}

func projectDocsServiceWith(normalizeRoot func(string) (string, error)) docsapp.Service {
	return docsapp.Service{
		NormalizeRoot: normalizeRoot,
		Revision:      docsfiles.RevisionFiles{},
		Routing:       docsfiles.RouteFiles{},
		Appending:     docsfiles.AppendFiles{},
	}
}

func newRepoRootResolver(defaultRoot string) func(string) (string, error) {
	cwd, cwdErr := os.Getwd()
	return newRepoRootResolverAt(defaultRoot, cwd, cwdErr)
}

// newRepoRootResolverAt resolves relative roots against an explicit base, so
// request-scoped services never fall back to the process cwd.
func newRepoRootResolverAt(defaultRoot, cwd string, cwdErr error) func(string) (string, error) {
	return func(root string) (string, error) {
		if root == "" {
			root = defaultRoot
		}
		if !filepath.IsAbs(root) {
			if cwdErr != nil {
				return "", cwdErr
			}
			root = filepath.Join(cwd, root)
		}
		return repopath.NormalizeRoot(root)
	}
}
