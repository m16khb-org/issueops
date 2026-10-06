package issueopscompletion

import (
	"reflect"
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopscompletion"
)

func TestCompletionEvidenceRejectsUnconfirmedOrEmptyEvidence(t *testing.T) {
	for _, tc := range []struct {
		name      string
		confirm   bool
		values    []string
		url, want string
	}{
		{"confirm first", false, nil, "bad", "execution complete requires confirm"},
		{"missing evidence", true, nil, "bad", "execution completion requires verification evidence"},
		{"blank entry", true, []string{"test", "  "}, "bad", "verification entries must be nonempty"},
		{"insecure URL", true, []string{"test"}, "http://example.com/pull/1", "execution completion requires an HTTPS draft PR or MR URL"},
		{"missing path", true, []string{"test"}, "https://example.com", "execution completion requires an HTTPS draft PR or MR URL"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := PrepareEvidence(tc.confirm, tc.values, tc.url)
			if err == nil || err.Error() != tc.want {
				t.Fatalf("error=%v want=%s", err, tc.want)
			}
		})
	}
	values := []string{"  go test ./...  ", "go vet ./..."}
	got, err := PrepareEvidence(true, values, " https://example.com/pull/1 ")
	if err != nil || !reflect.DeepEqual(got, []string{"go test ./...", "go vet ./..."}) {
		t.Fatalf("evidence=%v err=%v", got, err)
	}
	got[0] = "changed"
	if values[0] != "  go test ./...  " {
		t.Fatal("input evidence mutated")
	}
}

func TestCompletionHeadRequiresFullMatchingHash(t *testing.T) {
	for _, tc := range []struct {
		value, head string
		valid       bool
	}{
		{" " + strings.Repeat("A", 40) + " ", strings.Repeat("a", 40), true},
		{strings.Repeat("b", 64), strings.Repeat("b", 64), true},
		{"abcdef0", "abcdef0", false},
		{strings.Repeat("z", 40), strings.Repeat("z", 40), false},
		{strings.Repeat("a", 40), strings.Repeat("b", 40), false},
	} {
		if err := ValidateFinalHead(tc.value, tc.head); (err == nil) != tc.valid {
			t.Fatalf("value=%q err=%v", tc.value, err)
		}
	}
	if err := ValidatePrepared(false); err != contract.ErrExecutionNotPrepared {
		t.Fatalf("prepared error=%v", err)
	}
	if err := ValidatePrepared(true); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePhase("done"); err == nil {
		t.Fatal("done phase accepted as active")
	}
	if err := ValidatePhase("pr"); err != nil {
		t.Fatal(err)
	}
}

func TestTerminalCompletionRetryRequiresReleasedReceiptAndSameEvidence(t *testing.T) {
	command := Command{Generation: 3, FinalHead: strings.Repeat("a", 40), Verification: []string{"test"}, RemoteArtifactURL: "https://example.com/pull/1"}
	base := Snapshot{Phase: "done", Lease: Lease{Generation: 3, Status: "released", ReleasedAt: "then"}, Completion: &Completion{Generation: 3, FinalHead: strings.Repeat("A", 40), CompletedAt: "then", Verification: []string{"test"}, RemoteArtifactURL: command.RemoteArtifactURL}}
	for _, tc := range []struct {
		name    string
		change  func(*Snapshot)
		allowed bool
	}{
		{"same", func(s *Snapshot) {}, true},
		{"zero receipt generation", func(s *Snapshot) { s.Completion.Generation = 0 }, true},
		{"active phase", func(s *Snapshot) { s.Phase = "pr" }, false},
		{"different generation", func(s *Snapshot) { s.Lease.Generation = 4 }, false},
		{"active lease", func(s *Snapshot) { s.Lease.Status = "active" }, false},
		{"holder", func(s *Snapshot) { s.Lease.Holder = &Actor{} }, false},
		{"token", func(s *Snapshot) { s.Lease.ClaimTokenSHA256 = "token" }, false},
		{"missing release", func(s *Snapshot) { s.Lease.ReleasedAt = " " }, false},
		{"missing receipt", func(s *Snapshot) { s.Completion = nil }, false},
		{"missing timestamp", func(s *Snapshot) { s.Completion.CompletedAt = " " }, false},
		{"old receipt", func(s *Snapshot) { s.Completion.Generation = 2 }, false},
		{"different head", func(s *Snapshot) { s.Completion.FinalHead = strings.Repeat("b", 40) }, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := base
			receipt := *base.Completion
			snapshot.Completion = &receipt
			tc.change(&snapshot)
			if got := CanRetryCompletion(snapshot, command); got != tc.allowed {
				t.Fatalf("eligible=%v", got)
			}
		})
	}
	if !MatchesRetryEvidence(*base.Completion, command, true) {
		t.Fatal("identical retry rejected")
	}
	if MatchesRetryEvidence(*base.Completion, command, false) {
		t.Fatal("different report path accepted")
	}
	changed := command
	changed.Verification = []string{"other"}
	if MatchesRetryEvidence(*base.Completion, changed, true) {
		t.Fatal("different verification accepted")
	}
	changed = command
	changed.RemoteArtifactURL = "https://example.com/pull/2"
	if MatchesRetryEvidence(*base.Completion, changed, true) {
		t.Fatal("different artifact accepted")
	}
}
