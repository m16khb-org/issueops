package doctor

import (
	"errors"
	"reflect"
	"testing"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	statecontract "issueops/internal/contract/state"
	doctordomain "issueops/internal/domain/doctor"
)

func TestDoctorObservesInOrderAndSkipsLiveProbesForStaticRequests(t *testing.T) {
	for _, static := range []bool{false, true} {
		calls := []string{}
		record := func(name string) { calls = append(calls, name) }
		service := Service{Effects: Effects{
			NormalizeRoot: func(root string) (string, error) { record("normalize"); return root, nil },
			StateDir:      func() string { return "/state" },
			StateDoctor: func() (statecontract.StateDoctorResult, error) {
				record("state")
				return statecontract.StateDoctorResult{Healthy: true, StateDir: "/state"}, nil
			},
			ValidateLifecycle: func(string) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
				record("lifecycle")
				return lifecyclecontract.ProjectLifecycleStatePlan{Exists: true, NamespaceValid: true}, nil
			},
			ProjectDocs: func(string) doctordomain.ProjectDocsObservation {
				record("docs")
				return doctordomain.ProjectDocsObservation{}
			},
			RuntimeState: func(string) doctordomain.RuntimeStateObservation {
				record("runtime")
				return doctordomain.RuntimeStateObservation{}
			},
			LoopContracts: func(string) doctordomain.LoopObservation { record("loop"); return doctordomain.LoopObservation{} },
			PipeCapacity:  func() (int, error) { record("pipe"); return 0, errors.New("unavailable") },
			MCPGateways: func(string) doctordomain.GatewayObservation {
				record("gateway")
				return doctordomain.GatewayObservation{Home: "/home", Endpoints: []doctordomain.GatewayEndpoint{{Name: "local", URL: "http://localhost:9", Error: errors.New("refused")}}}
			},
			NativeIntegrations: func(string) doctordomain.NativeObservation { record("native"); return doctordomain.NativeObservation{} },
			BinaryDrift:        func(string) doctordomain.BinaryObservation { record("binary"); return doctordomain.BinaryObservation{} },
			Now:                func() time.Time { return time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) },
		}}
		result, err := service.Run(HarnessDoctorRequest{RepoRoot: "/repo", StaticOnly: static})
		if err != nil || !result.OK || result.Healthy != static {
			t.Fatalf("static=%t result=%+v err=%v", static, result, err)
		}
		want := []string{"normalize", "state", "lifecycle", "docs", "runtime", "loop"}
		if !static {
			want = append(want, "pipe", "gateway")
		}
		want = append(want, "native", "binary")
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("static=%t calls=%v want %v", static, calls, want)
		}
		if static {
			if len(result.Issues) != 0 {
				t.Fatalf("static issues=%+v", result.Issues)
			}
		} else {
			codes := []string{}
			for _, issue := range result.Issues {
				codes = append(codes, issue.Code)
			}
			if !reflect.DeepEqual(codes, []string{"mcp_gateway_unreachable", "pipe_capacity_unavailable"}) {
				t.Fatalf("live issues=%v", codes)
			}
		}
	}
}
