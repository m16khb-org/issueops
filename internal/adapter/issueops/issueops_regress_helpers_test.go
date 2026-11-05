package issueops

import model "issueops/internal/contract/issueops"

func RegressIssueOpsForReplan(root, id, reason string) (model.Record, error) {
	return planningRecorderForTest(nil).Regress(root, id, reason)
}
func RegressIssueOpsForReplanWithActor(root, id, reason string, actor model.Actor) (model.Record, error) {
	return planningRecorderForTest(&actor).Regress(root, id, reason)
}
