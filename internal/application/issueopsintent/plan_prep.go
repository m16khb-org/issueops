package issueopsintent

import (
	model "issueops/internal/contract/issueops"
	intentdomain "issueops/internal/domain/issueopsintent"
	intentport "issueops/internal/port/issueopsintent"
)

// RecordPlanPrep validates the plan preparation items in req (prior decisions,
// related issues, web research, codebase survey), stores them on record id and
// returns the updated record.
func RecordPlanPrep(store intentport.Store, stateRoot, id string, req model.PlanPrepRequest) (model.Record, error) {
	decisions, err := planPrepRecordItem("decisions", req.PriorDecisions)
	if err != nil {
		return model.Record{OK: false}, err
	}
	related, err := planPrepRecordItem("related_issues", req.RelatedIssues)
	if err != nil {
		return model.Record{OK: false}, err
	}
	research, err := planPrepRecordItem("web_research", req.WebResearch)
	if err != nil {
		return model.Record{OK: false}, err
	}
	survey, err := planPrepRecordItem("codebase_survey", req.CodebaseSurvey)
	if err != nil {
		return model.Record{OK: false}, err
	}
	record, err := store.Read(stateRoot, id)
	if err != nil {
		return record, err
	}
	record.PlanPrep = &model.PlanPrep{
		PriorDecisions: decisions,
		RelatedIssues:  related,
		WebResearch:    research,
		CodebaseSurvey: survey,
		RecordedAt:     store.Now(),
	}
	return store.TouchWrite(stateRoot, record)
}

func planPrepRecordItem(name string, req model.PlanPrepItemRequest) (model.PlanPrepItem, error) {
	item, err := intentdomain.BuildPlanPrepItem(name, req.Evidence, req.WaiveReason)
	if err != nil {
		return model.PlanPrepItem{}, err
	}
	return model.PlanPrepItem{Status: item.Status, Evidence: item.Evidence, WaiveReason: item.WaiveReason}, nil
}
