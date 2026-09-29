package gatescli

import (
	adapter "issueops/internal/adapter/gates"
	policyadapter "issueops/internal/adapter/policy"
	app "issueops/internal/application/gates"
	model "issueops/internal/contract/gates"
)

func gateServiceForTest() app.Service {
	return app.Service{Store: adapter.FileStore{}, Clock: adapter.Clock{}, Runner: app.CommandRunner{Evaluate: policyadapter.EvaluateCommandPolicy, Execute: policyadapter.RunCommand}}
}

func adapterCheck(req model.CheckRequest) (model.CheckResult, error) {
	return gateServiceForTest().Check(req)
}
func adapterInit(req model.InitRequest) (model.InitResult, error) {
	return gateServiceForTest().Init(req)
}
func adapterAbandon(req model.AbandonRequest) (model.AbandonResult, error) {
	return gateServiceForTest().Abandon(req)
}
