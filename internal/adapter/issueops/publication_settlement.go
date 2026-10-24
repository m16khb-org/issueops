package issueops

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"issueops/internal/adapter/outbound/processlease"
	publicationcontract "issueops/internal/contract/issueopspublication"
	"issueops/internal/port"
)

// publicationLockDirectory keeps one lifetime lock per repository head. The
// provider create inherits the descriptor, so the lock stays held while
// issueops or an orphaned gh/glab child of a dead issueops is alive.
const publicationLockDirectory = "publication-create-locks"

func publicationLockKey(repo, head string) string {
	return strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(head)
}

// HoldPublicationCreate takes the create lifetime lock for repo/head and
// returns a context whose provider subprocesses inherit it. A held lock means
// another create for the head is running: the call is refused before the
// provider is invoked.
func HoldPublicationCreate(ctx context.Context, stateRoot, repo, head string) (context.Context, func(), error) {
	// The lock does not wait; cancellation stays the provider call's to report.
	lease, err := processlease.Acquire(context.WithoutCancel(ctx), filepath.Join(stateRoot, publicationLockDirectory), publicationLockKey(repo, head))
	if errors.Is(err, processlease.ErrBusy) {
		return ctx, func() {}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("another PR/MR create for head %s is still running", head)}
	}
	if err != nil {
		return ctx, func() {}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("publication create lifetime lock: %w", err)}
	}
	return lease.Context(ctx), func() { _ = lease.Close() }, nil
}

// publicationQuietPeriod is twice the provider mutation timeout (60s): a
// create that started this long ago has finished or was abandoned by its
// bounded caller.
const publicationQuietPeriod = 2 * time.Minute

// PublicationCrashSettlement proves that a pending PR/MR create whose caller
// died can no longer land: the create started more than the quiet period ago,
// the head's create lifetime lock is free (no issueops create and no provider
// child that inherited it is alive), and the local process table has no
// provider create (gh pr create / glab mr create) for the head and no issueops
// create-pr for the cycle (creates started outside the lock). Any observation
// failure answers false, which keeps the intent.
func PublicationCrashSettlement(stateRoot string, now func() time.Time) func(context.Context, publicationcontract.Intent) (bool, error) {
	return func(ctx context.Context, intent publicationcontract.Intent) (bool, error) {
		started, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(intent.PendingStartedAt))
		if err != nil || now().Sub(started) < publicationQuietPeriod {
			return false, err
		}
		lease, err := processlease.Acquire(ctx, filepath.Join(stateRoot, publicationLockDirectory), publicationLockKey(intent.Request.Repo, intent.Request.HeadBranch))
		if err != nil {
			return false, err
		}
		_ = lease.Close()
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
