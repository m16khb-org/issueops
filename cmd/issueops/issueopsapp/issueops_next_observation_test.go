package issueopsapp

import (
	"reflect"
	"testing"

	issueopscore "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/issueops/implementation"
	issueopscontract "issueops/internal/contract/issueops"
)

func TestNextLocalReadinessObservationReusesOnlyTheSameRecord(t *testing.T) {
	recordA := issueopscontract.IssueOpsRecord{
		ID: "io-shared", Repo: "/repo-a", WorktreePath: "/repo-a.worktrees/feature", Branch: "feature",
		BranchPrepare: &issueopscontract.IssueOpsBranchPrepare{BaseBranch: "main", BaseSHA: "base-a"},
	}
	recordB := recordA
	recordB.Repo = "/repo-b"
	recordB.WorktreePath = "/repo-b.worktrees/feature"
	observeCalls, fallbackCalls := 0, 0
	observer := nextLocalReadinessObservation{
		observe: func(record issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsReadiness, implementation.LocalChangeObservation) {
			observeCalls++
			return issueopscontract.IssueOpsReadiness{OK: true, Ready: true}, implementation.LocalChangeObservation{
				Paths: []string{"db/migrations/001.sql"}, Verified: true,
			}
		},
		fallback: func(record issueopscontract.IssueOpsRecord) ([]string, bool) {
			fallbackCalls++
			return []string{"fallback.go"}, true
		},
	}

	readiness := observer.localReadiness(recordA)
	paths, observed := observer.changedPaths(recordA)
	otherPaths, otherObserved := observer.changedPaths(recordB)

	if !readiness.Ready || observeCalls != 1 {
		t.Fatalf("local readiness observation was not retained: readiness=%+v calls=%d", readiness, observeCalls)
	}
	if !observed || !reflect.DeepEqual(paths, []string{"db/migrations/001.sql"}) {
		t.Fatalf("same record must reuse the verified paths: paths=%v observed=%v", paths, observed)
	}
	if !otherObserved || !reflect.DeepEqual(otherPaths, []string{"fallback.go"}) || fallbackCalls != 1 {
		t.Fatalf("different repository must be observed independently: paths=%v observed=%v calls=%d", otherPaths, otherObserved, fallbackCalls)
	}
	paths[0] = "mutated"
	reusedAgain, _ := observer.changedPaths(recordA)
	if !reflect.DeepEqual(reusedAgain, []string{"db/migrations/001.sql"}) {
		t.Fatalf("callers must not mutate the retained observation: %v", reusedAgain)
	}
}

func TestNextLocalReadinessObservationDoesNotPromoteUnverifiedPaths(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{ID: "io-unverified", Repo: "/repo"}
	fallbackCalls := 0
	observer := nextLocalReadinessObservation{
		observe: func(issueopscontract.IssueOpsRecord) (issueopscontract.IssueOpsReadiness, implementation.LocalChangeObservation) {
			return issueopscontract.IssueOpsReadiness{OK: true, Ready: false}, implementation.LocalChangeObservation{
				Paths: []string{"changing.go"}, Verified: false,
			}
		},
		fallback: func(issueopscontract.IssueOpsRecord) ([]string, bool) {
			fallbackCalls++
			return []string{"fallback.go"}, true
		},
	}

	observer.localReadiness(record)
	paths, observed := observer.changedPaths(record)

	if observed || !reflect.DeepEqual(paths, []string{"changing.go"}) {
		t.Fatalf("unverified paths must remain unobservable to review tier: paths=%v observed=%v", paths, observed)
	}
	if fallbackCalls != 0 {
		t.Fatalf("an unverified same-request observation must fail closed instead of re-observing: %d", fallbackCalls)
	}
}

func TestNextLocalReadinessObservationFallsBackWithoutSameRequestEvidence(t *testing.T) {
	record := issueopscontract.IssueOpsRecord{ID: "io-implement", Repo: "/repo"}
	fallbackCalls := 0
	observer := nextLocalReadinessObservation{
		observe: issueopscore.ObserveIssueOpsLocalPRReadiness,
		fallback: func(issueopscontract.IssueOpsRecord) ([]string, bool) {
			fallbackCalls++
			return []string{"implementation.go"}, true
		},
	}

	paths, observed := observer.changedPaths(record)

	if !observed || !reflect.DeepEqual(paths, []string{"implementation.go"}) || fallbackCalls != 1 {
		t.Fatalf("phase without local readiness must use a fresh observation: paths=%v observed=%v calls=%d", paths, observed, fallbackCalls)
	}
}
