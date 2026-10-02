package issueopsnext

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
)

func TestNextExplicitSelectionAvoidsInventory(t *testing.T) {
	for _, selection := range []struct {
		name, id, env, want string
	}{
		{"explicit", " io-a ", "io-b", "io-a"},
		{"environment", "  ", " io-b ", "io-b"},
	} {
		t.Run(selection.name, func(t *testing.T) {
			records := map[string]issueopscontract.IssueOpsRecord{
				"io-a": {ID: "io-a", Repo: "/repo", Phase: issueopscontract.IssueOpsPhaseGrill, Execution: &issueopscontract.Execution{}},
				"io-b": {ID: "io-b", Repo: "/repo", Phase: issueopscontract.IssueOpsPhaseGrill, Execution: &issueopscontract.Execution{}},
			}
			ports := testPorts([]issueopsinventorycontract.ListEntry{{ID: "io-a"}, {ID: "io-b"}}, records)
			lists, reads := 0, 0
			ports.ListCycles = func(context.Context, string, string) (issueopsinventorycontract.ListResult, error) {
				lists++
				return issueopsinventorycontract.ListResult{Entries: []issueopsinventorycontract.ListEntry{{ID: "io-a"}, {ID: "io-b"}}}, nil
			}
			ports.ReadRecord = func(_, id string) (issueopscontract.IssueOpsRecord, error) {
				reads++
				return records[id], nil
			}
			scans := 0
			ports.ReadSelected = func(_ context.Context, root, id string) (issueopscontract.IssueOpsRecord, error) {
				return ports.ReadRecord(root, id)
			}
			ports.ScanRecords = func(context.Context, string, func(issueopscontract.IssueOpsRecord) error) error {
				scans++
				return nil
			}
			ports.Env = func(string) string { return selection.env }

			result, err := NewService(ports).Next(context.Background(), "/state", "/repo", selection.id)

			if err != nil || result.Selected == nil || result.Selected.ID != selection.want {
				t.Fatalf("selection = %+v, err = %v", result, err)
			}
			if lists != 0 || reads != 1 || scans != 0 {
				t.Fatalf("ListCycles=%d selected strict reads=%d sibling scans=%d; want 0, 1, 0", lists, reads, scans)
			}
		})
	}
}

func TestNextSelectedRepositoryFence(t *testing.T) {
	for _, scenario := range []struct {
		name, repo string
		invalid    bool
		wantStage  string
	}{
		{"alias", "/alias", false, "issue"},
		{"foreign", "/foreign", false, "invalid"},
		{"missing", "", true, "invalid"},
		{"corrupt", "", true, "invalid"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			ports := testPorts(nil, nil)
			ports.SourceRoot = func(path string) string {
				if path == "/alias" {
					return "/repo"
				}
				return path
			}
			ports.ReadSelected = func(context.Context, string, string) (issueopscontract.IssueOpsRecord, error) {
				return issueopscontract.IssueOpsRecord{ID: "io-selected", Repo: scenario.repo, Invalid: scenario.invalid, Phase: issueopscontract.IssueOpsPhaseGrill}, nil
			}
			ports.ListCycles = func(context.Context, string, string) (issueopsinventorycontract.ListResult, error) {
				t.Fatal("explicit selection must not list")
				return issueopsinventorycontract.ListResult{}, nil
			}

			result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "io-selected")

			if err != nil || result.Stage.Key != scenario.wantStage || result.Selected.ID != "io-selected" || len(result.Warnings) != 0 {
				t.Fatalf("result = %+v, err = %v", result, err)
			}
			if scenario.wantStage == "invalid" && result.Selected.Branch != "" {
				t.Fatalf("invalid selection leaked record metadata: %+v", result.Selected)
			}
		})
	}
}

func TestNextSelectedPropagatesIOErrors(t *testing.T) {
	failure := errors.New("database unavailable")
	ports := testPorts(nil, nil)
	ports.ReadSelected = func(context.Context, string, string) (issueopscontract.IssueOpsRecord, error) {
		return issueopscontract.IssueOpsRecord{}, failure
	}
	_, err := NewService(ports).Next(context.Background(), "/state", "/repo", "io-selected")
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
}

func TestNextSelectedPropagatesConflictScanErrors(t *testing.T) {
	failure := errors.New("inventory read failed")
	ports := testPorts(nil, nil)
	ports.ReadSelected = func(context.Context, string, string) (issueopscontract.IssueOpsRecord, error) {
		return issueopscontract.IssueOpsRecord{ID: "io-selected", Repo: "/repo", Branch: "topic", Phase: issueopscontract.IssueOpsPhasePlan}, nil
	}
	ports.ScanRecords = func(context.Context, string, func(issueopscontract.IssueOpsRecord) error) error {
		return failure
	}
	_, err := NewService(ports).Next(context.Background(), "/state", "/repo", "io-selected")
	if !errors.Is(err, failure) {
		t.Fatalf("error = %v, want %v", err, failure)
	}
}

func TestNextSelectedStreamsRootConflictInFirstIDOrder(t *testing.T) {
	ports := testPorts(nil, nil)
	ports.CleanPath = filepath.Clean
	ports.SourceRoot = func(path string) string {
		if path == "/alias" {
			return "/repo"
		}
		return path
	}
	ports.ReadSelected = func(context.Context, string, string) (issueopscontract.IssueOpsRecord, error) {
		return issueopscontract.IssueOpsRecord{ID: "io-selected", Repo: "/repo", Branch: "topic/one", Phase: issueopscontract.IssueOpsPhasePlan}, nil
	}
	scans := 0
	ports.ScanRecords = func(_ context.Context, _ string, visit func(issueopscontract.IssueOpsRecord) error) error {
		scans++
		for _, record := range []issueopscontract.IssueOpsRecord{
			{ID: "io-foreign", Repo: "/foreign", Execution: &issueopscontract.Execution{Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/topic-one"}}},
			{ID: "io-first", Repo: "/alias", Execution: &issueopscontract.Execution{Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/extra/../topic-one"}}},
			{ID: "io-last", Repo: "/repo", Execution: &issueopscontract.Execution{Workspace: issueopscontract.Workspace{Root: "/repo.worktrees/topic-one"}}},
		} {
			if err := visit(record); err != nil {
				return err
			}
		}
		return nil
	}

	result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "io-selected")

	if err != nil || scans != 1 || !strings.Contains(strings.Join(result.Warnings, "\n"), "cycle io-first") {
		t.Fatalf("root conflict = %+v, scans = %d, err = %v", result, scans, err)
	}
}

func TestNextAutoSelectionListsAndReadsEnvironmentOnce(t *testing.T) {
	ports := testPorts([]issueopsinventorycontract.ListEntry{{ID: "io-auto"}}, map[string]issueopscontract.IssueOpsRecord{
		"io-auto": {ID: "io-auto", Repo: "/repo", Phase: issueopscontract.IssueOpsPhaseGrill},
	})
	envs, lists := 0, 0
	ports.Env = func(string) string { envs++; return "" }
	list := ports.ListCycles
	ports.ListCycles = func(ctx context.Context, state, repo string) (issueopsinventorycontract.ListResult, error) {
		lists++
		return list(ctx, state, repo)
	}
	result, err := NewService(ports).Next(context.Background(), "/state", "/repo", "")
	if err != nil || result.Selected.ID != "io-auto" || lists != 1 || envs != 1 {
		t.Fatalf("result = %+v, lists = %d, envs = %d, err = %v", result, lists, envs, err)
	}
}
