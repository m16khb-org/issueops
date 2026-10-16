package operationalhealth

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	"reflect"
	"strings"
	"testing"
	"time"
)

func indexedHealthSnapshot(n int) Snapshot {
	s := healthySnapshot()
	s.Cycles, s.LeaseHolderIndexes = nil, nil
	s.GitWorktrees = s.GitWorktrees[:1]
	for i := range n {
		id := fmt.Sprintf("resource-%d", i)
		c := healthySnapshot().Cycles[0]
		c.ID, c.Branch, c.WorktreePath, c.HolderSessionID = id, id, "/worktrees/"+id, id
		s.Cycles = append(s.Cycles, c)
		s.LeaseHolderIndexes = append(s.LeaseHolderIndexes, operationalhealthcontract.LeaseHolderIndex{Key: id, LifecycleID: id, Generation: c.Generation, Host: c.HolderHost, SessionID: id})
		s.GitWorktrees = append(s.GitWorktrees, operationalhealthcontract.GitWorktree{Path: c.WorktreePath, Branch: id, Head: s.SourceHead, Clean: true})
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
			s.LeaseHolderIndexes = append(s.LeaseHolderIndexes, s.LeaseHolderIndexes[0])
		}, "749cdc54e00485c35735ac3296602a37231ac525fdb78ee439a48720c633e649"},
		{"missing", func(s *Snapshot) {
			s.GitWorktrees = s.GitWorktrees[:1]
			s.LeaseHolderIndexes = nil
		}, "c151e3ba51763bc000ed4df3288f913a8d3d3ec6a505233fdeb0ac72c9ed9d25"},
		{"mismatch", func(s *Snapshot) {
			s.GitWorktrees[1].Branch = "other"
			s.LeaseHolderIndexes[0].AgentID = "other"
		}, "a1ab9a8878279ca8f50614f5c6509e458ad12acfbc13abaed4abd5f8f3559750"},
		{"native-duplicate", func(s *Snapshot) {
			s.Cycles[1].HolderSessionID = s.Cycles[0].HolderSessionID
			s.LeaseHolderIndexes[1].SessionID = s.Cycles[0].HolderSessionID
			s.Cycles[1].HolderHost, s.LeaseHolderIndexes[1].Host = "CODEX", "CODEX"
		}, "83cfb89a7447d1ea77faada4ca046db4a279f479eaf5ec6459bc4213c4bcea3a"},
		{"whitespace", func(s *Snapshot) {
			s.LeaseHolderIndexes[0].LifecycleID = " " + s.LeaseHolderIndexes[0].LifecycleID + " "
		}, "74234e98afe7498fb5daf1f36ac2d78acc339464f950703b8c019892f982b90b"},
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
			values := make([]operationalhealthcontract.GitWorktree, n)
			for i := range values {
				values[i].Path = fmt.Sprint(i)
			}
			visits := 0
			index := indexBy(values, func(worktree operationalhealthcontract.GitWorktree) string {
				visits++
				return worktree.Path
			})
			for _, worktree := range values {
				if got, ok := index.unique(worktree.Path); !ok || got != worktree {
					t.Fatalf("lookup %q = %+v, %t", worktree.Path, got, ok)
				}
			}
			if visits != n {
				t.Fatalf("identity visits = %d, want %d", visits, n)
			}
		})
	}
}

func TestResourceIndexPreservesDuplicatesAndEmptyKeys(t *testing.T) {
	index := indexBy([]operationalhealthcontract.GitWorktree{{Path: " tree ", Branch: "first"}, {Path: "tree", Branch: "second"}, {Path: " "}}, func(worktree operationalhealthcontract.GitWorktree) string { return worktree.Path })
	if got, ok := index.unique("tree"); ok || got != (operationalhealthcontract.GitWorktree{}) || index.counts()["tree"] != 2 {
		t.Fatalf("duplicate lookup = %+v, %t; counts = %+v", got, ok, index.counts())
	}
	if got, ok := index.unique(""); !ok || got.Path != " " || index.counts()[""] != 0 {
		t.Fatalf("empty lookup = %+v, %t; counts = %+v", got, ok, index.counts())
	}
}

func TestLeaseHolderIndexUsesEveryExactTupleField(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*operationalhealthcontract.LeaseHolderIndex)
	}{
		{"lifecycle", func(i *operationalhealthcontract.LeaseHolderIndex) { i.LifecycleID = "other" }},
		{"generation", func(i *operationalhealthcontract.LeaseHolderIndex) { i.Generation++ }},
		{"host", func(i *operationalhealthcontract.LeaseHolderIndex) { i.Host = "claude" }},
		{"session", func(i *operationalhealthcontract.LeaseHolderIndex) { i.SessionID = "other" }},
		{"agent", func(i *operationalhealthcontract.LeaseHolderIndex) { i.AgentID = "other" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := healthySnapshot()
			tt.mutate(&s.LeaseHolderIndexes[0])
			result := Classify(s, Options{Now: time.Unix(1, 0)})
			want := []operationalhealthcontract.Finding{
				{Code: operationalhealthcontract.FindingInventoryUnknown, ResourceKind: "lease_holder", ResourceID: "current", Summary: "lease-holder reverse index must match exactly one active cycle"},
				{Code: operationalhealthcontract.FindingInventoryUnknown, ResourceKind: "lease_holder", ResourceID: "io-v1", Summary: "active cycle must match exactly one lease-holder reverse index"},
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
	s.GitWorktrees[1].Path = "/worktrees/replacement"
	result := Classify(s, opts)
	found := false
	for _, finding := range result.Findings {
		if finding.ResourceKind == "git_worktree" && finding.ResourceID == s.Cycles[0].WorktreePath && strings.Contains(finding.Summary, "exactly one Git worktree") {
			found = true
		}
	}
	if !found {
		t.Fatalf("later request reused worktree inventory: %+v", result.Findings)
	}
}
