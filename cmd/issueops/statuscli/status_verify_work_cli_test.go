package statuscli

import (
	"encoding/json"
	"errors"
	guardcontract "issueops/internal/contract/guard"
	policycontract "issueops/internal/contract/policy"
	preflightcontract "issueops/internal/contract/preflight"
	projectdocdomain "issueops/internal/domain/projectdoc"
	"strings"
	"testing"

	verifyworkapp "issueops/internal/application/verifywork"
)

func fakeVerifyWorkService(gitErr error) (verifyworkapp.Service, *guardcontract.GuardCheckRequest) {
	var seen guardcontract.GuardCheckRequest
	return verifyworkapp.Service{
		ResolveTarget: func(repo string) string { return repo },
		GitStatus:     func(string) (string, error) { return "", gitErr },
		Preflight: func(root string) preflightcontract.PreflightResult {
			return preflightcontract.PreflightResult{OK: true, Path: root}
		},
		Guard: func(req guardcontract.GuardCheckRequest) guardcontract.GuardCheckResult {
			seen = req
			return guardcontract.GuardCheckResult{OK: true}
		},
		RunCommand: func(policycontract.CommandPolicyRequest) policycontract.CommandRunResult {
			return policycontract.CommandRunResult{OK: true}
		},
		ProjectSignals: func(string) projectdocdomain.ProjectSignals { return projectdocdomain.ProjectSignals{} },
	}, &seen
}

func TestRunVerifyWorkRejectsUnknownFlag(t *testing.T) {
	service, _ := fakeVerifyWorkService(nil)
	if err := RunVerifyWork(service, []string{"--bogus"}); err == nil {
		t.Fatal("expected flag parse error")
	}
}

func TestRunVerifyWorkReportsIncompleteEvidenceAsErrorWithJSON(t *testing.T) {
	service, _ := fakeVerifyWorkService(errors.New("git broke"))
	var err error
	out := captureStatusVerifyStdout(t, func() error {
		err = RunVerifyWork(service, []string{"--repo", "/tmp/x", "--json"})
		return nil
	})
	if err == nil || err.Error() != "verify-work found incomplete evidence" {
		t.Fatalf("expected incomplete evidence error, got %v", err)
	}
	var decoded map[string]any
	if jsonErr := json.Unmarshal([]byte(out), &decoded); jsonErr != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", jsonErr, out)
	}
	if decoded["kind"] != "verify_work" || decoded["ok"] != false || decoded["repo"] != "/tmp/x" {
		t.Fatalf("unexpected JSON: %v", decoded)
	}
}

func TestRunVerifyWorkTextOutputAndAllFlagSelectsAllFiles(t *testing.T) {
	service, seen := fakeVerifyWorkService(errors.New("git broke"))
	var err error
	out := captureStatusVerifyStdout(t, func() error {
		err = RunVerifyWork(service, []string{"--repo", "/tmp/y", "--all"})
		return nil
	})
	if err == nil {
		t.Fatal("expected error for incomplete evidence")
	}
	if !seen.All || seen.Staged || seen.RepoRoot != "/tmp/y" {
		t.Fatalf("--all not forwarded to guard: %+v", *seen)
	}
	if !strings.HasPrefix(out, "verify-work ok=false repo=/tmp/y\n") || !strings.Contains(out, "- ") {
		t.Fatalf("unexpected text output:\n%s", out)
	}
}

func TestRunVerifyWorkDefaultsToStagedGuard(t *testing.T) {
	service, seen := fakeVerifyWorkService(nil)
	_ = captureStatusVerifyStdout(t, func() error {
		_ = RunVerifyWork(service, []string{"--json", "git", "status"})
		return nil
	})
	if seen.All || !seen.Staged || seen.RepoRoot != "." {
		t.Fatalf("expected staged guard on default repo, got %+v", *seen)
	}
}
