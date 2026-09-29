package issueopsapp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/loopcli"
	"issueops/cmd/issueops/mcpcli"
	loopcontract "issueops/internal/contract/looprun"
)

func TestLoopMCPInstancesKeepSameIDInSeparateStores(t *testing.T) {
	repo := t.TempDir()
	var dependencies [2]mcpcli.MCPDependencies
	var sessions [2]*mcp.ClientSession
	var states [2]string
	for i := range dependencies {
		states[i] = filepath.Join(t.TempDir(), "state")
		t.Setenv("ISSUEOPS_STATE_DIR", states[i])
		dependencies[i] = issueOpsMCPDependencies()
		sessions[i] = startHistoryMCPTestSession(t, dependencies[i])
	}
	wrong := filepath.Join(t.TempDir(), "wrong-state")
	t.Setenv("ISSUEOPS_STATE_DIR", wrong)
	call := func(i int, sdk bool, name string, args map[string]any, out any) bool {
		t.Helper()
		var raw string
		var failed bool
		if sdk {
			result, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
			if err != nil {
				t.Fatal(err)
			}
			failed = result.IsError
			raw = result.Content[0].(*mcp.TextContent).Text
		} else {
			params, err := json.Marshal(map[string]any{"name": name, "arguments": args})
			if err != nil {
				t.Fatal(err)
			}
			result, callErr := mcpcli.HandleToolCallWithDependencies(params, dependencies[i])
			if callErr != nil {
				t.Fatal(callErr)
			}
			value := result.(map[string]any)
			failed, _ = value["isError"].(bool)
			raw = value["content"].([]map[string]any)[0]["text"].(string)
		}
		if err := json.Unmarshal([]byte(raw), out); err != nil {
			t.Fatal(err)
		}
		return failed
	}
	var runs [2]loopcontract.LoopRun
	for i, goal := range []string{"first store goal", "second store goal"} {
		if call(i, i == 1, "loop_start", map[string]any{"repo": repo, "name": "same-id", "goal": goal, "max_attempts": 1}, &runs[i]) {
			t.Fatalf("start %d failed: %+v", i, runs[i])
		}
		if runs[i].Goal != goal {
			t.Fatalf("instance %d goal leaked: %+v", i, runs[i])
		}
	}
	if runs[0].ID != runs[1].ID {
		t.Fatal("fixture must use same identity across stores")
	}
	var group sync.WaitGroup
	for i, verdict := range []string{"pass", "fail"} {
		group.Add(1)
		go func(i int, verdict string) {
			defer group.Done()
			var attempt loopcontract.LoopRun
			if call(i, true, "loop_record_attempt", map[string]any{"id": runs[i].ID, "verdict": verdict, "evidence": []string{"separate state verification"}}, &attempt) {
				t.Errorf("instance %d attempt failed", i)
			}
		}(i, verdict)
	}
	group.Wait()
	var result loopcontract.LoopRun
	if call(0, false, "loop_stop", map[string]any{"id": runs[0].ID, "success": true}, &result) || result.Status != "succeeded" {
		t.Fatalf("first stop: %+v", result)
	}
	for i, want := range []string{"succeeded", "exhausted"} {
		for _, sdk := range []bool{false, true} {
			var status loopcontract.StatusResult
			if call(i, sdk, "loop_status", map[string]any{"id": runs[i].ID}, &status) || status.Loop.Status != want || status.AttemptCount != 1 {
				t.Fatalf("instance %d sdk=%t: %+v", i, sdk, status)
			}
		}
		if _, err := os.Stat(filepath.Join(states[i], "loop", "issueops.db")); err != nil {
			t.Fatal(err)
		}
	}
	var refusal map[string]any
	if !call(1, true, "loop_stop", map[string]any{"id": runs[1].ID, "success": true}, &refusal) {
		t.Fatalf("exhausted loop accepted as success: %v", refusal)
	}
	if _, err := os.Stat(wrong); !os.IsNotExist(err) {
		t.Fatalf("captured instance used changed state path: %v", err)
	}
}

func TestLoopCLIAndReadGateKeepCapturedPaths(t *testing.T) {
	base, state := t.TempDir(), filepath.Join(t.TempDir(), "state")
	t.Chdir(base)
	t.Setenv("ISSUEOPS_STATE_DIR", state)
	dependencies, reader := loopDependencies(), newLoopReader()
	other := filepath.Join(t.TempDir(), "unused")
	t.Setenv("ISSUEOPS_STATE_DIR", other)
	t.Chdir(t.TempDir())
	missing, warnings := reader.RepoGateMissing("project")
	if len(missing) != 0 || len(warnings) != 0 {
		t.Fatalf("empty gate %v %v", missing, warnings)
	}
	for _, path := range []string{state, other} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("read created %s: %v", path, err)
		}
	}
	raw := captureStdoutForContract(t, func() error {
		return loopcli.Run(dependencies, []string{"start", "--repo", "project", "--name", "cli", "--goal", "verify path capture", "--json"})
	})
	var started loopcontract.LoopRun
	if err := json.Unmarshal([]byte(raw), &started); err != nil {
		t.Fatal(err)
	}
	if started.Repo != filepath.Join(base, "project") {
		t.Fatalf("repo drift: %+v", started)
	}
	missing, warnings = reader.RepoGateMissing("project")
	if len(missing) != 1 || missing[0] != "loop_incomplete:"+started.ID || len(warnings) != 0 {
		t.Fatalf("active gate %v %v", missing, warnings)
	}
	id, err := dependencies.ResolveID("project", "cli")
	if err != nil || id != started.ID {
		t.Fatalf("resolve=%s %v", id, err)
	}
	if _, err := dependencies.Stop(id, true, ""); err == nil {
		t.Fatal("stop bypassed pass requirement")
	}
	status, err := dependencies.Status(id)
	if err != nil || status.AttemptCount != 0 || status.Loop.Status != "active" {
		t.Fatalf("refusal modified state: %+v %v", status, err)
	}
	if _, err := os.Stat(other); !os.IsNotExist(err) {
		t.Fatalf("wrote changed state dir: %v", err)
	}
}

func TestLoopInstancesSerializeAttemptsInSharedStore(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	first, second := newLoopService(), newLoopService()
	run, err := first.Start(loopcontract.StartLoopRequest{Repo: t.TempDir(), Name: "parallel", Goal: "preserve all attempts", MaxAttempts: 32})
	if err != nil {
		t.Fatal(err)
	}
	errors := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func(i int) {
			service := first
			if i%2 == 1 {
				service = second
			}
			_, err := service.RecordAttempt(run.ID, loopcontract.RecordAttemptRequest{Verdict: "pass", Evidence: []string{"parallel verification"}})
			errors <- err
		}(i)
	}
	for i := 0; i < 16; i++ {
		if err := <-errors; err != nil {
			t.Fatal(err)
		}
	}
	status, err := first.Status(run.ID)
	if err != nil || status.AttemptCount != 16 {
		t.Fatalf("lost attempts: %+v %v", status, err)
	}
	for i, attempt := range status.Loop.Attempts {
		if attempt.Seq != i+1 {
			t.Fatalf("nonserial sequence: %+v", status.Loop.Attempts)
		}
	}
}
