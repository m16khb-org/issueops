package gates

import (
	policyadapter "issueops/internal/adapter/policy"
	app "issueops/internal/application/gates"
	model "issueops/internal/contract/gates"
)

func gateServiceForTest() app.Service {
	return app.Service{Store: FileStore{}, Clock: Clock{}, Runner: app.CommandRunner{Evaluate: policyadapter.EvaluateCommandPolicy, Execute: policyadapter.RunCommand}}
}

func Check(req model.CheckRequest) (model.CheckResult, error) { return gateServiceForTest().Check(req) }
func Init(req model.InitRequest) (model.InitResult, error)    { return gateServiceForTest().Init(req) }
func Abandon(req model.AbandonRequest) (model.AbandonResult, error) {
	return gateServiceForTest().Abandon(req)
}
