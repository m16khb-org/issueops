package benchmark

import (
	"fmt"
	benchmarkcontract "issueops/internal/contract/issueopsbenchmark"
	"testing"
)

func TestValidateJudgeProvenanceFailsClosed(t *testing.T) {
	read := func(stateRoot, id string) (benchmarkcontract.IssueOpsBenchmarkRunResult, error) {
		if id == "prior-run" {
			return benchmarkcontract.IssueOpsBenchmarkRunResult{ID: id}, nil
		}
		return benchmarkcontract.IssueOpsBenchmarkRunResult{}, fmt.Errorf("not found")
	}
	const scored = "scored-run"

	for name, judge := range map[string]benchmarkcontract.IssueOpsJudgeMap{
		"missing source_run_id": {Provenance: "recorded judge"},
		"missing provenance":    {SourceRunID: "prior-run"},
		"self-attributed":       {SourceRunID: scored, Provenance: "recorded judge"},
		"unresolvable source":   {SourceRunID: "ghost", Provenance: "recorded judge"},
	} {
		if err := validateJudgeProvenance(judge, scored, "", read); err == nil {
			t.Fatalf("%s must fail closed", name)
		}
	}

	valid := benchmarkcontract.IssueOpsJudgeMap{SourceRunID: "prior-run", Provenance: "recorded fresh-context judge"}
	if err := validateJudgeProvenance(valid, scored, "", read); err != nil {
		t.Fatalf("distinct + resolvable source must pass: %v", err)
	}
}
