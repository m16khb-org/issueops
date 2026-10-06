package issueopspreparation

import (
	"fmt"
	"strings"
)

// requireArtifactDir rejects a preparation whose linked issue URL carries no
// issue number: sealed artifacts live only under .issueops/issues/<n>/artifact.
func requireArtifactDir(artifactDir string) error {
	if strings.TrimSpace(artifactDir) == "" {
		return fmt.Errorf("linked issue URL has no issue number; execution prepare needs .issueops/issues/<n>/artifact for sealed artifacts")
	}
	return nil
}
