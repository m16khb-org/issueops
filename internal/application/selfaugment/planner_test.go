package selfaugment

import (
	"encoding/json"
	"errors"
	docs "issueops/internal/contract/docs"
	contract "issueops/internal/contract/selfaugment"
	state "issueops/internal/contract/state"
	verifydomain "issueops/internal/domain/selfverify"
	"reflect"
	"strings"
	"testing"
	"time"
)

type planRepository struct {
	calls     *[]string
	geniusErr error
}

func (r planRepository) ReadGeniusThink(root string) (string, string, error) {
	*r.calls = append(*r.calls, "genius:"+root)
	return root + "/GENIUS_THINK.md", "GI", r.geniusErr
}
func (r planRepository) HasImplementationDelta(root string) bool {
	*r.calls = append(*r.calls, "git:"+root)
	return true
}
func (r planRepository) CollectSignals(root string, n int, skills []string, text string) contract.SelfAugmentRepoSignals {
	*r.calls = append(*r.calls, "signals:"+root)
	return contract.SelfAugmentRepoSignals{HasGeniusThink: text != "", DocsIndexed: n, Skills: skills}
}

func TestPlannerObservesEachGoalOnceAndUsesOneSnapshotForScoreAndEvidence(t *testing.T) {
	calls := []string{}
	lists := 0
	now := time.Date(2026, 9, 29, 3, 0, 0, 0, time.UTC)
	planner := Planner{
		Repository: planRepository{calls: &calls},
		DocsIndex: func(root, version string) docs.DocsIndexResult {
			calls = append(calls, "docs:"+root+":"+version)
			return docs.DocsIndexResult{OK: true}
		},
		ListSkillNames: func(root string) ([]string, error) {
			calls = append(calls, "skills:"+root)
			return []string{"self-augment"}, nil
		},
		StateList: func() (state.StateListResult, error) {
			lists++
			calls = append(calls, "list")
			if lists == 1 {
				return state.StateListResult{Keys: []string{"self-augment-lesson-observed"}}, nil
			}
			return state.StateListResult{}, nil
		},
		StateRead: func(key string) (state.StateResult, error) {
			calls = append(calls, "read:"+key)
			return state.StateResult{Record: state.RecordEnvelope{Content: currentSummaryFixture(t, true)}}, nil
		},
		Now: func() time.Time { return now },
	}
	result := planner.Plan(contract.SelfAugmentPlanRequest{Cycles: 2, TargetScore: 95}, "/repo", "v1")
	want := []string{"genius:/repo", "docs:/repo:v1", "skills:/repo", "signals:/repo", "git:/repo", "read:self-verify-latest", "list", "list"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("observations=%v want=%v", calls, want)
	}
	if !result.TerminationEligible || len(result.Goals) != 4 {
		t.Fatalf("unexpected goals: %+v", result.Goals)
	}
	for _, goal := range result.Goals {
		if !goal.Passed || goal.Score != 100 {
			t.Fatalf("unexpected goal: %+v", goal)
		}
	}
	if result.Goals[3].Evidence[0] != "augmentation lessons present under self-augment-lesson-* state keys" {
		t.Fatal(result.Goals[3])
	}
	if result.GeneratedAt != now.Format(time.RFC3339Nano) || result.GeniusThinkPath != "/repo/GENIUS_THINK.md" || result.IssueOpsRoot != "/repo" || result.Cycles != 2 {
		t.Fatalf("metadata not preserved: %+v", result)
	}
	if result.Warnings == nil || len(result.Warnings) != 0 {
		t.Fatal(result.Warnings)
	}
}

func TestPlannerObservationErrorsFailGoalsAndPreserveOrderedWarnings(t *testing.T) {
	calls := []string{}
	planner := Planner{
		Repository:     planRepository{calls: &calls, geniusErr: errors.New("not found")},
		DocsIndex:      func(string, string) docs.DocsIndexResult { return docs.DocsIndexResult{} },
		ListSkillNames: func(string) ([]string, error) { return nil, errors.New("skills unavailable") },
		StateList:      func() (state.StateListResult, error) { return state.StateListResult{}, errors.New("store unavailable") },
		StateRead:      func(string) (state.StateResult, error) { return state.StateResult{}, errors.New("read unavailable") },
		Now:            func() time.Time { return time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC) },
	}
	result := planner.Plan(contract.SelfAugmentPlanRequest{TargetScore: 95}, "/repo", "v1")
	if result.UsesGeniusThink || result.TerminationEligible || result.Goals[0].Passed || result.Goals[2].Passed || result.Goals[3].Passed {
		t.Fatal(result.Goals)
	}
	if len(result.Warnings) != 3 || !strings.HasPrefix(result.Warnings[0], "GENIUS_THINK.md not found") || result.Warnings[1] != "list skills: skills unavailable" || result.Warnings[2] != "lesson scan: store unavailable" {
		t.Fatal(result.Warnings)
	}
}

func TestLessonScanSkipsUnreadableMalformedAndWrongKindBeforeCounting(t *testing.T) {
	now := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	reads := []string{}
	planner := Planner{
		StateList: func() (state.StateListResult, error) {
			return state.StateListResult{Keys: []string{"unrelated", "self-augment-lesson-read-error", "self-augment-lesson-malformed", "self-augment-lesson-wrong-kind", "self-augment-lesson-recent"}}, nil
		},
		StateRead: func(key string) (state.StateResult, error) {
			reads = append(reads, key)
			switch key {
			case "self-augment-lesson-read-error":
				return state.StateResult{}, errors.New("unreadable")
			case "self-augment-lesson-malformed":
				return state.StateResult{Record: state.RecordEnvelope{Content: "{"}}, nil
			case "self-augment-lesson-wrong-kind":
				return state.StateResult{Record: state.RecordEnvelope{Content: `{"kind":"other","candidate_id":"wrong","severity":"error","generated_at":"2026-09-29T00:00:00Z"}`}}, nil
			default:
				return state.StateResult{Record: state.RecordEnvelope{Content: `{"schema_version":1,"kind":"self_augmentation_lesson","candidate_id":"candidate","severity":"error","generated_at":"2026-09-29T00:00:00Z"}`}}, nil
			}
		},
	}
	counts, warnings := planner.SevereLessonCountsAt(now)
	if !reflect.DeepEqual(counts, map[string]int{"candidate": 1}) || warnings != nil || len(reads) != 4 {
		t.Fatalf("counts=%v warnings=%v reads=%v", counts, warnings, reads)
	}
}

func currentSummaryFixture(t *testing.T, ok bool) string {
	t.Helper()
	snapshot := contract.SelfAugmentStateSnapshot{SchemaVersion: 1, Kind: "self_verification_summary", OK: ok, Summary: contract.SelfAugmentSummary{TerminationEligible: ok, Contract: verifydomain.ContractValue()}}
	NormalizeSnapshotFailureCause(&snapshot)
	data, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
