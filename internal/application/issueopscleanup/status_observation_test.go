package issueopscleanup_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type statusEnvironment struct {
	exists          bool
	branch, remotes string
	calls           []string
}

func (e *statusEnvironment) DirectoryExists(path string) bool {
	e.calls = append(e.calls, "dir:"+path)
	return e.exists
}
func (e *statusEnvironment) Git(path string, args ...string) (int, string, string) {
	e.calls = append(e.calls, strings.Join(args, " "))
	return 0, "", ""
}
func (e *statusEnvironment) GitOutput(path string, args ...string) string {
	e.calls = append(e.calls, strings.Join(args, " "))
	if args[0] == "branch" {
		return e.branch
	}
	return e.remotes
}

func TestStructuralStatusObservesOnlyAvailableWorktreeAndRemote(t *testing.T) {
	for _, tc := range []struct {
		name, path, branch, remotes string
		exists                      bool
		calls                       []string
	}{
		{name: "no path"},
		{name: "missing directory", path: " /missing ", calls: []string{"dir:/missing"}},
		{name: "detached", path: "/wt", remotes: "origin", exists: true, calls: []string{"dir:/wt", "status --porcelain=v1", "branch --show-current", "remote"}},
		{name: "no remote", path: "/wt", branch: "topic", exists: true, calls: []string{"dir:/wt", "status --porcelain=v1", "branch --show-current", "remote"}},
		{name: "first remote", path: "/wt", branch: "topic\n", remotes: " mirror\norigin\n", exists: true, calls: []string{"dir:/wt", "status --porcelain=v1", "branch --show-current", "remote", "ls-remote --heads mirror topic"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := &statusEnvironment{exists: tc.exists, branch: tc.branch, remotes: tc.remotes}
			(app.StructuralStatus{Environment: env}).ForRecord(model.IssueOpsRecord{WorktreePath: tc.path}, model.IssueOpsCleanupStatusRequest{})
			if !reflect.DeepEqual(env.calls, tc.calls) {
				t.Fatalf("observations=%v want=%v", env.calls, tc.calls)
			}
		})
	}
}

func TestStatusServiceOnlyNormalizesSameCycleReadinessErrors(t *testing.T) {
	failure := errors.New("preview refused")
	for _, tc := range []struct {
		name, id string
		missing  []string
		wantErr  bool
	}{
		{"readiness", "cycle", []string{"issue_closed"}, false},
		{"foreign cycle", "other", []string{"issue_closed"}, true},
		{"unclassified failure", "cycle", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := model.IssueOpsRecord{ID: "cycle", Repo: "/repo", Phase: model.IssueOpsPhaseDone, IssueURL: "https://github.com/acme/repo/issues/1", RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://github.com/acme/repo/pull/2", Labels: []string{"ready"}, Assignees: []string{"owner"}}}
			var calls []string
			ctx := context.WithValue(context.Background(), struct{}{}, "request")
			service := app.StatusService{
				Records: newCleanupStatusTestStore(record), Structural: app.StructuralStatus{Environment: &statusEnvironment{}},
				Provider: func(name string) (port.IssueProvider, error) {
					calls = append(calls, "provider")
					if name != "github" {
						t.Fatalf("provider=%q", name)
					}
					return nil, nil
				},
				CurrentDirectory: func() (string, error) { calls = append(calls, "cwd"); return "/outside", nil },
				PreviewFinish: func(got context.Context, req model.CleanupFinishRequest, _ port.IssueProvider) (model.CleanupFinishResult, error) {
					calls = append(calls, "preview")
					if got != ctx || req.ID != record.ID || req.CWD != "/outside" || req.Merged || req.CompletionReflected || req.IssueClosed || req.MergedBaseBranch != "" || req.Apply || req.Confirm || req.Fingerprint != "" {
						t.Fatalf("unexpected preview request=%+v", req)
					}
					return model.CleanupFinishResult{ID: tc.id, Missing: tc.missing}, failure
				},
			}
			status, err := service.Status(ctx, "cycle", true)
			if errors.Is(err, failure) != tc.wantErr {
				t.Fatalf("err=%v wantErr=%v", err, tc.wantErr)
			}
			if !reflect.DeepEqual(calls, []string{"provider", "cwd", "preview"}) {
				t.Fatalf("calls=%v", calls)
			}
			if tc.wantErr {
				if status.OK || status.ID != "cycle" {
					t.Fatalf("failed status=%+v", status)
				}
			} else if !status.OK || status.Ready || !status.Merged || !reflect.DeepEqual(status.Missing, tc.missing) || status.RemoteArtifactURL != record.RemoteArtifact.URL {
				t.Fatalf("blocked status=%+v", status)
			}
		})
	}
}
