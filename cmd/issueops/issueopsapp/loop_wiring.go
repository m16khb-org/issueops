package issueopsapp

import (
	"os"
	"path/filepath"
	"time"

	"issueops/cmd/issueops/loopcli"
	adapter "issueops/internal/adapter/looprun"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/looprun"
)

func newLoopStore() adapter.Store {
	return adapter.Store{Directory: filepath.Join(statestore.StateDir(), "loop"),
		OpenDatabase: func(dir string) (adapter.StateDatabase, error) { return sqlstore.Open(dir) },
		GetExisting:  sqlstore.GetExisting, ListExisting: sqlstore.ListExisting}
}
func newLoopIdentity() adapter.Identity {
	cwd, err := os.Getwd()
	return adapter.Identity{BaseDir: cwd, BaseError: err}
}
func newLoopService() app.Service {
	return loopServiceWith(newLoopIdentity())
}
func newScopedLoopService(cwd string) app.Service {
	service := loopServiceWith(adapter.Identity{BaseDir: cwd})
	service.Store = grantFencedLoopStore{Store: newLoopStore(), grantRoot: issueOpsStateRoot()}
	return service
}
func loopServiceWith(identity adapter.Identity) app.Service {
	return app.Service{Store: newLoopStore(), Identity: identity, Clock: adapter.Clock{Time: time.Now}, SchemaVersion: adapter.LoopRunCurrentSchemaVersion}
}
func newLoopReader() app.Reader {
	return app.Reader{Store: newLoopStore(), Identity: newLoopIdentity()}
}
func loopDependencies() loopcli.Dependencies {
	service := newLoopService()
	return loopcli.Dependencies{Start: service.Start, RecordAttempt: service.RecordAttempt, Stop: service.Stop, Status: service.Status, ResolveID: service.ResolveID}
}
