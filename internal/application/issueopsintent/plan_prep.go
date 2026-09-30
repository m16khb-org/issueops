package issueopsintent

import (
	model "issueops/internal/contract/issueops"
	intentdomain "issueops/internal/domain/issueopsintent"
	intentport "issueops/internal/port/issueopsintent"
)

func RecordPlanPrep(store intentport.Store, stateRoot, id string, req model.IssueOpsPlanPrepRequest) (model.IssueOpsRecord, error) {
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
		RecordedAt:     store.Now(),
	}
	return store.TouchWrite(stateRoot, record)
}

func planPrepRecordItem(name string, req model.IssueOpsPlanPrepItemRequest) (model.IssueOpsPlanPrepItem, error) {
	item, err := intentdomain.BuildPlanPrepItem(name, req.Evidence, req.WaiveReason)
	if err != nil {
		return model.IssueOpsPlanPrepItem{}, err
	}
	return model.IssueOpsPlanPrepItem{Status: item.Status, Evidence: item.Evidence, WaiveReason: item.WaiveReason}, nil
}
