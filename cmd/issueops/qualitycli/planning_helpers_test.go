package qualitycli

import (
	"context"
	"encoding/json"
	"issueops/cmd/issueops/mcpcli"
	"issueops/internal/adapter/augmentation"
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/install"
	statestore "issueops/internal/adapter/outbound/state"
	"issueops/internal/adapter/verification"
	docsapp "issueops/internal/application/docs"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	"time"
)

func planningForTest(root, dir, version string) mcpcli.SelfPlanningDependencies {

	planner := augmentapp.Planner{Repository: augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}, DocsIndex: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).Index, ListSkillNames: install.ListSkillNames, StateList: statestore.NewService().List, StateRead: statestore.NewService().Read, Now: time.Now}
	plan := func(req contract.PlanRequest) contract.PlanResult {
		return planner.Plan(req, root, version)
	}
	return mcpcli.SelfPlanningDependencies{
		Plan: plan,
		ExportCandidates: func() contract.SelfVerificationCandidateExportResult {
			return verifyapp.ExportCandidates(root, verifyapp.ExportCandidatesDeps{Source: verification.CandidateSource, Now: time.Now})
		},
		SaveCandidates: func(_ context.Context, result *contract.SelfVerificationCandidateExportResult, key string) error {
			return verifyapp.SaveCandidateExport(result, key, verifyapp.SaveCandidateExportDeps{Now: time.Now, Encode: func(snapshot contract.SelfVerificationCandidateExportStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: func(key, content string) (statecontract.Result, error) {
				return statestore.NewService().Write(context.Background(), key, content)
			}, StateDir: func() string { return dir }})
		},
		SaveLesson: func(_ context.Context, req contract.LessonRequest) (contract.LessonResult, error) {
			return augmentapp.SaveLesson(req, augmentapp.SaveLessonDeps{IssueOpsRoot: func() string { return root }, SelectCandidate: func() *contract.Candidate {
				return plan(contract.PlanRequest{Cycles: 1, TargetScore: 95}).SelectedCandidate
			}, Now: time.Now, Encode: func(snapshot contract.LessonStateSnapshot) ([]byte, error) {
				return json.MarshalIndent(snapshot, "", "  ")
			}, Write: func(key, content string) (statecontract.Result, error) {
				return statestore.NewService().Write(context.Background(), key, content)
			}, StateDir: func() string { return dir }, Prune: func(prefix string, maxAge time.Duration, maxRecords int, confirm bool) (statecontract.PruneResult, error) {
				return statestore.NewService().PrunePrefix(context.Background(), prefix, maxAge, maxRecords, confirm)
			}})
		},
	}
}
