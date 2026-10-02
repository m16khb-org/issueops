package issueopsapp

import (
	"issueops/cmd/issueops/gatescli"
	adapter "issueops/internal/adapter/gates"
	app "issueops/internal/application/gates"
)

func newGatesService() app.Service {
	return gatesServiceWith(adapter.FileStore{})
}

func newScopedGatesService(root, cwd string) app.Service {
	return gatesServiceWith(adapter.WorkspaceFileStore{Root: root, CWD: cwd})
}

func gatesServiceWith(store app.LedgerStore) app.Service {
	policy := newPolicyService()
	return app.Service{Store: store, Clock: adapter.Clock{}, Runner: app.CommandRunner{Evaluate: policy.Evaluate, Execute: policy.Run}}
}
func gatesDependencies() gatescli.Dependencies {
	service := newGatesService()
	return gatescli.Dependencies{Check: service.Check, Init: service.Init, Abandon: service.Abandon}
}
