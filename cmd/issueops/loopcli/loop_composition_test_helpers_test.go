package loopcli

import (
	"os"
	"path/filepath"
	"time"

	loopadapter "issueops/internal/adapter/looprun"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	loopapp "issueops/internal/application/looprun"
)

func testLoopStateRoot() string { return filepath.Join(statestore.StateDir(), "loop") }
func testLoopStore() loopadapter.Store {
	return loopadapter.Store{Directory: testLoopStateRoot(), OpenDatabase: func(dir string) (loopadapter.StateDatabase, error) { return sqlstore.Open(dir) }, GetExisting: sqlstore.GetExisting, ListExisting: sqlstore.ListExisting}
}
func testLoopIdentity() loopadapter.Identity {
	cwd, err := os.Getwd()
	return loopadapter.Identity{BaseDir: cwd, BaseError: err}
}
func testLoopService() loopapp.Service {
	return loopapp.Service{Store: testLoopStore(), Identity: testLoopIdentity(), Clock: loopadapter.Clock{Time: time.Now}, SchemaVersion: loopadapter.LoopRunCurrentSchemaVersion}
}
