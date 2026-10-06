package gates

import (
	policyadapter "issueops/internal/adapter/policy"
	app "issueops/internal/application/gates"
	policyapp "issueops/internal/application/policy"
	model "issueops/internal/contract/gates"
)

func gateServiceForTest() app.Service {
	policy := policyapp.Service{Observer: policyadapter.CommandObserver{}, Overrides: policyadapter.OverrideLoader{}, Executor: policyadapter.CommandExecutor{}, Clock: policyadapter.Clock{}}
	return app.Service{Store: FileStore{}, Clock: Clock{}, Runner: app.CommandRunner{Evaluate: policy.Evaluate, Execute: policy.Run}}
}

func Check(req model.CheckRequest) (model.CheckResult, error) { return gateServiceForTest().Check(req) }
func Init(req model.InitRequest) (model.InitResult, error)    { return gateServiceForTest().Init(req) }
func Abandon(req model.AbandonRequest) (model.AbandonResult, error) {
	return gateServiceForTest().Abandon(req)
}
