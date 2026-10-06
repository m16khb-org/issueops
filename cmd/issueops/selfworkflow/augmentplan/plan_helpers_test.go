package augmentplan

import (
	"time"

	"issueops/internal/adapter/install"
	app "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
	port "issueops/internal/port/selfaugment"
)

var Repository port.PlanRepository

func planner() app.Planner {
	return app.Planner{Repository: Repository, DocsIndex: DocsIndex, ListSkillNames: install.ListSkillNames, StateList: StateList, StateRead: StateRead, Now: time.Now}
}
func Plan(req contract.SelfAugmentPlanRequest, root, version string) contract.SelfAugmentPlanResult {
	return planner().Plan(req, root, version)
}
