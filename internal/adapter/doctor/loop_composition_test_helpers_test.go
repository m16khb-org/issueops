package doctor

import (
	"os"
	"path/filepath"

	loopadapter "issueops/internal/adapter/looprun"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	loopapp "issueops/internal/application/looprun"
	loopcontract "issueops/internal/contract/looprun"
)

func testLoopStateRoot() string { return filepath.Join(statestore.StateDir(), "loop") }
func testLoopStore() loopadapter.Store {
	return loopadapter.Store{Directory: testLoopStateRoot(), OpenDatabase: func(dir string) (loopadapter.StateDatabase, error) { return sqlstore.Open(dir) }, GetExisting: sqlstore.GetExisting, ListExisting: sqlstore.ListExisting}
}
func testLoopIdentity() loopadapter.Identity {
	cwd, err := os.Getwd()
	return loopadapter.Identity{BaseDir: cwd, BaseError: err}
}

func testLoopReader() loopapp.Reader {
	return loopapp.Reader{Store: testLoopStore(), Identity: testLoopIdentity()}
}

func testLoopRepoGateSummaryFor(repo string) (loopcontract.RepoGateSummary, []string) {
	return testLoopReader().RepoGateSummaryFor(repo)
}
