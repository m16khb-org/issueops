package issueopsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	issueopscore "issueops/internal/adapter/issueops"
	authorityoutbound "issueops/internal/adapter/outbound/authority"
	"issueops/internal/adapter/outbound/sqlstore"
	statestore "issueops/internal/adapter/outbound/state"
	authoritycontract "issueops/internal/contract/authority"
	model "issueops/internal/contract/issueops"
	authoritydomain "issueops/internal/domain/authority"
)

func authorizeForTest(t *testing.T, workspace string, receipt model.NativeProcessReceipt) (authoritycontract.Receipt, string, error) {
	t.Helper()
	var stdout bytes.Buffer
	err := mcpAuthorizeCommand{
		issuer: newAuthorityService(), observe: issueopscore.ObserveNativeProcessAncestry,
		pid: os.Getpid, stdout: &stdout,
	}.Run([]string{
		"--workspace-root", workspace, "--host", "codex", "--session-id", "authorize-test",
		"--session-pid", strconv.Itoa(receipt.PID), "--session-started-at", receipt.StartedAt,
		"--session-executable", receipt.Executable, "--json",
	})
	var printed authoritycontract.Receipt
	if decodeErr := json.Unmarshal(stdout.Bytes(), &printed); decodeErr != nil {
		t.Fatalf("authorize output is not JSON: %v %q", decodeErr, stdout.String())
	}
	return printed, stdout.String(), err
}

func requireAuthorityCode(t *testing.T, err error, code string) {
	t.Helper()
	if authorityErr, ok := errors.AsType[*authoritydomain.Error](err); !ok || authorityErr.Code != code {
		t.Fatalf("err=%v, want %s", err, code)
	}
}

func TestMCPAuthorizeIssuesPreLeaseCapabilityAndGuardRechecksEachSpan(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	workspace := t.TempDir()
	self, err := issueopscore.ObserveNativeProcessReceipt(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	receipt, output, err := authorizeForTest(t, workspace, self)
	if err != nil || !receipt.OK {
		t.Fatalf("authorize receipt=%+v err=%v", receipt, err)
	}
	key, token, err := (authorityoutbound.CredentialFiles{StateDir: statestore.StateDir()}).Read(context.Background(), receipt.AuthorityFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output, token) || strings.Contains(output, strings.SplitN(token, ".", 2)[1]) {
		t.Fatal("authorize printed the raw credential")
	}
	use := authoritycontract.Use{Key: key, Token: token, WorkspaceRoot: workspace}
	ctx, verified, err := bindIssueOpsAuthority(context.Background(), use)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Method != model.VerifiedByCapability || verified.Identity.ProcessAncestry != nil || *verified.Identity.SessionProcess != self {
		t.Fatalf("verified=%+v", verified)
	}

	database, err := sqlstore.Open(issueOpsStateRoot())
	if err != nil {
		t.Fatal(err)
	}
	spans := 0
	for range 2 {
		if err := database.WithSpan(ctx, func(spanCtx context.Context) error {
			spans++
			_, err := issueOpsActorVerifier().Verify(spanCtx, model.NativeActor{})
			return err
		}); err != nil {
			t.Fatalf("guarded span %d: %v", spans, err)
		}
	}
	if spans != 2 {
		t.Fatalf("successive top-level spans=%d", spans)
	}

	rotated, _, err := authorizeForTest(t, workspace, self)
	if err != nil || rotated.AuthorityFile == receipt.AuthorityFile {
		t.Fatalf("reissue receipt=%+v err=%v", rotated, err)
	}
	err = database.WithSpan(ctx, func(context.Context) error {
		spans++
		return nil
	})
	requireAuthorityCode(t, err, authoritycontract.CodeInvalid)
	if spans != 2 {
		t.Fatal("a revoked capability reached the span callback")
	}

	_, newToken, err := (authorityoutbound.CredentialFiles{StateDir: statestore.StateDir()}).Read(context.Background(), rotated.AuthorityFile)
	if err != nil {
		t.Fatal(err)
	}
	fresh, _, err := bindIssueOpsAuthority(context.Background(), authoritycontract.Use{Key: key, Token: newToken, WorkspaceRoot: workspace})
	if err != nil {
		t.Fatal(err)
	}
	foreign, err := sqlstore.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// A foreign root is neither bound nor an authority source: its span passes
	// the guard through, and identity inside it is still checked against the
	// grant root, so the revoked capability stays rejected there.
	foreignSpans := 0
	for _, tc := range []struct {
		ctx     context.Context
		revoked bool
	}{{ctx, true}, {fresh, false}} {
		if err := foreign.WithSpan(tc.ctx, func(spanCtx context.Context) error {
			foreignSpans++
			_, err := issueOpsActorVerifier().Verify(spanCtx, model.NativeActor{})
			if tc.revoked {
				requireAuthorityCode(t, err, authoritycontract.CodeInvalid)
				return nil
			}
			return err
		}); err != nil {
			t.Fatalf("foreign-root span revoked=%v: %v", tc.revoked, err)
		}
	}
	if foreignSpans != 2 {
		t.Fatalf("foreign-root spans=%d, want the guard passed through", foreignSpans)
	}
}

func TestMCPAuthorizeRejectsSessionOutsideObservedAncestry(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	child := exec.Command("cat")
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := child.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = child.Wait()
	})
	foreign, err := issueopscore.ObserveNativeProcessReceipt(child.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	receipt, _, err := authorizeForTest(t, t.TempDir(), foreign)
	if err == nil || receipt.OK || receipt.AuthorityFile != "" || !strings.Contains(err.Error(), "ancestry") {
		t.Fatalf("authorize outside ancestry receipt=%+v err=%v", receipt, err)
	}
	if _, err := os.Stat(statestore.StateDir() + "/mcp-http"); !os.IsNotExist(err) {
		t.Fatalf("rejected authorize wrote credentials: %v", err)
	}
}

func liveFixtureReceipt(t *testing.T) model.NativeProcessReceipt {
	t.Helper()
	receipt, err := issueopscore.ObserveNativeProcessReceipt(1)
	if err != nil {
		t.Fatalf("observe live fixture receipt: %v", err)
	}
	return receipt
}
