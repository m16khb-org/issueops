package audit

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestAuditHandoffDeliveryObservationWritesBounded0600JSONL(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	installAuditStateDepsForTest(t)

	observation := auditDeliveryObservationFixture()
	observation.Receipt.Location = "audit/orca/token=secret-value.json"
	record, err := AuditHandoffDeliveryObservation(observation)
	if err != nil {
		t.Fatal(err)
	}
	if !record.OK || record.Kind != "handoff_delivery_observation" || record.SchemaVersion != 1 || record.LogPath != "audit/handoff-delivery.jsonl" || record.RecordDigest == "" {
		t.Fatalf("record=%+v", record)
	}
	logPath := filepath.Join(stateDir, record.LogPath)
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("audit log mode=%#o", info.Mode().Perm())
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "secret-value") || strings.Contains(strings.ToLower(string(data)), "claim_token") {
		t.Fatalf("handoff delivery audit leaked sensitive data: %s", data)
	}
	var decoded HandoffDeliveryAuditRecord
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(data))), &decoded); err != nil {
		t.Fatalf("decode audit record: %v\n%s", err, data)
	}
	if decoded.Observation.AttemptID != observation.AttemptID || decoded.Observation.Receipt.Location != "audit/handoff-delivery.jsonl#audit_log_id="+decoded.AuditLogID || decoded.Observation.Receipt.Digest == "" {
		t.Fatalf("decoded=%+v", decoded)
	}
}

func TestAuditHandoffDeliveryObservationAcceptsUnknownProcessPreCall(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	installAuditStateDepsForTest(t)

	observation := auditDeliveryObservationFixture()
	observation.Target.Process = nil
	observation.Target.ProcessIncarnation = ""
	if _, err := AuditHandoffDeliveryObservation(observation); err != nil {
		t.Fatal(err)
	}
}

func TestAuditHandoffDeliveryObservationAppendsDistinctAttempts(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	installAuditStateDepsForTest(t)

	first := auditDeliveryObservationFixture()
	second := auditDeliveryObservationFixture()
	second.AttemptID = "attempt-2"
	second.Target.PaneID = "pane-2"

	if _, err := AuditHandoffDeliveryObservation(first); err != nil {
		t.Fatal(err)
	}
	if _, err := AuditHandoffDeliveryObservation(second); err != nil {
		t.Fatal(err)
	}
	file, err := os.Open(filepath.Join(stateDir, "audit", "handoff-delivery.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("audit record count=%d", count)
	}
}

func TestReadAndFoldHandoffDeliveryAuditObservations(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	installAuditStateDepsForTest(t)

	first := auditDeliveryObservationFixture()
	second := auditDeliveryObservationFixture()
	second.InputAccepted = issueopscontract.IssueOpsHandoffDeliveryState{
		Status:     issueopscontract.IssueOpsHandoffDeliveryStateObserved,
		ObservedAt: "2026-09-20T10:01:00Z",
		Evidence:   issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt,
	}
	if _, err := AuditHandoffDeliveryObservation(first); err != nil {
		t.Fatal(err)
	}
	if _, err := AuditHandoffDeliveryObservation(second); err != nil {
		t.Fatal(err)
	}

	observations, err := ReadHandoffDeliveryAuditObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 2 {
		t.Fatalf("observations=%d", len(observations))
	}
	folded, decisions, err := FoldHandoffDeliveryAuditObservations()
	if err != nil {
		t.Fatal(err)
	}
	if len(decisions) != 2 || !decisions[0].Accepted || !decisions[1].Accepted {
		t.Fatalf("decisions=%+v", decisions)
	}
	got := folded["io-delivery\x00lineage-1"]
	if got.InputAccepted.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved {
		t.Fatalf("folded observation did not merge accepted input: %+v", got)
	}
}

func TestFoldHandoffDeliveryAuditObservationsPreservesVerifiedPrefixOnTruncatedTail(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	installAuditStateDepsForTest(t)

	if _, err := AuditHandoffDeliveryObservation(auditDeliveryObservationFixture()); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, "audit", "handoff-delivery.jsonl")
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.WriteString(`{"ok":true,"kind":"handoff_delivery_observation"`); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	observations, err := ReadHandoffDeliveryAuditObservations()
	if err == nil || len(observations) != 1 {
		t.Fatalf("observations=%d err=%v", len(observations), err)
	}
	folded, decisions, err := FoldHandoffDeliveryAuditObservations()
	if err == nil || len(folded) != 1 || len(decisions) != 1 || !decisions[0].Accepted {
		t.Fatalf("folded=%d decisions=%+v err=%v", len(folded), decisions, err)
	}
}

