package issueops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func claimTokenRecord(t *testing.T) issueops.IssueOpsRecord {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return issueops.IssueOpsRecord{
		ID: "io-token",
		Execution: &issueops.Execution{
			Workspace: issueops.Workspace{Root: root},
			Lease:     issueops.WriteLease{Generation: 3},
		},
	}
}

func TestCreateClaimTokenRefusesToOverwrite(t *testing.T) {
	record := claimTokenRecord(t)
	token, path, err := createClaimToken(record)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if strings.TrimSpace(string(data)) != token {
		t.Fatalf("stored token %q != returned %q", data, token)
	}
	if _, _, err := createClaimToken(record); !os.IsExist(err) {
		t.Fatalf("second create err = %v, want exists", err)
	}
	after, _ := os.ReadFile(path)
	if string(after) != string(data) {
		t.Fatal("existing token file was overwritten")
	}
}

func TestLeaseIndexDeleteMutation(t *testing.T) {
	db, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	actor := issueops.NativeActor{Host: "codex", SessionID: "s1"}
	m, err := leaseIndexDeleteMutation(db, "io-a", actor)
	if err != nil || m != nil {
		t.Fatalf("absent index = %v, %v; want nil, nil", m, err)
	}
	key := leaseHolderIndexKey(actor)
	put := func(data string) {
		t.Helper()
		err := db.Apply(context.Background(), []port.RecordMutation{{Bucket: leaseHolderBucket, ID: key, Data: []byte(data)}})
		if err != nil {
			t.Fatal(err)
		}
	}
	put(`{"lifecycle_id":"io-a"}`)
	m, err = leaseIndexDeleteMutation(db, "io-a", actor)
	if err != nil || m == nil || !m.Delete || m.ID != key || m.Bucket != leaseHolderBucket {
		t.Fatalf("own index = %+v, %v; want delete of %s", m, err, key)
	}
	if m, err := leaseIndexDeleteMutation(db, "io-other", actor); err == nil || m != nil {
		t.Fatalf("foreign lifecycle = %+v, %v; want refusal", m, err)
	}
	put(`{broken`)
	if m, err := leaseIndexDeleteMutation(db, "io-a", actor); err == nil || m != nil {
		t.Fatalf("corrupt index = %+v, %v; want decode error", m, err)
	}
}

func TestSameNativeActorComparisons(t *testing.T) {
	rcpt := &issueops.NativeProcessReceipt{PID: 1, StartedAt: "t", Executable: "x"}
	a := &issueops.NativeActor{Host: "Codex", SessionID: "s", AgentID: "a", SessionProcess: rcpt}
	cases := []struct {
		name string
		b    *issueops.NativeActor
		want bool
	}{
		{"host case-insensitive", &issueops.NativeActor{Host: "codex", SessionID: "s", AgentID: "a", SessionProcess: &issueops.NativeProcessReceipt{PID: 1, StartedAt: "t", Executable: "x"}}, true},
		{"different session", &issueops.NativeActor{Host: "codex", SessionID: "S", AgentID: "a", SessionProcess: rcpt}, false},
		{"different agent", &issueops.NativeActor{Host: "codex", SessionID: "s", SessionProcess: rcpt}, false},
		{"different receipt", &issueops.NativeActor{Host: "codex", SessionID: "s", AgentID: "a", SessionProcess: &issueops.NativeProcessReceipt{PID: 2, StartedAt: "t", Executable: "x"}}, false},
		{"missing receipt", &issueops.NativeActor{Host: "codex", SessionID: "s", AgentID: "a"}, false},
		{"nil actor", nil, false},
	}
	for _, tc := range cases {
		if got := sameNativeActor(a, tc.b); got != tc.want {
			t.Errorf("%s: sameNativeActor = %v, want %v", tc.name, got, tc.want)
		}
		if got := sameNativeActor(tc.b, a); got != tc.want {
			t.Errorf("%s (reversed): sameNativeActor = %v, want %v", tc.name, got, tc.want)
		}
	}
	bare := func() *issueops.NativeActor { return &issueops.NativeActor{Host: "codex", SessionID: "s"} }
	if !sameNativeActor(bare(), bare()) {
		t.Error("actors without a session process must match each other")
	}
	if !sameNativeActor(nil, nil) {
		t.Error("two nil actors should match")
	}
}
