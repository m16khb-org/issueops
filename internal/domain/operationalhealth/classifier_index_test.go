package operationalhealth

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func indexedHealthSnapshot(n int) Snapshot {
	s := healthyDirectSnapshot()
	s.Cycles, s.LeaseHolderIndexes = nil, nil
	s.GitWorktrees = s.GitWorktrees[:1]
	s.OrcaObserved, s.OrcaRuntimeID, s.OrcaRepoID = true, "runtime", "repo-id"
	s.OrcaWorktrees = []OrcaWorktree{{RuntimeID: "runtime", RepoID: "repo-id", ID: "main", Repo: "/repo", Path: "/repo", Branch: "main", Head: s.SourceHead}}
	s.Messages.RuntimeID = "runtime"
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("resource-%d", i)
		c := healthyDirectSnapshot().Cycles[0]
		c.ID, c.Branch, c.WorktreePath, c.HolderSessionID = id, id, "/worktrees/"+id, id
		c.ExecutionMode, c.OrcaRuntimeID, c.OrcaRepoID, c.OrcaOwnerHost = "orca", "runtime", "repo-id", "codex"
		c.OrcaWorktreeID, c.OrcaWorktreeInstanceID, c.TerminalPTYID = id, id, id
		c.RunID, c.TaskID, c.DispatchID = "run", id, id
		s.Cycles = append(s.Cycles, c)
		s.LeaseHolderIndexes = append(s.LeaseHolderIndexes, LeaseHolderIndex{Key: id, LifecycleID: id, Generation: c.Generation, Host: c.HolderHost, SessionID: id})
		s.GitWorktrees = append(s.GitWorktrees, GitWorktree{Path: c.WorktreePath, Branch: id, Head: s.SourceHead, Clean: true})
		s.OrcaWorktrees = append(s.OrcaWorktrees, OrcaWorktree{RuntimeID: "runtime", RepoID: "repo-id", ID: id, InstanceID: id, Repo: "/repo", Path: c.WorktreePath, Branch: id, Head: s.SourceHead})
		s.Terminals = append(s.Terminals, OrcaTerminal{RuntimeID: "runtime", Handle: id, PTYID: id, WorktreeID: id, WorktreePath: c.WorktreePath, Connected: true, Writable: true})
		s.Tasks = append(s.Tasks, OrcaTask{RuntimeID: "runtime", RunID: "run", ID: id, Status: "dispatched", DispatchID: id})
		s.Dispatches = append(s.Dispatches, OrcaDispatch{RuntimeID: "runtime", RunID: "run", ID: id, TaskID: id, AssigneeHandle: id, Status: "dispatched"})
	}
	return s
}

