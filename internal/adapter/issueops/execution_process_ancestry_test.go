package issueops

import (
	"os"
	"testing"
)

func TestNativeProcessAncestryFromSnapshotWalksExactParentChain(t *testing.T) {
	t.Parallel()

	snapshot, err := parseNativeProcessSnapshot(`
100 50 Tue Jul 22 09:10:11 2026 /usr/local/bin/issueops
50 1 Tue Jul 22 09:00:00 2026 /Applications/Codex.app/Contents/MacOS/Codex
1 0 Tue Jul 22 08:00:00 2026 /sbin/launchd
`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := nativeProcessAncestryFromSnapshot(snapshot, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("ancestry length = %d, want 3: %+v", len(got), got)
	}
	if got[0].PID != 100 || got[0].Executable != "/usr/local/bin/issueops" || got[0].StartedAt == "" {
		t.Fatalf("child receipt = %+v", got[0])
	}
	if got[1].PID != 50 || got[1].Executable != "/Applications/Codex.app/Contents/MacOS/Codex" {
		t.Fatalf("parent receipt = %+v", got[1])
	}
	if got[2].PID != 1 || got[2].Executable != "/sbin/launchd" {
		t.Fatalf("root receipt = %+v", got[2])
	}
}

func TestQuiescenceProcessOwnershipReusesOneSnapshot(t *testing.T) {
	t.Parallel()

	snapshot, err := parseNativeProcessSnapshot(`
200 100 Tue Jul 22 09:11:00 2026 /usr/bin/child
100 50 Tue Jul 22 09:10:11 2026 /usr/local/bin/issueops
50 1 Tue Jul 22 09:00:00 2026 /Applications/Codex.app/Contents/MacOS/Codex
300 1 Tue Jul 22 09:12:00 2026 /usr/bin/external
1 0 Tue Jul 22 08:00:00 2026 /sbin/launchd
`)
	if err != nil {
		t.Fatal(err)
	}
	ancestry := nativeProcessAncestryPIDsFromSnapshot(snapshot, 100)
	if !ancestry[100] || !ancestry[50] || !ancestry[1] {
		t.Fatalf("requester ancestry = %+v", ancestry)
	}
	observed := replacementProcessSnapshot{entries: snapshot}
	if !observed.HasAncestor(200, map[int]bool{100: true}) || observed.HasAncestor(300, map[int]bool{100: true}) {
		t.Fatal("captured process tree did not distinguish requester child and external process")
	}

}

func TestObserveNativeProcessAncestryIncludesCurrentExactReceipt(t *testing.T) {
	t.Parallel()

	want, err := ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	got, err := ObserveNativeProcessAncestry(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	for _, receipt := range got {
		if receipt == want {
			return
		}
	}
	t.Fatalf("current process receipt %+v not found in ancestry %+v", want, got)
}
