package benchmarkcmd

import (
	"flag"
	"fmt"
)

// One handler per `issueops benchmark <subcommand>`. Run (benchmark.go) routes
// through handlers bound to its service, keeping the router low-branch and
// each subcommand independently testable.

func (c Command) runBenchmarkRun(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark run", flag.ContinueOnError)
	fixturesPath := fs.String("fixtures", "", "benchmark fixtures path")
	judge := fs.String("judge", "none", "judge backend: none or file")
	judgeFile := fs.String("judge-file", "", "provenanced judge map JSON path for --judge file ({\"source_run_id\":..,\"provenance\":..,\"scores\":{\"<fixtureID>\":<score>}}); reads stdin when empty")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := c.Service.Run(*fixturesPath, *judge, *judgeFile)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("%s fixtures=%d average=%.2f minimum=%.2f critical_failures=%d\n", result.ID, result.FixtureCount, result.AverageScore, result.MinimumScore, result.CriticalFailureCount)
	return nil
}

func (c Command) runBenchmarkCompare(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark compare", flag.ContinueOnError)
	baselineID := fs.String("baseline", "", "baseline benchmark id")
	candidateID := fs.String("candidate", "", "candidate benchmark id")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := c.Service.Compare(*baselineID, *candidateID)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("improved=%v average_delta=%.2f minimum_delta=%.2f critical_failure_delta=%d\n", result.Improved, result.AverageScoreDelta, result.MinimumScoreDelta, result.CriticalFailureDelta)
	return nil
}

func (c Command) runBenchmarkGate(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark gate", flag.ContinueOnError)
	baselineID := fs.String("baseline", "", "baseline benchmark id")
	candidateID := fs.String("candidate", "", "candidate benchmark id")
	candidateFile := fs.String("candidate-file", "", "IssueOps autoresearch candidate JSON file")
	var changedPaths repeatedFlag
	fs.Var(&changedPaths, "changed-path", "changed path to check against the candidate edit surface; repeatable")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	result, err := c.Service.Gate(*candidateFile, *baselineID, *candidateID, changedPaths)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(result)
	}
	fmt.Printf("keep_candidate=%v ok=%v discard_reasons=%d\n", result.KeepCandidate, result.OK, len(result.DiscardReasons))
	for _, reason := range result.DiscardReasons {
		fmt.Printf("- discard: %s\n", reason)
	}
	return nil
}

func (c Command) runBenchmarkReliability(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark reliability", flag.ContinueOnError)
	outcomesPath := fs.String("outcomes", "", "recorded offline outcomes JSON path ({\"runs\":[{\"run_id\":..,\"provenance\":..,\"outcomes\":{\"<fixtureID>\":<bool>}}]}); reads stdin when empty")
	alpha := fs.Float64("alpha", 0.05, "confidence level alpha for Clopper-Pearson intervals (0<alpha<1)")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	report, err := c.Service.Reliability(*outcomesPath, *alpha)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(report)
	}
	fmt.Printf("runs=%d macro_pass_at_1=%.4f max_k=%d\n", report.Runs, report.MacroPassAt1, report.MaxK)
	for _, point := range report.PassPowKCurve {
		fmt.Printf("- pass^%d=%.4f\n", point.K, point.PassPowK)
	}
	return nil
}

func (c Command) runBenchmarkConsensus(args []string) error {
	fs := flag.NewFlagSet("issueops benchmark consensus", flag.ContinueOnError)
	samplesPath := fs.String("samples", "", "offline-recorded judge samples JSON ({\"samples\":[{\"sample_id\":..,\"provenance\":..,\"score\":<IssueOpsBenchmarkScore>}]}); reads stdin when empty")
	jsonOut := fs.Bool("json", false, "print JSON")
	if help, err := parseFlags(fs, args); help || err != nil {
		return err
	}
	verdict, err := c.Service.Consensus(*samplesPath)
	if err != nil {
		return err
	}
	if *jsonOut {
		return printJSON(verdict)
	}
	fmt.Printf("samples=%d majority_passed=%t pass_agreement=%.4f median=%.4f spread=%.4f variance=%.4f\n",
		verdict.Samples, verdict.MajorityPassed, verdict.PassAgreement, verdict.MedianAverageScore, verdict.ScoreSpread, verdict.SampleVariance)
	fmt.Printf("caveat: %s\n", verdict.Caveat)
	return nil
}
