package issueopsapp

import (
	"context"
	"issueops/cmd/issueops/qualitycli"
	outbound "issueops/internal/adapter/outbound/quality"
	app "issueops/internal/application/quality"
	contract "issueops/internal/contract/quality"
	augmentcontract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	catalog "issueops/internal/domain/qualitycatalog"
	augmentdomain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
	"path/filepath"
	"time"
)

func newQualityDependencies(root, dir string) qualitycli.Deps {
	state := newStateService(dir)
	baseline := app.SNRBaselineStore{CanonicalRepository: outbound.CanonicalRepository, ReadState: state.Read, WriteState: func(key, content string) (statecontract.StateResult, error) {
		return state.Write(context.Background(), key, content)
	}}
	return qualitycli.Deps{Root: root, PrintJSON: printJSON, ReadSNRBaseline: baseline.Read, SaveSNRBaseline: baseline.Save, Inspect: func(target string) contract.InspectResult {
		if target == "" {
			target = root
		}
		if abs, err := filepath.Abs(target); err == nil {
			target = abs
		}
		planning := newSelfWorkflowPlanning(target, dir, version)
		return app.Inspect(target, app.InspectDeps{
			Now: func() string { return time.Now().UTC().Format(time.RFC3339Nano) },
			Coverage: func(repo string) (string, error) {
				return outbound.RunGoTestCoverage(repo, outbound.ExecuteGoTestCoverage, outbound.DefaultCoverageCacheBase)
			},
			BranchFunctions: outbound.CollectBranchFunctions, AuditItems: outbound.CollectAuditItems,
			CodeSNR: outbound.ComputeCodeSNR, PioneerCoverage: outbound.CollectPioneerCoverage,
			SelfAugmentOpenCount: func(string) (int, error) {
				plan := planning.Plan(augmentcontract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
				return len(augmentdomain.CandidateIDsByStatus(plan.Candidates, "open")), nil
			},
			SelfVerifyOpenCount: func(string) (int, error) {
				return len(verifydomain.CandidateIDsByStatus(planning.ExportCandidates().Candidates, "open")), nil
			},
			Candidates: func(string) []catalog.Candidate {
				plan := planning.Plan(augmentcontract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95})
				return app.CandidatesForPlan(plan.Candidates)
			},
		})
	}}
}
