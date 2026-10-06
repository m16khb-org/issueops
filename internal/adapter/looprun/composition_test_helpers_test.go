package looprun

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	loopapp "issueops/internal/application/looprun"
	loopcontract "issueops/internal/contract/looprun"
)

func testLoopStateRoot() string { return filepath.Join(statestore.StateDir(), "loop") }
func testLoopStore() Store {
	return Store{Directory: testLoopStateRoot(), OpenDatabase: func(dir string) (StateDatabase, error) { return sqlstore.Open(dir) }, GetExisting: sqlstore.GetExisting, ListExisting: sqlstore.ListExisting}
}
func testLoopIdentity() Identity {
	cwd, err := os.Getwd()
	return Identity{BaseDir: cwd, BaseError: err}
}
func testLoopService() loopapp.Service {
	return loopapp.Service{Store: testLoopStore(), Identity: testLoopIdentity(), Clock: Clock{Time: time.Now}, SchemaVersion: LoopRunCurrentSchemaVersion}
}
func testLoopReader() loopapp.Reader {
	return loopapp.Reader{Store: testLoopStore(), Identity: testLoopIdentity()}
}

func Start(req loopcontract.StartLoopRequest) (loopcontract.LoopRun, error) {
	return testLoopService().Start(context.Background(), req)
}
func Stop(id string, success bool, reason string) (loopcontract.LoopRun, error) {
	return testLoopService().Stop(context.Background(), id, success, reason)
}
func RecordAttempt(id string, req loopcontract.RecordAttemptRequest) (loopcontract.LoopRun, error) {
	return testLoopService().RecordAttempt(context.Background(), id, req)
}
func ReadLoop(id string) (loopcontract.LoopRun, error) { return testLoopStore().Read(id) }

func RepoGateSummaryFor(repo string) (loopcontract.RepoGateSummary, []string) {
	return testLoopReader().RepoGateSummaryFor(repo)
}

func Status(id string) (loopcontract.StatusResult, error) { return testLoopService().Status(id) }
