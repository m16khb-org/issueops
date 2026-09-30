package qualitycli

import (
	"errors"
	"flag"
	"fmt"
	application "issueops/internal/application/quality"
	contract "issueops/internal/contract/quality"
	quality "issueops/internal/domain/quality"
	"os"
)

var ErrQualityGateBlocked = errors.New("quality gate blocked")

func Run(args []string, deps Deps) error {
	if deps.Output == nil {
		deps.Output = os.Stdout
	}
	if len(args) == 0 {
		return errors.New("usage: quality inspect [--repo PATH] [--json]")
	}
	switch args[0] {
	case "inspect":
		return RunInspect(args[1:], deps)
	case "help", "--help", "-h":
		fmt.Fprintln(deps.Output, "issueops quality inspect [--repo PATH] [--json]")
		return nil
	default:
		return fmt.Errorf("unknown quality command %q", args[0])
	}
}

func RunInspect(args []string, deps Deps) error {
	fs := flag.NewFlagSet("quality inspect", flag.ContinueOnError)
	repo := fs.String("repo", deps.Root, "target repository path")
	jsonOut := fs.Bool("json", false, "print JSON")
	saveBaseline := fs.Bool("save-baseline", false, "persist the current code-SNR as the trend baseline in issueops state")
	trend := fs.Bool("trend", false, "report the code-SNR delta versus the saved baseline")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		*repo = fs.Arg(0)
	}
	if deps.Output == nil {
		deps.Output = os.Stdout
	}
	result, baseline, baselinePresent := application.InspectWithBaseline(application.InspectRequest{Root: *repo, Trend: *trend, SaveBaseline: *saveBaseline}, application.InspectBaselineDeps{Inspect: deps.Inspect, ReadSNRBaseline: deps.ReadSNRBaseline, SaveSNRBaseline: deps.SaveSNRBaseline})
	snrRatio, _ := quality.SuccessfulSignalValue(result.Signals, "code-snr")
	gateErr := error(nil)
	if result.GateStatus == contract.GateStatusBlock {
		gateErr = fmt.Errorf("%w: collection=%s health=%s", ErrQualityGateBlocked, result.CollectionStatus, result.HealthStatus)
	}
	if *jsonOut {
		if err := deps.PrintJSON(result); err != nil {
			return err
		}
		return gateErr
	}
	fmt.Fprintf(deps.Output, "quality inspect: ok=%v repo=%s candidates=%d warnings=%d\n", result.OK, result.IssueOpsRoot, len(result.Candidates), len(result.Warnings))
	fmt.Fprintf(deps.Output, "quality status: collection=%s health=%s gate=%s\n", result.CollectionStatus, result.HealthStatus, result.GateStatus)
	fmt.Fprintf(deps.Output, "self-augment open: %d\n", result.Summary.SelfAugmentOpenCandidates)
	fmt.Fprintf(deps.Output, "self-verify open: %d\n", result.Summary.SelfVerifyOpenCandidates)
	fmt.Fprintf(deps.Output, "low coverage packages: %d\n", result.Summary.LowCoveragePackages)
	fmt.Fprintf(deps.Output, "branch candidate functions: %d\n", result.Summary.BranchCandidateFunctions)
	fmt.Fprintf(deps.Output,
		"pioneer coverage: benchmark=%d/%d reproduction=%d/%d\n",
		result.PioneerCoverage.BenchmarkObserved,
		result.PioneerCoverage.Expected,
		result.PioneerCoverage.ReproductionObserved,
		result.PioneerCoverage.Expected,
	)
	fmt.Fprintf(deps.Output, "audit P0/P1/P2 items: %d\n", result.Summary.AuditP1P2Items)
	if *trend {
		if baselinePresent {
			fmt.Fprintf(deps.Output, "code-snr: %.4f (baseline %.4f, Δ %+.4f)\n", snrRatio, baseline, snrRatio-baseline)
		} else {
			fmt.Fprintf(deps.Output, "code-snr: %.4f (no baseline saved; run with --save-baseline)\n", snrRatio)
		}
	} else {
		fmt.Fprintf(deps.Output, "code-snr: %.4f\n", snrRatio)
	}
	for _, warning := range result.Warnings {
		fmt.Fprintf(deps.Output, "warning: %s\n", warning)
	}
	return gateErr
}
