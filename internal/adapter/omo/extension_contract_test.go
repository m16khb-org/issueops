package omo

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type generatedLifecycleContract struct {
	SchemaVersion int                               `json:"schema_version"`
	Events        map[string]generatedLifecycleRule `json:"events"`
	Message       generatedLifecycleMessage         `json:"message"`
	Warning       string                            `json:"warning"`
}

type generatedLifecycleRule struct {
	Subcommand   string `json:"subcommand"`
	AcceptedOnly bool   `json:"accepted_only"`
}

type generatedLifecycleMessage struct {
	CustomType  string `json:"custom_type"`
	Display     bool   `json:"display"`
	TriggerTurn bool   `json:"trigger_turn"`
}

type mockLifecycleExec struct {
	Code   int
	Stdout string
	Err    error
}

type mockLifecycleObservation struct {
	ExecArgv      [][]string
	Notifications []string
	Messages      []generatedLifecycleMessage
	Contents      []string
}

func TestGeneratedLifecycleExtensionRunsMockPiContract(t *testing.T) {
	contract := parseGeneratedLifecycleContract(t, LifecycleExtension("/private/bin/issueops"))
	if contract.SchemaVersion != 1 {
		t.Fatalf("schema version = %d, want 1", contract.SchemaVersion)
	}
	if !reflect.DeepEqual(contract.Events, map[string]generatedLifecycleRule{
		"session_start":   {Subcommand: "session-start"},
		"session_compact": {Subcommand: "post-compact", AcceptedOnly: true},
	}) {
		t.Fatalf("events = %#v", contract.Events)
	}
	if contract.Message != (generatedLifecycleMessage{CustomType: "issueops:project-docs", Display: false, TriggerTurn: false}) {
		t.Fatalf("message contract = %#v", contract.Message)
	}

	tests := []struct {
		name        string
		event       string
		accepted    bool
		exec        mockLifecycleExec
		wantArgv    [][]string
		wantWarn    int
		wantSend    int
		wantContent string
	}{
		{
			name:  "session start invokes session-start and injects hidden context",
			event: "session_start", exec: mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"catalog"}`},
			wantArgv: [][]string{{"hook", "session-start", "--repo", "/repo", "--json"}}, wantSend: 1, wantContent: "catalog",
		},
		{
			name:  "accepted compact invokes post-compact",
			event: "session_compact", accepted: true, exec: mockLifecycleExec{Stdout: `{"should_inject":true,"compact":"restored"}`},
			wantArgv: [][]string{{"hook", "post-compact", "--repo", "/repo", "--json"}}, wantSend: 1, wantContent: "restored",
		},
		{name: "rejected compact invokes nothing", event: "session_compact", accepted: false},
		{name: "malformed hook json warns without sending", event: "session_start", exec: mockLifecycleExec{Stdout: `{`}, wantArgv: [][]string{{"hook", "session-start", "--repo", "/repo", "--json"}}, wantWarn: 1},
		{name: "nonzero hook exit warns without sending", event: "session_start", exec: mockLifecycleExec{Code: 7}, wantArgv: [][]string{{"hook", "session-start", "--repo", "/repo", "--json"}}, wantWarn: 1},
		{name: "hook execution error warns without sending", event: "session_start", exec: mockLifecycleExec{Err: errors.New("exec failed")}, wantArgv: [][]string{{"hook", "session-start", "--repo", "/repo", "--json"}}, wantWarn: 1},
		{name: "no inject sends nothing", event: "session_start", exec: mockLifecycleExec{Stdout: `{"should_inject":false,"compact":"catalog"}`}, wantArgv: [][]string{{"hook", "session-start", "--repo", "/repo", "--json"}}},
		{name: "empty compact sends nothing", event: "session_start", exec: mockLifecycleExec{Stdout: `{"should_inject":true,"compact":""}`}, wantArgv: [][]string{{"hook", "session-start", "--repo", "/repo", "--json"}}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := runGeneratedLifecycleContract(contract, test.event, test.accepted, test.exec)
			if !reflect.DeepEqual(got.ExecArgv, test.wantArgv) {
				t.Fatalf("exec argv = %#v, want %#v", got.ExecArgv, test.wantArgv)
			}
			if len(got.Notifications) != test.wantWarn {
				t.Fatalf("notifications = %#v, want count %d", got.Notifications, test.wantWarn)
			}
			if len(got.Messages) != test.wantSend || len(got.Contents) != test.wantSend {
				t.Fatalf("messages = %#v contents = %#v, want count %d", got.Messages, got.Contents, test.wantSend)
			}
			if test.wantSend == 1 {
				if got.Messages[0] != contract.Message || got.Contents[0] != test.wantContent {
					t.Fatalf("message = %#v content = %q", got.Messages[0], got.Contents[0])
				}
			}
		})
	}
}

func parseGeneratedLifecycleContract(t *testing.T, source string) generatedLifecycleContract {
	t.Helper()
	const prefix = "const issueopsLifecycleContract = "
	start := strings.Index(source, prefix)
	if start < 0 {
		t.Fatal("generated extension has no lifecycle contract")
	}
	remaining := source[start+len(prefix):]
	end := strings.IndexByte(remaining, '\n')
	if end < 0 {
		t.Fatal("generated lifecycle contract is not line-delimited")
	}
	var contract generatedLifecycleContract
	if err := json.Unmarshal([]byte(remaining[:end]), &contract); err != nil {
		t.Fatalf("decode generated lifecycle contract: %v", err)
	}
	return contract
}

func runGeneratedLifecycleContract(contract generatedLifecycleContract, event string, accepted bool, exec mockLifecycleExec) mockLifecycleObservation {
	rule, ok := contract.Events[event]
	if !ok || rule.AcceptedOnly && !accepted {
		return mockLifecycleObservation{}
	}
	observation := mockLifecycleObservation{ExecArgv: [][]string{{"hook", rule.Subcommand, "--repo", "/repo", "--json"}}}
	if exec.Err != nil || exec.Code != 0 {
		observation.Notifications = append(observation.Notifications, contract.Warning)
		return observation
	}
	var payload struct {
		ShouldInject bool   `json:"should_inject"`
		Compact      string `json:"compact"`
	}
	if err := json.Unmarshal([]byte(exec.Stdout), &payload); err != nil {
		observation.Notifications = append(observation.Notifications, contract.Warning)
		return observation
	}
	if payload.ShouldInject && payload.Compact != "" {
		observation.Messages = append(observation.Messages, contract.Message)
		observation.Contents = append(observation.Contents, payload.Compact)
	}
	return observation
}
