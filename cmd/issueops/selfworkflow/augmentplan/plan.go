package augmentplan

import (
	"time"

	app "issueops/internal/application/selfaugment"
	contract "issueops/internal/contract/selfaugment"
	port "issueops/internal/port/selfaugment"
)

type Request = contract.SelfAugmentPlanRequest
type Result = contract.SelfAugmentPlanResult

var Repository port.PlanRepository

func planner() app.Planner {
	return app.Planner{Repository: Repository, DocsIndex: DocsIndex, ListSkillNames: ListSkillNames, StateList: StateList, StateRead: StateRead, Now: time.Now}
}
func Plan(req Request, root, version string) Result { return planner().Plan(req, root, version) }
