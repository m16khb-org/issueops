package intentdesign

import (
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/domain/issueopsintent"
)

// RecordPlanPrep stores the pre-plan evidence gate: prior-decision lookup,
// related-issue scoring, web research, and the codebase survey. Each item must
// carry either evidence or a waive reason (mutually exclusive). The plan
// readiness gate then checks these for non-trivial intent classes.
func RecordPlanPrep(store Store, stateRoot, id string, req model.IssueOpsPlanPrepRequest) (model.IssueOpsRecord, error) {
	decisions, err := planPrepRecordItem("decisions", req.PriorDecisions)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	related, err := planPrepRecordItem("related_issues", req.RelatedIssues)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	research, err := planPrepRecordItem("web_research", req.WebResearch)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	survey, err := planPrepRecordItem("codebase_survey", req.CodebaseSurvey)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.PlanPrep = &model.IssueOpsPlanPrep{
		PriorDecisions: decisions,
		RelatedIssues:  related,
		WebResearch:    research,
		CodebaseSurvey: survey,
		RecordedAt:     time.Now().UTC().Format(time.RFC3339Nano),
	}
	return store.TouchWrite(stateRoot, record)
}

func planPrepRecordItem(name string, req model.IssueOpsPlanPrepItemRequest) (model.IssueOpsPlanPrepItem, error) {
	item, err := issueopsintent.BuildPlanPrepItem(name, req.Evidence, req.WaiveReason)
	if err != nil {
		return model.IssueOpsPlanPrepItem{}, err
	}
	return model.IssueOpsPlanPrepItem{Status: item.Status, Evidence: item.Evidence, WaiveReason: item.WaiveReason}, nil
}
