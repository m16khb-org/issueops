//go:build unix

package issueops

import (
	"context"
	"os/exec"
	"testing"

	"issueops/internal/adapter/outbound/processlease"
	"issueops/internal/port"
)

type childCreatorContextSpy struct{ ctx context.Context }

func (spy *childCreatorContextSpy) CreateChild(ctx context.Context, _ port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	spy.ctx = ctx
	return port.IssueProviderCreateChildResult{OK: true}, nil
}

// leaseInherited reports whether a command started under ctx would inherit the
// create lifetime lock descriptor.
func leaseInherited(ctx context.Context) bool {
	cmd := exec.Command("true")
	processlease.Attach(ctx, cmd)
	return len(cmd.ExtraFiles) == 1
}

func TestLockedChildCreatorPassesLeaseContext(t *testing.T) {
	type callerKey struct{}
	caller := context.WithValue(t.Context(), callerKey{}, "caller")
	req := port.IssueProviderCreateChildRequest{Repo: t.TempDir(), Title: "child", Confirm: true}

	spy := &childCreatorContextSpy{}
	creator := LockedChildCreator{Inner: spy, StateRoot: t.TempDir()}
	if _, err := creator.CreateChild(caller, req); err != nil {
		t.Fatal(err)
	}
	if spy.ctx.Value(callerKey{}) != "caller" {
		t.Fatal("provider context does not derive from the caller context")
	}
	if !leaseInherited(spy.ctx) {
		t.Fatal("provider context does not carry the create lifetime lock")
	}

	req.Confirm = false
	spy = &childCreatorContextSpy{}
	if _, err := (LockedChildCreator{Inner: spy}).CreateChild(caller, req); err != nil {
		t.Fatal(err)
	}
	if spy.ctx != caller || leaseInherited(spy.ctx) {
		t.Fatal("a dry run must pass the caller context through without taking the lock")
	}
}
