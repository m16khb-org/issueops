package issueopsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/contract/mcpservice"
)

type recordingMCPService struct{ calls []string }

func (s *recordingMCPService) reply(action string) (mcpservice.Status, error) {
	s.calls = append(s.calls, action)
	return mcpservice.Status{OK: true, Status: mcpservice.StatusRunning, PID: 77, BuildID: "build-1", URL: "http://127.0.0.1:47831/mcp"}, nil
}

func (s *recordingMCPService) Start(context.Context) (mcpservice.Status, error) {
	return s.reply("start")
}
func (s *recordingMCPService) Stop(context.Context) (mcpservice.Status, error) {
	return s.reply("stop")
}
func (s *recordingMCPService) Status(context.Context) (mcpservice.Status, error) {
	return s.reply("status")
}

func TestMCPServiceCommandCallsInjectedPortAndPrintsSecretFreeDTO(t *testing.T) {
	for _, action := range []string{"start", "stop", "status"} {
		t.Run(action, func(t *testing.T) {
			service := &recordingMCPService{}
			var stdout bytes.Buffer
			if err := (mcpServiceCommand{service: service, stdout: &stdout}).Run([]string{action, "--json"}); err != nil {
				t.Fatal(err)
			}
			var printed map[string]any
			if err := json.Unmarshal(stdout.Bytes(), &printed); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"ok": true, "status": "running", "pid": float64(77), "build_id": "build-1", "url": "http://127.0.0.1:47831/mcp", "error_code": ""}
			if len(service.calls) != 1 || service.calls[0] != action || len(printed) != len(want) {
				t.Fatalf("calls=%v printed=%v", service.calls, printed)
			}
			for key, value := range want {
				if printed[key] != value {
					t.Fatalf("%s = %v, want %v", key, printed[key], value)
				}
			}
		})
	}
	if err := (mcpServiceCommand{service: &recordingMCPService{}, stdout: &bytes.Buffer{}}).Run([]string{"restart"}); err == nil {
		t.Fatal("unknown service action accepted")
	}
}

func TestCLIRecordObserverBindsTraceparentOnceAndKeepsRequestCorrelation(t *testing.T) {
	const cli = "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	slow := issueopsrecord.SpanObservation{Operation: "issueops.update", Outcome: "error"}
	var output bytes.Buffer
	observer := issueOpsCLIRecordObserver(cli, &output)
	observer.Observe(slow)
	request := slow
	request.TraceID, request.ParentID, request.TraceFlags = "11111111111111111111111111111111", "2222222222222222", "00"
	observer.Observe(request)
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], `"trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"`) || !strings.Contains(lines[1], `"trace_id":"11111111111111111111111111111111"`) {
		t.Fatalf("observations:\n%s", output.String())
	}

	output.Reset()
	const private = "private-traceparent-value"
	issueOpsCLIRecordObserver(private, &output).Observe(slow)
	if strings.Contains(output.String(), private) || strings.Contains(output.String(), "trace_id") || !strings.Contains(output.String(), "ignoring invalid TRACEPARENT") {
		t.Fatalf("invalid traceparent output:\n%s", output.String())
	}
}