func TestFoldHandoffDeliveryAuditCorruptionIsScopedToItsLineage(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	installAuditStateDepsForTest(t)

	current := auditDeliveryObservationFixture()
	unrelated := auditDeliveryObservationFixture()
	unrelated.LifecycleID = "io-unrelated"
	unrelated.LineageID = "lineage-unrelated"
	if _, err := AuditHandoffDeliveryObservation(current); err != nil {
		t.Fatal(err)
	}
	if _, err := AuditHandoffDeliveryObservation(unrelated); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(stateDir, "audit", "handoff-delivery.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	var corrupted HandoffDeliveryAuditRecord
	if err := json.Unmarshal([]byte(lines[1]), &corrupted); err != nil {
		t.Fatal(err)
	}
	corrupted.RecordDigest = strings.Repeat("f", 64)
	encoded, err := json.Marshal(corrupted)
	if err != nil {
		t.Fatal(err)
	}
	lines[1] = string(encoded)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	folded, _, err := FoldHandoffDeliveryAuditObservationsForAt(stateDir, current.LifecycleID, current.LineageID)
	if err != nil || len(folded) != 1 {
		t.Fatalf("unrelated corruption blocked current lineage: folded=%d err=%v", len(folded), err)
	}
	if _, _, err := FoldHandoffDeliveryAuditObservationsForAt(stateDir, unrelated.LifecycleID, unrelated.LineageID); err == nil {
		t.Fatal("current-lineage corruption did not fail closed")
	}
}

func TestAuditHandoffDeliveryObservationDoesNotTouchIssueOpsState(t *testing.T) {
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	installAuditStateDepsForTest(t)

	if _, err := AuditHandoffDeliveryObservation(auditDeliveryObservationFixture()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "audit" {
		t.Fatalf("observation audit touched non-audit state: %+v", entries)
	}
	if _, err := os.Stat(filepath.Join(stateDir, "issueops_v1")); !os.IsNotExist(err) {
		t.Fatalf("observation audit must not create IssueOps claim/status state: %v", err)
	}
}

func TestReadHandoffDeliveryAuditObservationsFailsClosedOnBadJSONL(t *testing.T) {
	tests := []struct {
		name string
		line func() string
	}{
		{
			name: "malformed",
			line: func() string { return "{not-json}\n" },
		},
		{
			name: "unknown observation schema",
			line: func() string {
				record := HandoffDeliveryAuditRecord{
					OK: true, Kind: "handoff_delivery_observation",
					Observation: auditDeliveryObservationFixture(),
				}
				record.Observation.SchemaVersion = 99
				data, err := json.Marshal(record)
				if err != nil {
					panic(err)
				}
				return string(data) + "\n"
			},
		},
		{
			name: "oversized",
			line: func() string { return strings.Repeat("x", 300*1024) + "\n" },
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateDir := t.TempDir()
			t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
			installAuditStateDepsForTest(t)
			path := filepath.Join(stateDir, "audit", "handoff-delivery.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.line()), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadHandoffDeliveryAuditObservations(); err == nil {
				t.Fatal("bad handoff delivery JSONL was accepted")
			}
		})
	}
}

func TestHandoffDeliveryAuditFailsClosedOnUnsafeLogFile(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, path string)
	}{
		{
			name: "symlink",
			setup: func(t *testing.T, path string) {
				t.Helper()
				target := filepath.Join(filepath.Dir(path), "target.jsonl")
				if err := os.WriteFile(target, []byte("{}\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "symlink parent",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Remove(filepath.Dir(path)); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(filepath.Dir(filepath.Dir(path)), "audit-target")
				if err := os.Mkdir(target, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Dir(path)); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "permissive permissions",
			setup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("{}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			stateDir := t.TempDir()
			t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
			installAuditStateDepsForTest(t)
			path := filepath.Join(stateDir, "audit", "handoff-delivery.jsonl")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			test.setup(t, path)
			if _, err := ReadHandoffDeliveryAuditObservations(); err == nil {
				t.Fatal("unsafe handoff delivery audit log was readable")
			}
			if _, err := AuditHandoffDeliveryObservation(auditDeliveryObservationFixture()); err == nil {
				t.Fatal("unsafe handoff delivery audit log was appendable")
			}
		})
	}
}

func auditDeliveryObservationFixture() issueopscontract.IssueOpsHandoffDeliveryObservation {
	return issueopscontract.IssueOpsHandoffDeliveryObservation{
		SchemaVersion:  issueopscontract.IssueOpsHandoffDeliverySchemaVersion,
		AttemptID:      "attempt-1",
		LineageID:      "lineage-1",
		LifecycleID:    "io-delivery",
		PromptSHA256:   strings.Repeat("a", 64),
		MaterialSHA256: strings.Repeat("e", 64),
		Request: issueopscontract.IssueOpsHandoffDeliveryRequest{
			DurableID: "request-1",
		},
		Launcher: issueopscontract.IssueOpsHandoffDeliveryLauncher{
			Name: "orca", Version: "1.4.200", Path: "/usr/local/bin/orca", RuntimeID: "runtime-1", MachineID: "machine-1", ServerID: "server-1",
		},
		Target: issueopscontract.IssueOpsHandoffDeliveryTarget{
			TerminalID: "term-1", PaneID: "pane-1", ProcessIncarnation: "incarnation-1", Process: &issueopscontract.NativeProcessReceipt{
				PID: 8080, StartedAt: "2026-09-20T10:00:00Z", Executable: "/usr/local/bin/codex",
			},
		},
		SourceGeneration: 7,
		CreatedAt:        "2026-09-20T10:00:01Z",
		UpdatedAt:        "2026-09-20T10:05:00Z",
		Receipt: issueopscontract.IssueOpsHandoffDeliveryReceipt{
			Location: "audit/orca/receipt-1.json", Digest: strings.Repeat("d", 64),
		},
		InputAccepted: issueopscontract.IssueOpsHandoffDeliveryState{
			Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: "2026-09-20T10:01:00Z", Evidence: issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt,
		},
		NativeTurnObserved: issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		OwnerClaimed:       issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
		Ambiguous:          issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved},
	}
}

func installAuditStateDepsForTest(t *testing.T) {
	t.Helper()
	oldStateDir, oldWithKeyLock := StateDir, WithKeyLock
	StateDir = func() string { return os.Getenv("ISSUEOPS_STATE_DIR") }
	WithKeyLock = func(ctx context.Context, dir, key string, fn func(context.Context) error) error {
		return fn(ctx)
	}
	t.Cleanup(func() {
		StateDir, WithKeyLock = oldStateDir, oldWithKeyLock
	})
}
