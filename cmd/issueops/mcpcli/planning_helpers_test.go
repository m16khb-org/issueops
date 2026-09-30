package mcpcli

import (
	"encoding/json"
	docsapp "issueops/internal/application/docs"
	"time"

	"issueops/internal/adapter/augmentation"
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/install"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
)

func planningForTest(root, dir, version string) SelfPlanningDependencies {

	planner := augmentapp.Planner{Repository: augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}, DocsIndex: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).Index, ListSkillNames: install.ListSkillNames, StateList: statestore.StateList, StateRead: statestore.StateRead, Now: time.Now}
	plan := func(req contract.SelfAugmentPlanRequest) contract.SelfAugmentPlanResult {
		return planner.Plan(req, root, version)
	}
	return SelfPlanningDependencies{
		Plan: plan,
		ExportCandidates: func() contract.SelfVerificationCandidateExportResult {
			return verifyapp.ExportCandidates(root, verifyapp.ExportCandidatesDeps{Source: verification.CandidateSource, Now: time.Now})
		},
		SaveCandidates: func(result *contract.SelfVerificationCandidateExportResult, key string) error {
			return verifyapp.SaveCandidateExport(result, key, verifyapp.SaveCandidateExportDeps{Now: time.Now, Encode: func(snapshot contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: statestore.StateWrite, StateDir: func() string { return dir }})
		},
		SaveLesson: func(req contract.SelfAugmentLessonRequest) (contract.SelfAugmentLessonResult, error) {
			return augmentapp.SaveLesson(req, augmentapp.SaveLessonDeps{IssueOpsRoot: func() string { return root }, SelectCandidate: func() *contract.SelfAugmentCandidate {
				return plan(contract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}).SelectedCandidate
			}, Now: time.Now, Encode: func(snapshot contract.SelfAugmentLessonStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: statestore.StateWrite, StateDir: func() string { return dir }, Prune: statestore.StatePrunePrefix})
		},
	}
}
