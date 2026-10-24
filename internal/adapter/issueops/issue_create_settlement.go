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
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

// issueCreateLockDirectory keeps one lifetime lock per repository and sealed
// issue title; the provider create inherits it like a PR create does.
const issueCreateLockDirectory = "issue-create-locks"

func issueCreateLockKey(repo, title string) string {
	return strings.TrimSpace(repo) + "\x00" + strings.TrimSpace(title)
}

// HoldIssueCreate takes the issue create lifetime lock for repo/title; a held
// lock means another create of the same issue is running and the call is
// refused before the provider is invoked.
func HoldIssueCreate(ctx context.Context, stateRoot, repo, title string) (context.Context, func(), error) {
	// The lock does not wait; cancellation stays the provider call's to report.
	lease, err := processlease.Acquire(context.WithoutCancel(ctx), filepath.Join(stateRoot, issueCreateLockDirectory), issueCreateLockKey(repo, title))
	if errors.Is(err, processlease.ErrBusy) {
		return ctx, func() {}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("another create of issue %q is still running", title)}
	}
	if err != nil {
		return ctx, func() {}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("issue create lifetime lock: %w", err)}
	}
	return lease.Context(ctx), func() { _ = lease.Close() }, nil
}

// IssueCreateCrashSettlement proves that a pending issue create whose caller
// died created nothing and can no longer create anything: the intent is older
// than the quiet period, its lifetime lock is free, no gh/glab issue create
// with its title is running, and the provider's newest issues (a listing, not
// the eventually consistent search) carry no issue with its marker. Any
// observation failure or an indeterminate listing answers false.
func IssueCreateCrashSettlement(stateRoot string, resolve func(string) (port.IssueProvider, error), now func() time.Time) func(context.Context, model.IssueOpsRecord) (bool, error) {
	return func(ctx context.Context, record model.IssueOpsRecord) (bool, error) {
		if record.IssueCreateIntent == nil {
			return false, nil
		}
		return settleIssueCreate(ctx, stateRoot, resolve, now, record, *record.IssueCreateIntent, "")
	}
}

// ChildCreateCrashSettlement is the same proof for a pending child create
// (the child carries the same marker and sealed title).
func ChildCreateCrashSettlement(stateRoot string, resolve func(string) (port.IssueProvider, error), now func() time.Time) func(context.Context, model.IssueOpsRecord, model.ChildCreateOperation) (bool, error) {
	return func(ctx context.Context, record model.IssueOpsRecord, op model.ChildCreateOperation) (bool, error) {
		return settleIssueCreate(ctx, stateRoot, resolve, now, record, op.IssueOpsIssueCreateIntent, "task")
	}
}

func settleIssueCreate(ctx context.Context, stateRoot string, resolve func(string) (port.IssueProvider, error), now func() time.Time, record model.IssueOpsRecord, intent model.IssueOpsIssueCreateIntent, issueType string) (bool, error) {
	if intent.Status != model.IssueCreateIntentPending {
		return false, nil
	}
	started, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(firstNonEmptyString(intent.UpdatedAt, intent.StartedAt)))
	if err != nil || now().Sub(started) < publicationQuietPeriod {
		return false, err
	}
	lease, err := processlease.Acquire(ctx, filepath.Join(stateRoot, issueCreateLockDirectory), issueCreateLockKey(record.Repo, intent.Title))
	if err != nil {
		return false, err
	}
	_ = lease.Close()
	cmd := exec.CommandContext(ctx, "ps", "-axww", "-o", "args=")
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	rows, err := cmd.Output()
	if err != nil {
		return false, err
	}
	if issueCreateInFlight(string(rows), intent.OperationID, record.ID) {
		return false, nil
	}
	provider, err := resolve(intent.Provider)
	if err != nil {
		return false, err
	}
	lister, ok := provider.(port.IssueProviderRecentIssueLister)
	if !ok {
		return false, fmt.Errorf("issue provider %q cannot list recent issues", intent.Provider)
	}
	return lister.RecentIssuesLackMarker(ctx, port.IssueProviderRecentIssuesRequest{Repo: record.Repo, ProjectAuthority: intent.ProjectAuthority, Marker: intent.Marker, SinceRFC3339: started.UTC().Format(time.RFC3339Nano), IssueType: issueType})
}

// issueCreateInFlight matches on ASCII identities a C-locale ps keeps intact:
// the operation ID inside the marker that a provider create carries in its
// body argument, or issueops remote create-issue for the cycle ID.
func issueCreateInFlight(rows, operationID, id string) bool {
	operationID, id = strings.TrimSpace(operationID), strings.TrimSpace(id)
	for row := range strings.SplitSeq(rows, "\n") {
		fields := strings.Fields(row)
		provider := false
		for _, field := range fields {
			if base := filepath.Base(field); base == "gh" || base == "glab" {
				provider = true
				break
			}
		}
		args := " " + strings.Join(fields, " ") + " "
		// Any gh/glab call carrying the operation ID (the random value in the
		// create marker) is a create in flight: GitLab children use `glab api
		// graphql ... workItemCreate`, not `issue create`.
		if provider && operationID != "" && strings.Contains(row, operationID) && !providerReadRow(args) {
			return true
		}
		if (strings.Contains(args, " remote create-issue ") || strings.Contains(args, " remote create-child ")) && id != "" &&
			(strings.Contains(args, " "+id+" ") || strings.Contains(args, "="+id+" ")) {
			return true
		}
	}
	return false
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// ChildCreator is the provider capability that creates a child issue.
type ChildCreator interface {
	CreateChild(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error)
}

// LockedChildCreator holds the issue-create lifetime lock for a confirmed
// child create, so a crash settlement can tell a running create from a dead
// one (the provider call has no context, so an orphaned gh child is covered
// by the process scan instead of the inherited lock).
type LockedChildCreator struct {
	Inner     ChildCreator
	StateRoot string
}

func (c LockedChildCreator) CreateChild(req port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	if req.Confirm {
		if c.StateRoot == "" {
			return port.IssueProviderCreateChildResult{OK: false}, &port.IssueProviderCreateError{Invoked: false, Err: fmt.Errorf("child create needs the state root for its lifetime lock")}
		}
		_, release, err := HoldIssueCreate(context.Background(), c.StateRoot, req.Repo, req.Title)
		if err != nil {
			return port.IssueProviderCreateChildResult{OK: false}, err
		}
		defer release()
	}
	return c.Inner.CreateChild(req)
}

// providerReadRow is a reconcile read that carries the marker but writes
// nothing: gh issue list --search, or a glab GraphQL query (not a mutation).
func providerReadRow(args string) bool {
	return strings.Contains(args, " issue list ") || (strings.Contains(args, " api graphql ") && !strings.Contains(args, "mutation"))
}
