package issueopsorphancleanup

import (
	"slices"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsorphancleanup"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	corehealth "issueops/internal/domain/operationalhealth"
)

const orphanHead = "0123456789abcdef0123456789abcdef01234567"

func orphanRequest() contract.Request {
	return contract.Request{
		ID: "io-orphan", RepoRoot: "/repo", WorktreePath: "/wt/orphan", Branch: "12-orphan",
		Artifact: issueopscontract.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: "https://example.test/pr/1"},
	}
}

func orphanSnapshot() corehealth.Snapshot {
	return corehealth.Snapshot{
		RepoRoot: "/repo",
		GitWorktrees: []operationalhealthcontract.GitWorktree{
			{Path: "/repo", Branch: "main", Head: orphanHead, Canonical: true},
			{Path: "/wt/orphan", Branch: "12-orphan", Head: orphanHead},
		},
		LocalRefs: []operationalhealthcontract.GitRef{{Branch: "12-orphan", OID: orphanHead, Location: "local"}},
	}
}

func TestNormalizeRequestTrimsAndRequiresIdentity(t *testing.T) {
	request := orphanRequest()
	request.ID, request.Branch = " io-orphan ", " 12-orphan "
	request.Artifact.Provider, request.Artifact.Kind = " GitHub ", " PR "
	got, err := NormalizeRequest(request)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "io-orphan" || got.Branch != "12-orphan" || got.Artifact.Provider != "github" || got.Artifact.Kind != "pr" {
		t.Fatalf("normalized request = %+v", got)
	}
	for name, mutate := range map[string]func(*contract.Request){
		"id is required":     func(r *contract.Request) { r.ID = " " },
		"branch is required": func(r *contract.Request) { r.Branch = "" },
		"artifact URL":       func(r *contract.Request) { r.Artifact.URL = "" },
	} {
		request := orphanRequest()
		mutate(&request)
		if _, err := NormalizeRequest(request); err == nil || !strings.Contains(err.Error(), name) {
			t.Errorf("%s: error = %v", name, err)
		}
	}
}

func TestValidGitOIDAcceptsOnlyFullSHA1OrSHA256Hex(t *testing.T) {
	for value, want := range map[string]bool{
		orphanHead:                  true,
		strings.ToUpper(orphanHead): true,
		" " + orphanHead + " ":      true,
		strings.Repeat("a", 64):     true,
		orphanHead[:39]:             false,
		strings.Repeat("a", 41):     false,
		orphanHead[:39] + "g":       false,
		"":                          false,
	} {
		if got := validGitOID(value); got != want {
			t.Errorf("validGitOID(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestInspectInventoryReadiesARecordlessMergedWorktree(t *testing.T) {
	request := orphanRequest()
	result := ResultForRequest(request)
	InspectInventory(&result, request, orphanSnapshot())
	Finish(&result)
	if !result.Ready || len(result.Missing) != 0 {
		t.Fatalf("clean orphan should be ready, missing=%v", result.Missing)
	}
	if result.RecoveryHead != orphanHead || result.RecoveryPath != "/wt/orphan-recovery-"+orphanHead[:12] {
		t.Fatalf("recovery = %s %s", result.RecoveryHead, result.RecoveryPath)
	}
}

func TestInspectInventoryReportsEachUnmetRequirement(t *testing.T) {
	for want, mutate := range map[string]func(*corehealth.Snapshot){
		"repo_root_match":       func(s *corehealth.Snapshot) { s.RepoRoot = "/other" },
		"branch_match":          func(s *corehealth.Snapshot) { s.GitWorktrees[1].Branch = "other" },
		"canonical_repo_root":   func(s *corehealth.Snapshot) { s.GitWorktrees[0].Canonical = false },
		"target_worktree_count": func(s *corehealth.Snapshot) { s.GitWorktrees = s.GitWorktrees[:1] },
		"worktree_head":         func(s *corehealth.Snapshot) { s.GitWorktrees[1].Head = "abc" },
		"local_branch_head":     func(s *corehealth.Snapshot) { s.LocalRefs[0].OID = strings.Repeat("f", 40) },
		"record_present":        func(s *corehealth.Snapshot) { s.Cycles = []operationalhealthcontract.Cycle{{ID: "io-orphan"}} },
		"target_lease_authority": func(s *corehealth.Snapshot) {
			s.LeaseHolderIndexes = []operationalhealthcontract.LeaseHolderIndex{{LifecycleID: "io-orphan"}}
		},
		"orca_worktree_authority": func(s *corehealth.Snapshot) {
			s.OrcaWorktrees = []operationalhealthcontract.OrcaWorktree{{Path: "/wt/orphan"}}
		},
	} {
		t.Run(want, func(t *testing.T) {
			request, snapshot := orphanRequest(), orphanSnapshot()
			mutate(&snapshot)
			result := ResultForRequest(request)
			InspectInventory(&result, request, snapshot)
			Finish(&result)
			if result.Ready || !slices.Contains(result.Missing, want) {
				t.Fatalf("missing = %v, want %s", result.Missing, want)
			}
		})
	}
}
