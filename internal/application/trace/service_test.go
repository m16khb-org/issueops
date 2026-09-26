package trace

import (
	"testing"

	tracecontract "issueops/internal/contract/trace"
)

type fakeEffects struct {
	loaded   bool
	analyzed bool
}

func (f *fakeEffects) Load(string) (string, []byte, error) {
	f.loaded = true
	return "state", []byte("{}"), nil
}
func (f *fakeEffects) Analyze([]byte) ([]tracecontract.TraceAnalysisFinding, []string, []string) {
	f.analyzed = true
	return []tracecontract.TraceAnalysisFinding{{FailureClass: "failure"}}, []string{"state"}, nil
}

func TestAnalyzeRejectsMissingInputBeforeEffects(t *testing.T) {
	f := &fakeEffects{}
	result, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: " "})
	if err == nil || result.OK || f.loaded || f.analyzed {
		t.Fatalf("result=%+v effects=%+v err=%v", result, f, err)
	}
}

func TestAnalyzePreservesSourceAndCount(t *testing.T) {
	f := &fakeEffects{}
	result, err := (Service{Effects: f}).Analyze(tracecontract.TraceAnalyzeRequest{Input: " key "})
	if err != nil || !result.OK || result.Input != "key" || result.InputSource != "state" || result.FindingCount != 1 || !f.analyzed {
		t.Fatalf("result=%+v effects=%+v err=%v", result, f, err)
	}
}
