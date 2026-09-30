package remoteverification

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	model "issueops/internal/contract/remoteverification"
)

type readerFixture struct {
	artifact func(context.Context, model.Target) (model.Artifact, error)
	github   func(context.Context, string) error
	gitlab   func(context.Context, model.ChildTarget, bool) (model.TaskMetadata, error)
}

func (r readerFixture) Artifact(ctx context.Context, t model.Target) (model.Artifact, error) {
	return r.artifact(ctx, t)
}
func (r readerFixture) GitHubChild(ctx context.Context, url string) error { return r.github(ctx, url) }
func (r readerFixture) GitLabChild(ctx context.Context, t model.ChildTarget, metadata bool) (model.TaskMetadata, error) {
	return r.gitlab(ctx, t, metadata)
}

func TestVerificationUsesOneReadbackAndPreservesUnknownMerge(t *testing.T) {
	request := issueopscontract.IssueOpsRemoteArtifactVerification{Provider: " GitHub ", Kind: "pull_request", URL: " https://github.com/acme/repo/pull/7 "}
	calls := 0
	unavailable := errors.New("provider unavailable")
	live := model.Artifact{Merged: true, HeadRefName: " feature ", HeadRefOID: " abc123 ", BaseRefName: " main "}
	var readErr error
	service := Service{Reader: readerFixture{artifact: func(_ context.Context, target model.Target) (model.Artifact, error) {
		calls++
		if target != (model.Target{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/7"}) {
			t.Fatalf("target=%+v", target)
		}
		if calls > 1 {
			t.Fatal("merged/head must use exactly one readback")
		}
		return live, readErr
	}}}
	head, err := service.MergedHead(context.Background(), request)
	if err != nil || head != (issueopscontract.CleanupRemoteBranchArtifactHead{HeadRefName: "feature", HeadRefOID: "abc123", BaseRefName: "main"}) || calls != 1 {
		t.Fatalf("head=%+v calls=%d err=%v", head, calls, err)
	}
	calls = 0
	live.Merged = false
	if merged, err := service.ObserveMerged(request); merged || err != nil || calls != 1 {
		t.Fatalf("unmerged=%v calls=%d err=%v", merged, calls, err)
	}
	calls = 0
	readErr = unavailable
	if merged, err := service.ObserveMerged(request); merged || !errors.Is(err, unavailable) || calls != 1 {
		t.Fatalf("unknown=%v calls=%d err=%v", merged, calls, err)
	}
	calls = 0
	readErr = nil
	if _, err := service.MergedHead(context.Background(), request); err == nil || !strings.Contains(err.Error(), "not verified merged") {
		t.Fatalf("unmerged accepted: %v", err)
	}
	calls = 0
	if target, err := service.ObserveTarget(request); err != nil || target != "main" || calls != 1 {
		t.Fatalf("target=%q calls=%d err=%v", target, calls, err)
	}
}

func TestVerificationChecksEvidenceBeforeSuccess(t *testing.T) {
	req := issueopscontract.IssueOpsRemoteArtifactVerificationRequest{Provider: "github", Kind: "issue", URL: "https://github.com/acme/repo/issues/7", Labels: []string{"bug"}, Assignees: []string{"alice"}}
	for _, tc := range []struct {
		name string
		live model.Artifact
		want string
	}{
		{"URL", model.Artifact{URL: "wrong"}, "does not match requested URL"},
		{"labels", model.Artifact{URL: req.URL}, "missing verified label(s): bug"},
		{"assignees", model.Artifact{URL: req.URL, Labels: []string{"bug"}}, "missing verified assignee(s): alice"},
		{"accepted", model.Artifact{URL: req.URL + "/", Labels: []string{" BUG "}, Assignees: []string{" ALICE "}}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			service := Service{Reader: readerFixture{artifact: func(ctx context.Context, target model.Target) (model.Artifact, error) {
				calls++
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("verification must be bounded")
				}
				if target.Kind != "issue" || target.Provider != "github" || target.URL != req.URL {
					t.Fatalf("target=%+v", target)
				}
				return tc.live, nil
			}}}
			err := service.VerifyContext(nil, req) //nolint:staticcheck // Verify the public nil-context normalization contract.
			if calls != 1 || (tc.want == "" && err != nil) || (tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want))) {
				t.Fatalf("calls=%d err=%v want=%q", calls, err, tc.want)
			}
		})
	}
	// Refused routes and canceled requests must perform no remote effects.
	service := Service{Reader: readerFixture{artifact: func(context.Context, model.Target) (model.Artifact, error) {
		t.Fatal("unexpected provider call")
		return model.Artifact{}, nil
	}}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := service.VerifyContext(ctx, req); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled=%v", err)
	}
	req.Provider = "unsupported"
	if err := service.Verify(req); err == nil || !strings.Contains(err.Error(), "unsupported remote artifact verification") {
		t.Fatalf("unsupported=%v", err)
	}
}

func TestChildWorkItemFallbackRequiresTaskEvidence(t *testing.T) {
	unavailable := errors.New("not found")
	for _, tc := range []struct {
		name                string
		firstErr, secondErr error
		metadata            model.TaskMetadata
		want                string
		calls               []string
	}{
		{"native work item", nil, nil, model.TaskMetadata{}, "", []string{"work_items:probe"}},
		{"task fallback", unavailable, nil, model.TaskMetadata{Type: "TASK"}, "", []string{"work_items:probe", "issues:metadata"}},
		{"issue type fallback", unavailable, nil, model.TaskMetadata{IssueType: "Task"}, "", []string{"work_items:probe", "issues:metadata"}},
		{"non task refused", unavailable, nil, model.TaskMetadata{Type: "ISSUE"}, "did not return a Task work item", []string{"work_items:probe", "issues:metadata"}},
		{"empty refused", unavailable, nil, model.TaskMetadata{Empty: true}, "did not return task metadata", []string{"work_items:probe", "issues:metadata"}},
		{"lookup failed", unavailable, unavailable, model.TaskMetadata{}, "not found", []string{"work_items:probe", "issues:metadata"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls []string
			service := Service{Reader: readerFixture{gitlab: func(_ context.Context, target model.ChildTarget, metadata bool) (model.TaskMetadata, error) {
				if target.Provider != "gitlab" || target.Hostname != "gitlab.example" || target.Project != "group/project" || target.IID != "7" {
					t.Fatalf("target=%+v", target)
				}
				kind := "probe"
				if metadata {
					kind = "metadata"
				}
				calls = append(calls, target.Kind+":"+kind)
				if len(calls) == 1 {
					return model.TaskMetadata{}, tc.firstErr
				}
				return tc.metadata, tc.secondErr
			}}}
			err := service.Child("https://gitlab.example/group/project/-/work_items/7")
			if !reflect.DeepEqual(calls, tc.calls) || (tc.want == "" && err != nil) || (tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want))) {
				t.Fatalf("calls=%v err=%v want=%q", calls, err, tc.want)
			}
		})
	}
}
