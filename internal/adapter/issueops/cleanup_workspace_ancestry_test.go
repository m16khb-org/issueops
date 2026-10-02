package issueops

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestBuildCleanupOccupancyAncestryComputedOncePerPID(t *testing.T) {
	for _, n := range []int{100, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			snapshot := map[int]nativeProcessSnapshotEntry{1: cleanupSnapshotEntry(1, 0, "root")}
			procs := []workspaceProcess{{PID: 1, Command: "root"}}
			want := port.CleanupWorkspaceOccupancy{Ancestry: map[int][]int{1: {}, n: {1}}}
			for pid := 2; pid <= n; pid++ {
				snapshot[pid] = cleanupSnapshotEntry(pid, 1, "child")
				if pid%2 == 0 {
					procs = append(procs, workspaceProcess{PID: pid, Command: "child"})
					want.Ancestry[pid] = []int{1}
				}
			}
			for _, proc := range procs {
				entry := snapshot[proc.PID]
				occupant := issueops.CleanupWorkspaceProcess{
					PID: proc.PID, Command: proc.Command,
					StartedAt: entry.Receipt.StartedAt, Executable: entry.Receipt.Executable,
				}
				if proc.PID == 1 {
					occupant.Descendants = n - 1
					occupant.Collateral = n - len(procs)
				}
				want.Occupants = append(want.Occupants, occupant)
			}
			calls := map[int]int{}
			returned := map[int][]int{}
			original := map[int][]int{}
			got, err := buildCleanupOccupancyWithAncestry(procs, snapshot, n, func(pid int) ([]int, error) {
				calls[pid]++
				ancestors, err := cleanupAncestorPIDs(snapshot, pid)
				returned[pid] = ancestors
				original[pid] = slices.Clone(ancestors)
				return ancestors, err
			})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("occupancy = %+v, want %+v", got, want)
			}
			if len(calls) != n {
				t.Fatalf("resolved %d distinct PIDs, want %d", len(calls), n)
			}
			total := 0
			for _, count := range calls {
				total += count
			}
			t.Logf("N=%d: ancestry computations=%d; identical occupancy", n, total)
			for pid, count := range calls {
				if count != 1 {
					t.Fatalf("PID %d computed %d times, want 1", pid, count)
				}
				if !reflect.DeepEqual(returned[pid], original[pid]) {
					t.Fatalf("shared ancestry for PID %d was mutated", pid)
				}
			}
			actual, err := buildCleanupOccupancy(procs, snapshot, n)
			if err != nil || !reflect.DeepEqual(actual, want) {
				t.Fatalf("production occupancy = %+v, err=%v, want %+v", actual, err, want)
			}
		})
	}
}

func TestCleanupAncestryLookupReusesSlicesAndErrors(t *testing.T) {
	failure := errors.New("broken ancestry")
	ancestry := []int{2, 1}
	calls := map[int]int{}
	lookup := memoCleanupAncestry(func(pid int) ([]int, error) {
		calls[pid]++
		if pid == 9 {
			return nil, failure
		}
		return ancestry, nil
	})
	for range 2 {
		got, err := lookup(3)
		if err != nil || !reflect.DeepEqual(got, ancestry) || &got[0] != &ancestry[0] {
			t.Fatalf("successful ancestry must be reused: got=%v err=%v", got, err)
		}
		got, err = lookup(9)
		if got != nil || !errors.Is(err, failure) {
			t.Fatalf("failed ancestry must be reused: got=%v err=%v", got, err)
		}
	}
	if calls[3] != 1 || calls[9] != 1 {
		t.Fatalf("resolver calls = %v, want one per PID including failures", calls)
	}
}

func TestBuildCleanupOccupancyAncestryBrokenChains(t *testing.T) {
	for _, kind := range []string{"missing parent", "cycle", "128 entries", "129 entries"} {
		t.Run(kind, func(t *testing.T) {
			snapshot := map[int]nativeProcessSnapshotEntry{
				1: cleanupSnapshotEntry(1, 0, "root"),
				2: cleanupSnapshotEntry(2, 1, "occupant"),
				3: cleanupSnapshotEntry(3, 2, "requester"),
			}
			switch kind {
			case "missing parent":
				snapshot[10] = cleanupSnapshotEntry(10, 9999, "broken")
			case "cycle":
				snapshot[10] = cleanupSnapshotEntry(10, 11, "broken")
				snapshot[11] = cleanupSnapshotEntry(11, 10, "broken")
			default:
				length := 128
				if kind == "129 entries" {
					length++
				}
				for i := 0; i < length; i++ {
					parent := 0
					if i < length-1 {
						parent = 11 + i
					}
					snapshot[10+i] = cleanupSnapshotEntry(10+i, parent, "chain")
				}
			}
			procs := []workspaceProcess{{PID: 2, Command: "occupant"}}
			got, err := buildCleanupOccupancy(procs, snapshot, 3)
			if err != nil || len(got.Occupants) != 1 || got.Occupants[0].Descendants != 1 || got.Occupants[0].Collateral != 1 {
				t.Fatalf("unrelated broken chain must not fail or inflate counts: occupancy=%+v err=%v", got, err)
			}
			_, chainErr := cleanupAncestorPIDs(snapshot, 10)
			_, requesterErr := buildCleanupOccupancy(procs, snapshot, 10)
			_, occupantErr := buildCleanupOccupancy([]workspaceProcess{{PID: 10}}, snapshot, 3)
			if kind == "128 entries" {
				if chainErr != nil || requesterErr != nil || occupantErr != nil {
					t.Fatalf("128-entry chain rejected: %v / %v / %v", chainErr, requesterErr, occupantErr)
				}
			} else if chainErr == nil || requesterErr == nil || occupantErr == nil ||
				!strings.Contains(requesterErr.Error(), chainErr.Error()) || !strings.Contains(occupantErr.Error(), chainErr.Error()) {
				t.Fatalf("requester/occupant must retain chain failure: %v / %v / %v", chainErr, requesterErr, occupantErr)
			}
		})
	}
}

func TestBuildCleanupOccupancyAncestryRefreshesBetweenInvocations(t *testing.T) {
	snapshot := map[int]nativeProcessSnapshotEntry{
		1: cleanupSnapshotEntry(1, 0, "root"),
		2: cleanupSnapshotEntry(2, 1, "parent"),
		3: cleanupSnapshotEntry(3, 1, "requester"),
	}
	first, err := buildCleanupOccupancy(nil, snapshot, 3)
	if err != nil {
		t.Fatal(err)
	}
	snapshot[3] = cleanupSnapshotEntry(3, 2, "requester")
	second, err := buildCleanupOccupancy(nil, snapshot, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Ancestry[3], []int{1}) || !reflect.DeepEqual(second.Ancestry[3], []int{2, 1}) {
		t.Fatalf("invocations must observe their own snapshot: first=%v second=%v", first.Ancestry, second.Ancestry)
	}
}
