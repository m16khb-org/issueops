package issueopsapp

import (
	"context"
	"encoding/json"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/selfworkflow/verifycmd"
	catalog "issueops/internal/adapter/inbound/catalog/mcp"
	app "issueops/internal/application/selfverify"
	augmentcontract "issueops/internal/contract/selfaugment"
	verify "issueops/internal/contract/selfverify"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSelfVerifyProductionWiringPreservesCommittedScopeFailure(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init", "-q")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-qm", "base")
	if err := os.MkdirAll(filepath.Join(root, "internal"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "internal", "scope.go"), []byte("package internal\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("-c", "user.name=Test", "-c", "user.email=test@example.com", "commit", "-qm", "Go change")
	fake := t.TempDir()
	// The real scoped collector and runner emit a deliberately oversized failed command log.
	if err := os.WriteFile(filepath.Join(fake, "go"), []byte("#!/bin/sh\nhead -c 20000 /dev/zero | tr '\\000' x\nprintf 'scope-command-tail\\n'\nexit 7\n"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", fake+string(os.PathListSeparator)+os.Getenv("PATH"))
	deps := selfVerifyLoopDeps(root)
	deps.StepDeps.ValidateHarnessInvariants = func(string) verify.StepResult { return verify.StepResult{OK: true} }
	deps.StepDeps.ValidateGoFormat = func(string) verify.StepResult { return verify.StepResult{OK: true} }
	deps.StepDeps.RunCommandStep = func(string, string, time.Duration, string, string, ...string) verify.StepResult {
		return verify.StepResult{OK: true}
	}
	result, err := app.ExecuteLoop(app.LoopRequest{BaseRef: "HEAD~1", BaseSeed: 100, TargetScore: 95}, deps)
	if err == nil || result.OK || len(result.Runs) != 1 || len(result.Runs[0].Steps) != 5 {
		t.Fatalf("scope pipeline result=%+v err=%v", result, err)
	}
	step := result.Runs[0].Steps[4]
	if step.OK || !step.StdoutTruncated || !strings.Contains(step.Command, "go test -race ./... -count=1") || !strings.Contains(step.Stdout, "scope-command-tail") || !strings.Contains(step.Stdout, `"base_sha"`) || !strings.Contains(step.Stdout, `"head_sha"`) {
		t.Fatalf("scope evidence lost: %+v", step)
	}
	encoded, _ := json.Marshal(result)
	if !strings.Contains(string(encoded), "base_sha") || !strings.Contains(string(encoded), "head_sha") {
		t.Fatal("final JSON lost scope")
	}
}

func TestSelfVerifySDKScopeMatchesCLIRequest(t *testing.T) {
	var got []string
	execute := func(request app.LoopRequest) (augmentcontract.SelfAugmentResult, error) {
		got = append(got, request.BaseRef)
		return augmentcontract.SelfAugmentResult{OK: true}, nil
	}
	err := verifycmd.Run([]string{"--base-ref", "HEAD~1", "--llm-eval=false", "--json"}, verifycmd.Deps{Verify: execute, PrintJSON: func(any) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	session := startHistoryMCPTestSession(t, mcpcli.MCPDependencies{Catalog: catalog.Build(), SelfVerify: execute})
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "self_verify", Arguments: map[string]any{"base_ref": "HEAD~1"}})
	if err != nil || result.IsError || len(got) != 2 || got[0] != "HEAD~1" || got[1] != got[0] {
		t.Fatalf("surface scope mismatch: %v %+v %v", got, result, err)
	}
	for _, value := range []any{"", 7, nil} {
		_, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "self_verify", Arguments: map[string]any{"base_ref": value}})
		if err == nil {
			t.Fatalf("SDK accepted invalid scope %#v", value)
		}
	}
	if len(got) != 2 {
		t.Fatal("invalid input reached executor")
	}
}