func TestClassifyIndexedFindingsBaseline(t *testing.T) {
	// Digests cover every field and the order of the complete Findings slice.
	tests := []struct {
		name   string
		mutate func(*Snapshot)
		want   string
	}{
		{"healthy", func(s *Snapshot) {}, "74234e98afe7498fb5daf1f36ac2d78acc339464f950703b8c019892f982b90b"},
		{"duplicates", func(s *Snapshot) {
			s.Cycles = append(s.Cycles, s.Cycles[0])
			s.GitWorktrees = append(s.GitWorktrees, s.GitWorktrees[1])
			s.OrcaWorktrees = append(s.OrcaWorktrees, s.OrcaWorktrees[1])
			s.Terminals = append(s.Terminals, s.Terminals[0])
			s.Tasks = append(s.Tasks, s.Tasks[0])
			s.Dispatches = append(s.Dispatches, s.Dispatches[0])
			s.LeaseHolderIndexes = append(s.LeaseHolderIndexes, s.LeaseHolderIndexes[0])
			s.Gates = []OrcaGate{{RuntimeID: "runtime", ID: "gate", Status: "pending"}, {RuntimeID: "runtime", ID: "gate", Status: "pending"}}
		}, "f584dfaa9efeaa0447da99217c853df3b99444fd521bc3805c76d4bb1e448138"},
		{"missing", func(s *Snapshot) {
			s.GitWorktrees = s.GitWorktrees[:1]
			s.OrcaWorktrees = s.OrcaWorktrees[:1]
			s.Terminals, s.Tasks, s.Dispatches, s.LeaseHolderIndexes = nil, nil, nil, nil
		}, "ca7ef8a84fa41eb81b3eb2dd9513384bb4e5be18c607e63bbff0da5915f4da8e"},
		{"mismatch", func(s *Snapshot) {
			s.OrcaWorktrees[1].InstanceID = "other"
			s.Terminals[0].WorktreeID = "other"
			s.Tasks[0].DispatchID = "other"
			s.Dispatches[0].RunID = "other"
			s.LeaseHolderIndexes[0].AgentID = "other"
		}, "51838c9d1355e73e3b686642fac795bb7682c32653e5a343731bb7723d91874d"},
		{"legacy-one", func(s *Snapshot) { s.Cycles[0].RunID = "" }, "74234e98afe7498fb5daf1f36ac2d78acc339464f950703b8c019892f982b90b"},
		{"legacy-zero", func(s *Snapshot) { s.Cycles[0].RunID = ""; s.Tasks = nil }, "5a278d19cff7a916a1fab9a75219fbbe082d6a5b5435953e15cf4fb729f2b6a9"},
		{"legacy-multiple", func(s *Snapshot) {
			s.Cycles[0].RunID = ""
			task := s.Tasks[0]
			task.RunID = "other"
			s.Tasks = append(s.Tasks, task)
		}, "8806496abea6c3a656bdadecbd46f5a0345ccc127c50e38055e6c93519a6b945"},
		{"same-task-different-run", func(s *Snapshot) {
			s.Cycles[1].TaskID = s.Cycles[0].TaskID
			s.Cycles[1].RunID = "other"
			s.Tasks[1].ID, s.Tasks[1].RunID = s.Tasks[0].ID, "other"
			s.Dispatches[1].TaskID, s.Dispatches[1].RunID = s.Tasks[0].ID, "other"
		}, "74234e98afe7498fb5daf1f36ac2d78acc339464f950703b8c019892f982b90b"},
		{"native-duplicate", func(s *Snapshot) {
			s.Cycles[1].HolderSessionID = s.Cycles[0].HolderSessionID
			s.LeaseHolderIndexes[1].SessionID = s.Cycles[0].HolderSessionID
			s.Cycles[1].HolderHost, s.LeaseHolderIndexes[1].Host = "CODEX", "CODEX"
		}, "e5ea4b300d8c6ca32642801d5474ceba4ce02b7fc6639b6ea2ec7b8a0a8d5ecc"},
		{"whitespace", func(s *Snapshot) {
			s.Cycles[0].DispatchID = " " + s.Cycles[0].DispatchID + " "
			s.Cycles[0].TerminalPTYID = " " + s.Cycles[0].TerminalPTYID + " "
			s.LeaseHolderIndexes[0].LifecycleID = " " + s.LeaseHolderIndexes[0].LifecycleID + " "
		}, "608281ea5ed1ab6977b6deab4a9e67aab82e0a5153f4e2f4f9103e17f9487fea"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := indexedHealthSnapshot(2)
			tt.mutate(&s)
			before, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			result := Classify(s, Options{Now: time.Unix(1, 0)})
			data, err := json.Marshal(result.Findings)
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != tt.want {
				t.Errorf("complete findings digest = %s, want %s", got, tt.want)
			}
			after, err := json.Marshal(s)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("Classify mutated snapshot")
			}
		})
	}
}

func BenchmarkClassifyScaling(b *testing.B) {
	for _, n := range []int{100, 1000, 10000} {
		b.Run(fmt.Sprint(n), func(b *testing.B) {
			s := indexedHealthSnapshot(n)
			opts := Options{Now: time.Unix(1, 0)}
			if result := Classify(s, opts); !result.Healthy {
				b.Fatalf("fixture: %+v", result.Findings)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if result := Classify(s, opts); !result.Healthy {
					b.Fatal(result.Findings)
				}
			}
		})
	}
}

