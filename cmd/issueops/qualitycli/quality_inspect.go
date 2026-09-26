package qualitycli

import (
	"errors"
	"flag"
	"fmt"
	quality "issueops/internal/domain/quality"
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

var ErrQualityGateBlocked = errors.New("quality gate blocked")

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

func Run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: quality inspect [--repo PATH] [--json]")
	}
	switch args[0] {
	case "inspect":
		return RunInspectWithDeps(args[1:], InspectDeps{})
	case "help", "--help", "-h":
		fmt.Println("issueops quality inspect [--repo PATH] [--json]")
		return nil
	default:
		return fmt.Errorf("unknown quality command %q", args[0])
	}
}

func RunInspectWithDeps(args []string, deps InspectDeps) error {
	fs := flag.NewFlagSet("quality inspect", flag.ContinueOnError)
	repo := fs.String("repo", hostDeps.IssueOpsRoot(), "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	saveBaseline := fs.Bool("save-baseline", false, "persist the current code-SNR as the trend baseline in issueops state")
	trend := fs.Bool("trend", false, "report the code-SNR delta versus the saved baseline")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	deps = deps.withDefaults()
	result := Inspect(*repo, deps)
	snrRatio, snrAvailable := successfulSignalValue(result.Signals, "code-snr")
	baseline, baselinePresent := float64(0), false
	if *trend {
		var baselineErr error
		baseline, baselinePresent, baselineErr = deps.ReadSNRBaseline(*repo)
		if baselineErr != nil {
			addQualityCollectorFailure(&result, "read-baseline: "+baselineErr.Error())
		}
	}
	if *saveBaseline {
		if !snrAvailable {
			addQualityCollectorFailure(&result, "save-baseline: code-snr signal is unavailable")
		} else if err := deps.SaveSNRBaseline(*repo, snrRatio); err != nil {
			addQualityCollectorFailure(&result, "save-baseline: "+err.Error())
		}
	}
	if *trend && snrAvailable && baselinePresent && quality.SNRRegressed(baseline, snrRatio) {
		addSNRRegressionFinding(&result, baseline, snrRatio)
	}
	gateErr := error(nil)
	if result.GateStatus == GateStatusBlock {
		gateErr = fmt.Errorf("%w: collection=%s health=%s", ErrQualityGateBlocked, result.CollectionStatus, result.HealthStatus)
	}
	if *jsonOut {
		if err := hostDeps.PrintJSON(result); err != nil {
			return err
		}
		return gateErr
	}
	fmt.Printf("quality inspect: ok=%v repo=%s candidates=%d warnings=%d\n", result.OK, result.IssueOpsRoot, len(result.Candidates), len(result.Warnings))
	fmt.Printf("quality status: collection=%s health=%s gate=%s\n", result.CollectionStatus, result.HealthStatus, result.GateStatus)
	fmt.Printf("self-augment open: %d\n", result.Summary.SelfAugmentOpenCandidates)
	fmt.Printf("self-verify open: %d\n", result.Summary.SelfVerifyOpenCandidates)
	fmt.Printf("low coverage packages: %d\n", result.Summary.LowCoveragePackages)
	fmt.Printf("branch candidate functions: %d\n", result.Summary.BranchCandidateFunctions)
	fmt.Printf(
		"pioneer coverage: benchmark=%d/%d reproduction=%d/%d\n",
		result.PioneerCoverage.BenchmarkObserved,
		result.PioneerCoverage.Expected,
		result.PioneerCoverage.ReproductionObserved,
		result.PioneerCoverage.Expected,
	)
	fmt.Printf("audit P0/P1/P2 items: %d\n", result.Summary.AuditP1P2Items)
	if *trend {
		if baselinePresent {
			fmt.Printf("code-snr: %.4f (baseline %.4f, Δ %+.4f)\n", snrRatio, baseline, snrRatio-baseline)
		} else {
			fmt.Printf("code-snr: %.4f (no baseline saved; run with --save-baseline)\n", snrRatio)
		}
	} else {
		fmt.Printf("code-snr: %.4f\n", snrRatio)
	}
	for _, warning := range result.Warnings {
		fmt.Printf("warning: %s\n", warning)
	}
	return gateErr
}

func addQualityCollectorFailure(result *InspectResult, warning string) {
	quality.AddQualityCollectorFailure(result, warning)
}

func addSNRRegressionFinding(result *InspectResult, baseline, current float64) {
	quality.AddSNRRegressionFinding(result, baseline, current)
}

func successfulSignalValue(signals []Signal, id string) (float64, bool) {
	return quality.SuccessfulSignalValue(signals, id)
}

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

func collectQualityFindings(warnings []string, lowCoverage []CoveragePackage, branchFunctions []BranchFunction, auditItems []AuditItem, pioneer PioneerCoverage) []Finding {
	return quality.CollectQualityFindings(warnings, lowCoverage, branchFunctions, auditItems, pioneer)
}

func boundedEvidence(evidence []string, limit int) []string {
	return quality.BoundedEvidence(evidence, limit)
}

func qualityStatuses(warnings []string, findings []Finding) (string, string, string) {
	return quality.QualityStatuses(warnings, findings)
}
