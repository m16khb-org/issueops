package qualitycli

import (
	"path/filepath"
	"time"

	qualityapp "issueops/internal/application/quality"
	contract "issueops/internal/contract/quality"
	"issueops/internal/domain/qualitycatalog"
)

type InspectDeps struct {
	Now                  func() string
	Coverage             func(root string) (string, error)
	SelfAugmentOpenCount func(root string) (int, error)
	SelfVerifyOpenCount  func(root string) (int, error)
	Candidates           func(root string) []QualityCandidate
	CodeSNR              func(root string) (SNRResult, error)
	PioneerCoverage      func(root string) (PioneerCoverage, error)
	SaveSNRBaseline      func(string, float64) error
	ReadSNRBaseline      func(string) (float64, bool, error)
}

type QualityCandidate = qualitycatalog.Candidate

const (
	CollectionStatusOK    = "ok"
	CollectionStatusError = "error"

	HealthStatusHealthy        = "healthy"
	HealthStatusNeedsAttention = "needs_attention"
	HealthStatusUnknown        = "unknown"

	GateStatusPass       = "pass"
	GateStatusReportOnly = "report_only"
	GateStatusBlock      = "block"
)

type InspectResult = contract.InspectResult
type Summary = contract.Summary
type Signal = contract.Signal
type CoveragePackage = contract.CoveragePackage
type BranchFunction = contract.BranchFunction
type AuditItem = contract.AuditItem
type Finding = contract.Finding
type PioneerCoverage = contract.PioneerCoverage
type PioneerBlockedCase = contract.PioneerBlockedCase

func testCLIDeps(deps InspectDeps) Deps {
	deps = deps.withDefaults()
	return Deps{Root: hostDeps.IssueOpsRoot(), PrintJSON: hostDeps.PrintJSON, Inspect: func(root string) InspectResult { return Inspect(root, deps) }, SaveSNRBaseline: deps.SaveSNRBaseline, ReadSNRBaseline: deps.ReadSNRBaseline}
}
func RunInspectWithDeps(args []string, deps InspectDeps) error {
	return RunInspect(args, testCLIDeps(deps))
}
func runForTest(args []string) error { return Run(args, testCLIDeps(InspectDeps{})) }

func Inspect(root string, deps InspectDeps) InspectResult {
	root = resolveRoot(root)
	deps = deps.withDefaults()
	return qualityapp.Inspect(root, qualityapp.InspectDeps{
		Now:                  deps.Now,
		Coverage:             deps.Coverage,
		BranchFunctions:      collectBranchFunctions,
		AuditItems:           collectAuditItems,
		SelfAugmentOpenCount: deps.SelfAugmentOpenCount,
		SelfVerifyOpenCount:  deps.SelfVerifyOpenCount,
		Candidates:           deps.Candidates,
		CodeSNR:              deps.CodeSNR,
		PioneerCoverage:      deps.PioneerCoverage,
	})
}

func (deps InspectDeps) withDefaults() InspectDeps {
	if deps.Now == nil {
		deps.Now = func() string { return time.Now().UTC().Format(time.RFC3339Nano) }
	}
	if deps.Coverage == nil {
		deps.Coverage = runGoTestCoverage
	}
	if deps.SelfAugmentOpenCount == nil {
		deps.SelfAugmentOpenCount = collectSelfAugmentOpenCount
	}
	if deps.SelfVerifyOpenCount == nil {
		deps.SelfVerifyOpenCount = collectSelfVerifyOpenCount
	}
	if deps.Candidates == nil {
		deps.Candidates = collectQualityCandidates
	}
	if deps.CodeSNR == nil {
		deps.CodeSNR = computeCodeSNR
	}
	if deps.PioneerCoverage == nil {
		deps.PioneerCoverage = collectPioneerCoverage
	}
	if deps.SaveSNRBaseline == nil {
		deps.SaveSNRBaseline = saveSNRBaseline
	}
	if deps.ReadSNRBaseline == nil {
		deps.ReadSNRBaseline = readSNRBaseline
	}
	return deps
}

func resolveRoot(root string) string {
	if root == "" {
		root = hostDeps.IssueOpsRoot()
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return root
	}
	return abs
}

func collectPioneerCoverage(root string) (PioneerCoverage, error) {
	return pioneerCoverageCollector(root)
}