func TestResourceIndexVisitsEachValueOnce(t *testing.T) {
	for _, n := range []int{100, 1000, 10000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			values := make([]OrcaTask, n)
			for i := range values {
				values[i].ID = fmt.Sprint(i)
			}
			visits := 0
			index := indexBy(values, func(task OrcaTask) string {
				visits++
				return task.ID
			})
			for _, task := range values {
				if got, ok := index.unique(task.ID); !ok || got != task {
					t.Fatalf("lookup %q = %+v, %t", task.ID, got, ok)
				}
			}
			if visits != n {
				t.Fatalf("identity visits = %d, want %d", visits, n)
			}
		})
	}
}

func TestResourceIndexPreservesDuplicatesAndEmptyKeys(t *testing.T) {
	index := indexBy([]OrcaTask{{ID: " task ", RunID: "first"}, {ID: "task", RunID: "second"}, {ID: " "}}, func(task OrcaTask) string { return task.ID })
	if got, ok := index.unique("task"); ok || got != (OrcaTask{}) || index.counts()["task"] != 2 {
		t.Fatalf("duplicate lookup = %+v, %t; counts = %+v", got, ok, index.counts())
	}
	if got, ok := index.unique(""); !ok || got.ID != " " || index.counts()[""] != 0 {
		t.Fatalf("empty lookup = %+v, %t; counts = %+v", got, ok, index.counts())
	}
}

func TestLeaseHolderIndexUsesEveryExactTupleField(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*LeaseHolderIndex)
	}{
		{"lifecycle", func(i *LeaseHolderIndex) { i.LifecycleID = "other" }},
		{"generation", func(i *LeaseHolderIndex) { i.Generation++ }},
		{"host", func(i *LeaseHolderIndex) { i.Host = "claude" }},
		{"session", func(i *LeaseHolderIndex) { i.SessionID = "other" }},
		{"agent", func(i *LeaseHolderIndex) { i.AgentID = "other" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := healthyDirectSnapshot()
			tt.mutate(&s.LeaseHolderIndexes[0])
			result := Classify(s, Options{Now: time.Unix(1, 0)})
			want := []Finding{
				{Code: FindingInventoryUnknown, ResourceKind: "lease_holder", ResourceID: "current", Summary: "lease-holder reverse index must match exactly one active cycle"},
				{Code: FindingInventoryUnknown, ResourceKind: "lease_holder", ResourceID: "io-v1", Summary: "active cycle must match exactly one lease-holder reverse index"},
			}
			if !reflect.DeepEqual(result.Findings, want) {
				t.Fatalf("findings = %+v, want %+v", result.Findings, want)
			}
		})
	}
	// The reverse tuple is case-sensitive, unlike duplicate-native-owner identity.
	if holderKey("id", 1, "CODEX", "session", "") == holderKey("id", 1, "codex", "session", "") {
		t.Fatal("reverse tuple lowercased host")
	}
	if nativeHolderIdentity("CODEX", "session", "") != nativeHolderIdentity("codex", "session", "") {
		t.Fatal("native owner identity stopped lowercasing host")
	}
}

func TestClassifyIndexesAreRequestLocal(t *testing.T) {
	s := indexedHealthSnapshot(1)
	opts := Options{Now: time.Unix(1, 0)}
	if result := Classify(s, opts); !result.Healthy {
		t.Fatal(result.Findings)
	}
	s.Tasks[0].ID = "replacement"
	result := Classify(s, opts)
	found := false
	for _, finding := range result.Findings {
		if finding.ResourceKind == "task" && finding.ResourceID == s.Cycles[0].TaskID && strings.Contains(finding.Summary, "exactly one task") {
			found = true
		}
	}
	if !found {
		t.Fatalf("later request reused task inventory: %+v", result.Findings)
	}
}
