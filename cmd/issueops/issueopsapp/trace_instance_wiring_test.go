package issueopsapp

import (
	traceapp "issueops/internal/application/trace"
	tracecontract "issueops/internal/contract/trace"
	"os"
	"path/filepath"
	"testing"
)

func TestTraceServicesKeepCapturedStateAndFilePriority(t *testing.T) {
	var services [2]traceapp.Service
	for i, step := range []string{"policy check", "daemon probe"} {
		state := t.TempDir()
		t.Setenv("ISSUEOPS_STATE_DIR", state)
		if _, err := newStateService(state).Write("trace-fixture", `{"failed_steps":1,"failed_step":"`+step+`"}`); err != nil {
			t.Fatal(err)
		}
		services[i] = newTraceService()
	}
	ambient := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", ambient)
	for i, step := range []string{"policy check", "daemon probe"} {
		result, err := services[i].Analyze(tracecontract.TraceAnalyzeRequest{Input: "trace-fixture"})
		if err != nil || result.InputSource != "state" || len(result.Findings) != 1 || result.Findings[0].RecurringPattern != step {
			t.Fatalf("state context lost: result=%+v error=%v", result, err)
		}
	}
	file := filepath.Join(t.TempDir(), "trace.json")
	if err := os.WriteFile(file, []byte(`{"guard":{"findings":[{"rule":"file-first"}]}}`), 0600); err != nil {
		t.Fatal(err)
	}
	result, err := services[0].Analyze(tracecontract.TraceAnalyzeRequest{Input: file})
	if err != nil || result.InputSource != "file" || result.Findings[0].FailureClass != "guard_file-first" {
		t.Fatalf("file priority changed: result=%+v err=%v", result, err)
	}
	entries, err := os.ReadDir(ambient)
	if err != nil || len(entries) != 0 {
		t.Fatalf("ambient state touched: %v %v", entries, err)
	}
}
