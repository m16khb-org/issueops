package issueopsapp

import (
	"encoding/json"
	"issueops/cmd/issueops/mcpcli"
	"issueops/internal/adapter/augmentation"
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/install"
	"issueops/internal/adapter/verification"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
	"time"
)

func newSelfWorkflowPlanning(root, dir, version string) mcpcli.SelfPlanningDependencies {
	state := newStateService(dir)
	planner := augmentapp.Planner{Repository: augmentation.Repository{ListDocs: docs.ListDocs}, DocsIndex: docs.DocsIndex, ListSkillNames: install.ListSkillNames, StateList: state.List, StateRead: state.Read, Now: time.Now}
	plan := func(req contract.SelfAugmentPlanRequest) contract.SelfAugmentPlanResult {
		return planner.Plan(req, root, version)
	}
	return mcpcli.SelfPlanningDependencies{
		Plan: plan,
		ExportCandidates: func() contract.SelfVerificationCandidateExportResult {
			return verifyapp.ExportCandidates(root, verifyapp.ExportCandidatesDeps{Source: verification.CandidateSource, Now: time.Now})
		},
		SaveCandidates: func(result *contract.SelfVerificationCandidateExportResult, key string) error {
			return verifyapp.SaveCandidateExport(result, key, verifyapp.SaveCandidateExportDeps{Now: time.Now, Encode: func(snapshot contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: state.Write, StateDir: func() string { return dir }})
		},
		SaveLesson: func(req contract.SelfAugmentLessonRequest) (contract.SelfAugmentLessonResult, error) {
			return augmentapp.SaveLesson(req, augmentapp.SaveLessonDeps{IssueOpsRoot: func() string { return root }, SelectCandidate: func() *contract.SelfAugmentCandidate {
				return plan(contract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}).SelectedCandidate
			}, Now: time.Now, Encode: func(snapshot contract.SelfAugmentLessonStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: state.Write, StateDir: func() string { return dir }, Prune: state.PrunePrefix})
		},
	}
}
