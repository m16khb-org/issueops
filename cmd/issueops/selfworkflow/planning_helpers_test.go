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
	domain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
	"time"
)

func planSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	return (app.Planner{Repository: augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}, DocsIndex: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).Index, ListSkillNames: install.ListSkillNames, StateList: state.NewService().List, StateRead: state.NewService().Read, Now: time.Now}).Plan(req, IssueOpsRoot(), Version)
}
func PlanSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	return planSelfAugmentation(req)
}
func SaveSelfAugmentLesson(req SelfAugmentLessonRequest) (SelfAugmentLessonResult, error) {
	return app.SaveLesson(req, app.SaveLessonDeps{IssueOpsRoot: IssueOpsRoot, SelectCandidate: func() *SelfAugmentCandidate {
		return planSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}).SelectedCandidate
	}, Now: time.Now, Encode: func(snapshot contract.SelfAugmentLessonStateSnapshot) ([]byte, error) {
		return json.MarshalIndent(snapshot, "", "  ")
	}, Write: func(key, content string) (statecontract.StateResult, error) {
		return state.NewService().Write(context.Background(), key, content)
	}, StateDir: state.StateDir, Prune: func(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.StatePruneResult, error) {
		return state.NewService().PrunePrefix(context.Background(), prefix, maxAge, maxRecords, confirm)
	}})
}
func StateKeySlug(s string) string { return domain.StateKeySlug(s) }
func ExportSelfVerificationCandidates() SelfVerificationCandidateExportResult {
	return verifyapp.ExportCandidates(IssueOpsRoot(), verifyapp.ExportCandidatesDeps{Source: verification.CandidateSource, Now: time.Now})
}
func SaveSelfVerificationCandidateExport(result *SelfVerificationCandidateExportResult, key string) error {
	return verifyapp.SaveCandidateExport(result, key, verifyapp.SaveCandidateExportDeps{Now: time.Now, Encode: func(snapshot contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error) {
		return json.MarshalIndent(snapshot, "", "  ")
	}, Write: func(key, content string) (statecontract.StateResult, error) {
		return state.NewService().Write(context.Background(), key, content)
	}, StateDir: state.StateDir})
}
func SelectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	return verifydomain.SelectedCandidateID(candidate)
}
func SelfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	return verifydomain.CandidateIDsByStatus(candidates, status)
}
