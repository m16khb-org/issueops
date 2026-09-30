package doctor

import (
	"errors"
	doctorcontract "issueops/internal/contract/doctor"
	"strings"
	"testing"
	"time"
)

func TestDiagnosisPreservesHealthBoundariesAndUnknownObservations(t *testing.T) {
	cases := []struct {
		name           string
		observation    Observations
		check          string
		healthy        bool
		issue, summary string
	}{
		{name: "empty pipe error is still unavailable", observation: Observations{Pipe: &PipeObservation{Error: errors.New("")}}, check: "pipe_capacity", healthy: true, issue: "pipe_capacity_unavailable"},
		{name: "empty gateway error is still unreachable", observation: Observations{Gateways: &GatewayObservation{Home: "/home", Endpoints: []GatewayEndpoint{{Name: "local", URL: "http://localhost:9", Error: errors.New("")}}}}, check: "mcp_gateway", issue: "mcp_gateway_unreachable"},
		{name: "pipe below threshold", observation: Observations{Pipe: &PipeObservation{Capacity: 8191}}, check: "pipe_capacity", issue: "pipe_capacity_degraded"},
		{name: "pipe at threshold", observation: Observations{Pipe: &PipeObservation{Capacity: 8192}}, check: "pipe_capacity", healthy: true},
		{name: "pipe unavailable", observation: Observations{Pipe: &PipeObservation{Error: errors.New("closed")}}, check: "pipe_capacity", healthy: true, issue: "pipe_capacity_unavailable", summary: "unavailable: closed"},
		{name: "fd below threshold", observation: Observations{Gateways: &GatewayObservation{Home: "/home", Endpoints: []GatewayEndpoint{{Name: "local", URL: "http://localhost:9"}}, FDs: []GatewayFD{{Port: 9, Count: 511, Available: true}}}}, check: "mcp_gateway", healthy: true, summary: "fd[:9]=511"},
		{name: "fd at threshold", observation: Observations{Gateways: &GatewayObservation{Home: "/home", Endpoints: []GatewayEndpoint{{Name: "local", URL: "http://localhost:9"}}, FDs: []GatewayFD{{Port: 9, Count: 512, Available: true}}}}, check: "mcp_gateway", issue: "mcp_gateway_fd_pressure"},
		{name: "fd unavailable", observation: Observations{Gateways: &GatewayObservation{Home: "/home", Endpoints: []GatewayEndpoint{{Name: "local", URL: "http://localhost:9"}}, FDs: []GatewayFD{{Port: 9}}}}, check: "mcp_gateway", healthy: true, summary: "fd[:9]=unavailable"},
		{name: "gateway unreachable", observation: Observations{Gateways: &GatewayObservation{Home: "/home", Endpoints: []GatewayEndpoint{{Name: "local", URL: "http://localhost:9", Error: errors.New("refused")}}}}, check: "mcp_gateway", issue: "mcp_gateway_unreachable"},
		{name: "loop exhausted", observation: Observations{Loop: LoopObservation{Exhausted: 1}}, check: "loop_contracts", issue: "loop_contracts_incomplete"},
		{name: "loop unreadable before incomplete", observation: Observations{Loop: LoopObservation{Active: 1, Warnings: []string{"read failed"}}}, check: "loop_contracts", issue: "loop_contracts_unreadable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := Evaluate(tc.observation)
			found := false
			for _, check := range result.Checks {
				if check.Name == tc.check {
					found = true
					if check.Healthy != tc.healthy || !strings.Contains(check.Summary, tc.summary) {
						t.Fatalf("check=%+v", check)
					}
				}
			}
			if !found {
				t.Fatalf("missing check %s: %+v", tc.check, result)
			}
			if tc.issue == "" {
				if len(result.Issues) != 0 {
					t.Fatalf("unexpected issues: %+v", result.Issues)
				}
			} else if len(result.Issues) != 1 || result.Issues[0].Code != tc.issue {
				t.Fatalf("issues=%+v want %s", result.Issues, tc.issue)
			}
			if tc.issue != "" && Healthy(result.Checks, result.Issues) {
				t.Fatal("warning diagnosed healthy")
			}
		})
	}
}

func TestDiagnosisDocumentNativeAndBinaryFacts(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	facts := Observations{Root: "/repo's", ProjectDocs: ProjectDocsObservation{Directory: "/repo's/.issueops", Missing: []string{"TESTING.md"}}, RuntimeState: RuntimeStateObservation{Paths: []string{"/repo's/.issueops/state"}, DocumentPath: "/repo's/.issueops/STATE.md", Document: "Shared JSONL Schema"}, Native: NativeObservation{Home: "/home", HooksPath: "/home/.codex/hooks.json", HooksMissing: true}, Binary: BinaryObservation{Root: "/harness", Path: "/harness/bin/issueops", Found: true, BuiltAt: now, LatestSource: now.Add(2 * time.Second)}}
	result := Evaluate(facts)
	want := []string{"project_docs_missing", "repo_local_state_present", "repo_local_state_present", "codex_hooks_missing", "binary_drift"}
	if len(result.Issues) != len(want) {
		t.Fatalf("issues=%+v", result.Issues)
	}
	for i, code := range want {
		if result.Issues[i].Code != code {
			t.Fatalf("issue %d=%+v", i, result.Issues[i])
		}
	}
	if result.Issues[0].Fix.Command != "issueops project bootstrap --repo '/repo'\\''s'" {
		t.Fatalf("unescaped fix: %s", result.Issues[0].Fix.Command)
	}
	facts.Binary.LatestSource = now
	if Evaluate(facts).Issues[len(want)-2].Code != "codex_hooks_missing" {
		t.Fatal("unexpected issue ordering")
	}
}

func TestLifecycleDiagnosisKeepsMissingMismatchAndErrorSeparate(t *testing.T) {
	for _, tc := range []struct {
		exists, valid, failed bool
		code                  string
	}{
		{code: "lifecycle_state_missing"}, {exists: true, code: "lifecycle_namespace_mismatch"}, {exists: true, valid: true}, {failed: true, code: "lifecycle_state_error"},
	} {
		result := EvaluateLifecycle("/repo", doctorcontract.ProjectLifecycleStatePlan{Exists: tc.exists, NamespaceValid: tc.valid}, tc.failed)
		if tc.code == "" {
			if len(result.Issues) != 0 {
				t.Fatal(result)
			}
		} else if len(result.Issues) != 1 || result.Issues[0].Code != tc.code {
			t.Fatalf("result=%+v want %s", result, tc.code)
		}
	}
}
