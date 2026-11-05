package selfworkflow

import (
	"context"
	"encoding/json"
	"issueops/internal/adapter/augmentation"
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/install"
	state "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification"
	docsapp "issueops/internal/application/docs"
	app "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	"time"
)

func planSelfAugmentation(req contract.PlanRequest) contract.PlanResult {
	return (app.Planner{Repository: augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}, DocsIndex: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).Index, ListSkillNames: install.ListSkillNames, StateList: state.NewService().List, StateRead: state.NewService().Read, Now: time.Now}).Plan(req, IssueOpsRoot(), Version)
}

func SaveSelfAugmentLesson(req contract.LessonRequest) (contract.LessonResult, error) {
	return app.SaveLesson(req, app.SaveLessonDeps{IssueOpsRoot: IssueOpsRoot, SelectCandidate: func() *contract.Candidate {
		return planSelfAugmentation(contract.PlanRequest{Cycles: 1, TargetScore: 95}).SelectedCandidate
	}, Now: time.Now, Encode: func(snapshot contract.LessonStateSnapshot) ([]byte, error) {
		return json.MarshalIndent(snapshot, "", "  ")
	}, Write: func(key, content string) (statecontract.Result, error) {
		return state.NewService().Write(context.Background(), key, content)
	}, StateDir: state.Dir, Prune: func(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.PruneResult, error) {
		return state.NewService().PrunePrefix(context.Background(), prefix, maxAge, maxRecords, confirm)
	}})
}

func ExportSelfVerificationCandidates() contract.SelfVerificationCandidateExportResult {
	return verifyapp.ExportCandidates(IssueOpsRoot(), verifyapp.ExportCandidatesDeps{Source: verification.CandidateSource, Now: time.Now})
}
func SaveSelfVerificationCandidateExport(result *contract.SelfVerificationCandidateExportResult, key string) error {
	return verifyapp.SaveCandidateExport(result, key, verifyapp.SaveCandidateExportDeps{Now: time.Now, Encode: func(snapshot contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error) {
		return json.MarshalIndent(snapshot, "", "  ")
	}, Write: func(key, content string) (statecontract.Result, error) {
		return state.NewService().Write(context.Background(), key, content)
	}, StateDir: state.Dir})
}
