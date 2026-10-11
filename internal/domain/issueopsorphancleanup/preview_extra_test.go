package issueopsorphancleanup

import (
	"slices"
	"strings"
	"testing"

	contract "issueops/internal/contract/issueopsorphancleanup"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
)

func TestInspectInventoryClassifiesCycleRecordsOwningTheTarget(t *testing.T) {
	for _, test := range []struct {
		name      string
		cycle     operationalhealthcontract.Cycle
		want      []string
		notWanted []string
		absent    bool
	}{
		{
			name:  "leaseless planning cycle on the target branch is a preserved lifecycle owner",
			cycle: operationalhealthcontract.Cycle{ID: "io-other", Repo: "/repo", Branch: "12-orphan", Phase: "plan"},
			want:  []string{"target_record_present", "target_lifecycle_owner"},
		},
		{
			name:  "cycle with unsupported phase on the target worktree has unknown authority",
			cycle: operationalhealthcontract.Cycle{ID: "io-other", Repo: "/repo", Branch: "x", Phase: "bogus", WorktreePath: "/wt/orphan"},
			want:  []string{"target_record_present", "target_lifecycle_authority_unknown"},
		},
		{
			name:      "same branch name in another repo does not own the target",
			cycle:     operationalhealthcontract.Cycle{ID: "io-other", Repo: "/elsewhere", Branch: "12-orphan", Phase: "plan"},
			notWanted: []string{"target_record_present", "target_lifecycle_owner"},
			absent:    true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, snapshot := orphanRequest(), orphanSnapshot()
			snapshot.Cycles = []operationalhealthcontract.Cycle{test.cycle}
			result := ResultForRequest(request)
			InspectInventory(&result, request, snapshot)
			Finish(&result)
			for _, code := range test.want {
				if !slices.Contains(result.Missing, code) {
					t.Errorf("missing = %v, want %s", result.Missing, code)
				}
			}
			for _, code := range test.notWanted {
				if slices.Contains(result.Missing, code) {
					t.Errorf("missing = %v, must not contain %s", result.Missing, code)
				}
			}
			if result.RecordAbsent != test.absent {
				t.Errorf("RecordAbsent = %v, want %v", result.RecordAbsent, test.absent)
			}
			if len(test.want) > 0 && result.Ready {
				t.Error("an owning record must block readiness")
			}
		})
	}
}

func TestInspectInventoryRejectsCanonicalTargetAndIncompleteInventory(t *testing.T) {
	request, snapshot := orphanRequest(), orphanSnapshot()
	request.WorktreePath = "/repo"
	snapshot.InventoryProblems = []operationalhealthcontract.InventoryProblem{{Source: "orca", Code: "unreadable"}}
	result := ResultForRequest(request)
	InspectInventory(&result, request, snapshot)
	Finish(&result)
	for _, code := range []string{"canonical_worktree", "inventory_complete"} {
		if !slices.Contains(result.Missing, code) {
			t.Errorf("missing = %v, want %s", result.Missing, code)
		}
	}
	if !result.TargetCanonical {
		t.Error("TargetCanonical must record that the target is the canonical worktree")
	}
}

func TestInspectInventoryRequiresExactlyOneLocalBranchRef(t *testing.T) {
	request, snapshot := orphanRequest(), orphanSnapshot()
	snapshot.LocalRefs = append(snapshot.LocalRefs,
		operationalhealthcontract.GitRef{Branch: "12-orphan", OID: orphanHead, Location: "local"})
	result := ResultForRequest(request)
	InspectInventory(&result, request, snapshot)
	Finish(&result)
	if !slices.Contains(result.Missing, "local_branch_head") {
		t.Fatalf("duplicate local refs must be reported, missing=%v", result.Missing)
	}
}

func TestLocalPreviewReportsFilesystemObservations(t *testing.T) {
	request, snapshot := orphanRequest(), orphanSnapshot()
	full := LocalObservation{PathsObservable: true, CleanObservable: true, Clean: true, StateOutsideTarget: true}

	got := LocalPreview(request, snapshot, full)
	Finish(&got)
	if !got.Ready || !got.InventoryRefreshed || !got.TargetClean {
		t.Fatalf("fully observed clean orphan should be ready: %+v", got)
	}

	for want, mutate := range map[string]func(*LocalObservation){
		"inventory_complete":         func(o *LocalObservation) { o.PathsObservable = false },
		"worktree_git_status":        func(o *LocalObservation) { o.CleanObservable = false },
		"worktree_clean":             func(o *LocalObservation) { o.Clean = false },
		"state_store_outside_target": func(o *LocalObservation) { o.StateOutsideTarget = false },
	} {
		observed := full
		mutate(&observed)
		result := LocalPreview(request, snapshot, observed)
		Finish(&result)
		if result.Ready || !slices.Contains(result.Missing, want) {
			t.Errorf("%s: ready=%v missing=%v", want, result.Ready, result.Missing)
		}
	}

	unobservable := LocalPreview(request, snapshot, LocalObservation{PathsObservable: true, Clean: true, StateOutsideTarget: true})
	if unobservable.TargetClean {
		t.Error("clean flag must be ignored when cleanliness was not observable")
	}
}

func TestMergeEvidenceRecordsProviderReadback(t *testing.T) {
	var result contract.Result
	MergeEvidence(&result, true)
	if !result.RemoteMerged || len(result.Missing) != 0 || len(result.Warnings) != 0 {
		t.Fatalf("verified merge: %+v", result)
	}
	MergeEvidence(&result, false)
	if result.RemoteMerged || !slices.Contains(result.Missing, "remote_artifact_merged") || len(result.Warnings) != 1 {
		t.Fatalf("unverified merge must block with a warning: %+v", result)
	}
}

func TestUniqueSortedTrimsDedupesAndDropsBlanks(t *testing.T) {
	got := uniqueSorted([]string{" b ", "a", "", "  ", "b"})
	if !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("uniqueSorted = %q", got)
	}
}

func TestInspectInventoryIgnoresRemoteRefsOfTheTargetBranch(t *testing.T) {
	request, snapshot := orphanRequest(), orphanSnapshot()
	snapshot.RemoteRefs = []operationalhealthcontract.GitRef{{Branch: "12-orphan", OID: strings.Repeat("f", 40), Location: "remote"}}
	snapshot.LocalRefs = append(snapshot.LocalRefs, operationalhealthcontract.GitRef{Branch: "12-orphan", OID: strings.Repeat("e", 40), Location: "remote"})
	result := ResultForRequest(request)
	InspectInventory(&result, request, snapshot)
	Finish(&result)
	if slices.Contains(result.Missing, "local_branch_head") {
		t.Fatalf("remote-located refs must not count as local branch refs: %v", result.Missing)
	}
}
