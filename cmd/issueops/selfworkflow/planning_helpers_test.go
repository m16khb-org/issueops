package selfworkflow

import (
	"encoding/json"
	"issueops/cmd/issueops/selfworkflow/augmentlesson"
	"issueops/internal/adapter/augmentation"
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/install"
	state "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification"
	app "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
	domain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
	"time"
)

func planSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	return (app.Planner{Repository: augmentation.Repository{ListDocs: docs.ListDocs}, DocsIndex: docs.DocsIndex, ListSkillNames: install.ListSkillNames, StateList: state.StateList, StateRead: state.StateRead, Now: time.Now}).Plan(req, IssueOpsRoot(), Version)
}
func PlanSelfAugmentation(req SelfAugmentPlanRequest) SelfAugmentPlanResult {
	return planSelfAugmentation(req)
}
func SaveSelfAugmentLesson(req SelfAugmentLessonRequest) (SelfAugmentLessonResult, error) {
	return app.SaveLesson(req, app.SaveLessonDeps{IssueOpsRoot: IssueOpsRoot, SelectCandidate: func() *SelfAugmentCandidate {
		return planSelfAugmentation(SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}).SelectedCandidate
	}, Now: time.Now, Encode: func(snapshot contract.SelfAugmentLessonStateSnapshot) ([]byte, error) {
		return json.MarshalIndent(snapshot, "", "  ")
	}, Write: state.StateWrite, StateDir: state.StateDir, Prune: state.StatePrunePrefix})
}
func RunSelfAugmentLesson(args []string) error {
	return augmentlesson.Run(args, augmentlesson.Deps{Save: SaveSelfAugmentLesson, PrintJSON: printJSON})
}
func StateKeySlug(s string) string { return domain.StateKeySlug(s) }
func ExportSelfVerificationCandidates() SelfVerificationCandidateExportResult {
	return verifyapp.ExportCandidates(IssueOpsRoot(), verifyapp.ExportCandidatesDeps{Source: verification.CandidateSource, Now: time.Now})
}
func SaveSelfVerificationCandidateExport(result *SelfVerificationCandidateExportResult, key string) error {
	return verifyapp.SaveCandidateExport(result, key, verifyapp.SaveCandidateExportDeps{Now: time.Now, Encode: func(snapshot contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error) {
		return json.MarshalIndent(snapshot, "", "  ")
	}, Write: state.StateWrite, StateDir: state.StateDir})
}
func SelectedSelfVerificationCandidateID(candidate *SelfVerificationCandidate) string {
	return verifydomain.SelectedCandidateID(candidate)
}
func SelfVerificationCandidateCatalog() []SelfVerificationCandidate {
	return verifydomain.CandidateCatalog()
}
func SelfVerificationCandidateIDsByStatus(candidates []SelfVerificationCandidate, status string) []string {
	return verifydomain.CandidateIDsByStatus(candidates, status)
}
