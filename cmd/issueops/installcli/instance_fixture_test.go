package installcli

import (
	"issueops/internal/adapter/installutil"
	installcontract "issueops/internal/contract/install"
	"path/filepath"
	"testing"
)

func testInstallCommand(t *testing.T, deps Deps) Command {
	t.Helper()
	deps.StateRoot = filepath.Join(t.TempDir(), "issueops")
	deps.EnsureSymlinkPlan = installutil.EnsureSymlinkPlan
	deps.PrepareManagedCommandPathCandidate = func(target, candidate, path string, adopt, dry bool) (ManagedCommandPathTransaction, installcontract.ManagedCommandPathPlan, error) {
		tx, plan, err := installutil.PrepareManagedCommandPathCandidate(target, candidate, path, adopt, dry)
		if tx == nil {
			return nil, plan, err
		}
		return tx, plan, err
	}
	return Command{Deps: deps}
}
