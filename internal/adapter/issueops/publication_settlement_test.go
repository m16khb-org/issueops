package issueops

import (
	"context"
	"testing"
	"time"

	publicationcontract "issueops/internal/contract/issueopspublication"
)

// A live provider create for the head, or issueops create-pr for the cycle,
// can still land; other processes and other branches cannot.
func TestPublicationCreateInFlight(t *testing.T) {
	for _, tc := range []struct {
		rows string
		want bool
	}{
		{"/opt/homebrew/bin/gh pr create --head 41-add --base main --title x", true},
		{"glab mr create --source-branch 41-add --target-branch main", true},
		{"bash /home/u/.shims/gh pr create --head 41-add --base main", true},
		{"/usr/local/bin/issueops remote create-pr --id io-1 --head 41-add --confirm", true},
		{"/opt/homebrew/bin/gh pr create --head 42-other --base main", false},
		{"/opt/homebrew/bin/gh pr list --head 41-add", false},
		{"/usr/local/bin/issueops execution reconcile --id io-1 --confirm", false},
		{"vim 41-add.txt", false},
	} {
		if got := publicationCreateInFlight(tc.rows, "41-add", "io-1"); got != tc.want {
			t.Fatalf("%q: got %v", tc.rows, got)
		}
	}
}

// Inside the quiet period nothing is settled, whatever the process table says.
func TestPublicationCrashSettlementWaitsOutTheQuietPeriod(t *testing.T) {
	started := time.Date(2026, 10, 25, 0, 0, 0, 0, time.UTC)
	intent := publicationcontract.Intent{PendingStartedAt: started.Format(time.RFC3339Nano), Request: publicationcontract.ProviderCreateRequest{HeadBranch: "41-unlikely-branch-name"}}
	settled := PublicationCrashSettlement(func() time.Time { return started.Add(time.Minute) })
	if ok, _ := settled(context.Background(), intent); ok {
		t.Fatal("settled inside the quiet period")
	}
	settled = PublicationCrashSettlement(func() time.Time { return started.Add(3 * time.Minute) })
	if ok, err := settled(context.Background(), intent); !ok || err != nil {
		t.Fatalf("after the quiet period with no create alive: ok=%v err=%v", ok, err)
	}
	intent.PendingStartedAt = "garbage"
	if ok, _ := settled(context.Background(), intent); ok {
		t.Fatal("unparseable start must not settle")
	}
}
