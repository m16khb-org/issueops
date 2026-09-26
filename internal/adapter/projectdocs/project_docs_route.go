package projectdocs

import (
	"os"
	"path/filepath"
	"time"

	projectdocsapp "issueops/internal/application/projectdocs"
	projectdocscontract "issueops/internal/contract/projectdocs"
)

func RouteProjectDocs(repoRoot, task string) (projectdocscontract.ProjectDocsRouteResult, error) {
	root, err := NormalizeRepoRoot(repoRoot)
	if err != nil {
		return projectdocscontract.ProjectDocsRouteResult{}, err
	}
	return projectdocsapp.Route(root, task, routeFileEffects{}), nil
}

type routeFileEffects struct{}

func (routeFileEffects) Path(root, rel string) string {
	return filepath.Join(root, filepath.FromSlash(rel))
}
func (routeFileEffects) Exists(path string) bool { _, err := os.Stat(path); return err == nil }
func (routeFileEffects) Now() time.Time          { return time.Now() }
