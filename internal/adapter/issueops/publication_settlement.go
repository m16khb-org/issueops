package issueops

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	publicationcontract "issueops/internal/contract/issueopspublication"
)

// publicationQuietPeriod is twice the provider mutation timeout (60s): a
// create that started this long ago has finished or was abandoned by its
// bounded caller.
const publicationQuietPeriod = 2 * time.Minute

// PublicationCrashSettlement proves that a pending PR/MR create whose caller
// died can no longer land: the create started more than the quiet period ago,
// and the local process table has no provider create (gh pr create / glab mr
// create) for its head branch and no issueops create-pr for its cycle. Any
// observation failure answers false, which keeps the intent.
func PublicationCrashSettlement(now func() time.Time) func(context.Context, publicationcontract.Intent) (bool, error) {
	return func(ctx context.Context, intent publicationcontract.Intent) (bool, error) {
		started, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(intent.PendingStartedAt))
		if err != nil || now().Sub(started) < publicationQuietPeriod {
			return false, err
		}
		cmd := exec.CommandContext(ctx, "ps", "-axww", "-o", "args=")
		cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
		out, err := cmd.Output()
		if err != nil {
			return false, err
		}
		return !publicationCreateInFlight(string(out), intent.Request.HeadBranch, intent.Record.ID), nil
	}
}

// publicationCreateInFlight scans `ps -o args=` rows for a live create that
// could still land: a provider create for head, or issueops create-pr for id.
func publicationCreateInFlight(rows, head, id string) bool {
	head, id = strings.TrimSpace(head), strings.TrimSpace(id)
	for row := range strings.SplitSeq(rows, "\n") {
		fields := strings.Fields(row)
		if len(fields) < 3 {
			continue
		}
		args := " " + strings.Join(fields, " ") + " "
		// Any argv word may be the provider binary (a shim runs
		// `bash /path/gh pr create ...`).
		provider := false
		for _, field := range fields {
			if base := filepath.Base(field); base == "gh" || base == "glab" {
				provider = true
				break
			}
		}
		if provider && head != "" && strings.Contains(args, " "+head+" ") &&
			(strings.Contains(args, " pr create ") || strings.Contains(args, " mr create ")) {
			return true
		}
		if strings.Contains(args, " remote create-pr ") && id != "" && strings.Contains(args, id) {
			return true
		}
	}
	return false
}
