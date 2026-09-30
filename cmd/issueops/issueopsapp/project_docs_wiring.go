package issueopsapp

import (
	"os"
	"path/filepath"

	docsfiles "issueops/internal/adapter/projectdocs"
	"issueops/internal/adapter/repopath"
	docsapp "issueops/internal/application/projectdocs"
)

func newProjectDocsService(defaultRoot string) docsapp.Service {
	return docsapp.Service{
		NormalizeRoot: newRepoRootResolver(defaultRoot),
		Revision:      docsfiles.RevisionFiles{},
		Routing:       docsfiles.RouteFiles{},
		Appending:     docsfiles.AppendFiles{},
	}
}

func newRepoRootResolver(defaultRoot string) func(string) (string, error) {
	cwd, cwdErr := os.Getwd()
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
