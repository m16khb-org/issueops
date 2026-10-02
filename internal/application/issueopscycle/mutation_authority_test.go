package issueopscycle

import (
	"context"
	"errors"
	"strings"
	"testing"

	authorityapp "issueops/internal/application/authority"
	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
	authoritydomain "issueops/internal/domain/authority"
	authorityport "issueops/internal/port/authority"
)

func liveVerifier() authorityport.ActorVerifier {
	return authorityapp.New(nil, authorityport.ProcessInspectorFunc(func(_ context.Context, receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
		return "live", receipt, nil
	}), nil, nil, nil, nil)
}

type verifierFunc func(context.Context, model.NativeActor) (model.VerifiedActor, error)

func (f verifierFunc) Verify(ctx context.Context, actor model.NativeActor) (model.VerifiedActor, error) {
	return f(ctx, actor)
}

func TestMutationAuthorityValidatesExecutionBeforePathObservation(t *testing.T) {
	calls := 0
	authority := NewMutationAuthority(func(string, string) bool { calls++; return true }, liveVerifier())
	if err := authority.Validate(context.Background(), model.IssueOpsRecord{}, nil); err != nil || calls != 0 {
		t.Fatalf("planning err=%v paths=%d", err, calls)
	}
	record := model.IssueOpsRecord{Execution: &model.Execution{}}
	if err := authority.Validate(context.Background(), record, nil); err == nil || !strings.Contains(err.Error(), "invalid IssueOps execution v1 record") || calls != 0 {
		t.Fatalf("invalid execution err=%v paths=%d", err, calls)
	}
}

func TestAuthorizeHolderObservesPathsOnlyAfterIdentityMatches(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/host"}
	record := model.IssueOpsRecord{Execution: &model.Execution{Workspace: model.Workspace{Root: "/canonical"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &model.NativeActor{Host: "codex", SessionID: "holder", SessionProcess: &process}}}}
	actor := model.IssueOpsActor{Host: "codex", SessionID: "holder", CWD: "/request", NativeProcessAncestry: []model.NativeProcessReceipt{process}}
	calls := 0
	paths := func(left, right string) bool {
		calls++
		if left != "/request" || right != "/canonical" {
			t.Fatalf("paths=%q %q", left, right)
		}
		return false
	}
	ctx := context.Background()
	if err := AuthorizeHolder(ctx, record, &actor, paths, liveVerifier()); err == nil || calls != 1 {
		t.Fatalf("foreign path error=%v calls=%d", err, calls)
	}
	actor.SessionID = "other"
	if err := AuthorizeHolder(ctx, record, &actor, paths, liveVerifier()); err == nil || calls != 1 {
		t.Fatalf("foreign holder error=%v calls=%d", err, calls)
	}
	actor.SessionID = "holder"
	if err := AuthorizeHolder(ctx, record, &actor, nil, liveVerifier()); err == nil {
		t.Fatal("missing path observer accepted")
	}
	if err := AuthorizeHolder(ctx, record, &actor, func(string, string) bool { return true }, nil); err == nil {
		t.Fatal("missing verifier accepted")
	}
	actor.NativeProcessAncestry = []model.NativeProcessReceipt{{PID: 42, StartedAt: "reused", Executable: "/bin/host"}}
	if err := AuthorizeHolder(ctx, record, &actor, func(string, string) bool { return true }, liveVerifier()); err == nil || !strings.Contains(err.Error(), "write lease holder") {
		t.Fatalf("reused holder pid accepted: %v", err)
	}
}

func TestAuthorizeHolderUsesCapabilityIdentityWithoutAncestry(t *testing.T) {
	process := model.NativeProcessReceipt{PID: 42, StartedAt: "start", Executable: "/bin/host"}
	holder := model.NativeActor{Host: "codex", SessionID: "holder", SessionProcess: &process}
	record := model.IssueOpsRecord{Execution: &model.Execution{Workspace: model.Workspace{Root: "/canonical"}, Lease: model.WriteLease{Generation: 1, Status: model.LeaseStatusActive, Holder: &holder}}}
	actor := model.IssueOpsActor{Host: "codex", SessionID: "holder", CWD: "/canonical"}
	same := func(a, b string) bool { return a == b }
	var seen model.NativeActor
	granted := verifierFunc(func(_ context.Context, input model.NativeActor) (model.VerifiedActor, error) {
		seen = input
		return model.VerifiedActor{Identity: holder, Method: model.VerifiedByCapability}, nil
	})
	if err := AuthorizeHolder(context.Background(), record, &actor, same, granted); err != nil {
		t.Fatalf("capability holder refused: %v", err)
	}
	if seen.SessionProcess == nil || *seen.SessionProcess != process || len(seen.ProcessAncestry) != 0 {
		t.Fatalf("verifier input must carry the holder receipt and no fabricated ancestry: %+v", seen)
	}
	otherProcess := model.NativeProcessReceipt{PID: 77, StartedAt: "start", Executable: "/bin/host"}
	other := verifierFunc(func(context.Context, model.NativeActor) (model.VerifiedActor, error) {
		return model.VerifiedActor{Identity: model.NativeActor{Host: "codex", SessionID: "holder", SessionProcess: &otherProcess}, Method: model.VerifiedByCapability}, nil
	})
	if err := AuthorizeHolder(context.Background(), record, &actor, same, other); err == nil {
		t.Fatal("capability of another session process acted on the holder lease")
	}
	invalid := verifierFunc(func(context.Context, model.NativeActor) (model.VerifiedActor, error) {
		return model.VerifiedActor{}, authoritydomain.Invalid("expired")
	})
	err := AuthorizeHolder(context.Background(), record, &actor, same, invalid)
	if authorityErr, ok := errors.AsType[*authoritydomain.Error](err); !ok || authorityErr.Code != authoritycontract.CodeInvalid {
		t.Fatalf("authority failure must stay typed: %v", err)
	}
}
