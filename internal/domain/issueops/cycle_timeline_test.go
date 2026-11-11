package issueops

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
)

const (
	tlT1 = "2026-11-01T10:00:00Z"
	tlT2 = "2026-11-01T11:00:00Z"
)

// wantTimeline builds the expected timeline of a record whose dated events are
// exactly events, in order, with nothing undated and nothing capped.
func wantTimeline(limits []string, events ...model.TimelineEvent) model.CycleTimeline {
	if events == nil {
		events = []model.TimelineEvent{}
	}
	return model.CycleTimeline{EventsTotal: len(events), EventsShown: len(events), Limits: limits, Events: events}
}

func TestBuildCycleTimeline(t *testing.T) {
	phaseLimit := []string{model.TimelineLimitPhaseLedger}
	leaseLimit := []string{model.TimelineLimitLeaseLastValue}
	singleLimit := []string{model.TimelineLimitSingleReview}
	lease := func(mutate func(*model.WriteLease)) *model.Execution {
		l := model.WriteLease{Generation: 3, Holder: &model.NativeActor{Host: "claude"}}
		mutate(&l)
		return &model.Execution{Lease: l}
	}

	tests := []struct {
		name   string
		record model.Record
		want   model.CycleTimeline
	}{
		{"cycle_started", model.Record{CreatedAt: tlT1},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "cycle_started"})},
		{"intent_recorded", model.Record{Intent: &model.IntentContract{RecordedAt: tlT1}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "intent_recorded"})},
		{"delegated", model.Record{Delegation: &model.DelegationContract{ParentCycleID: "io-parent", DelegatedAt: tlT1}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "delegated", Ref: "io-parent"})},
		{"issue_create_started", model.Record{IssueCreateIntent: &model.IssueCreateIntent{Provider: "github", StartedAt: tlT1}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "issue_create_started", Subject: "github"})},
		{"phase_entered", model.Record{PhaseLedger: model.PhaseLedger{model.PhasePlan: {Phase: model.PhasePlan, EnteredAt: tlT1}}},
			wantTimeline(phaseLimit, model.TimelineEvent{At: tlT1, Kind: "phase_entered", Phase: "plan"})},
		{"phase_completed", model.Record{PhaseLedger: model.PhaseLedger{model.PhasePlan: {Phase: model.PhasePlan, CompletedAt: tlT1}}},
			wantTimeline(phaseLimit, model.TimelineEvent{At: tlT1, Kind: "phase_completed", Phase: "plan"})},
		{"regressed", model.Record{RegressEvents: []model.RegressEvent{{Reason: "free text", FromPhase: model.PhaseImplement, At: tlT1}}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "regressed", Phase: "implement"})},
		{"decision", model.Record{Decisions: []model.Decision{{Title: "free title", Body: "free body", Kind: "scope", CreatedAt: tlT1}}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "decision", Subject: "scope"})},
		{"review_architecture", model.Record{ArchitectureReview: &reviewcontract.ArchitectureReview{
			Approved: true, ReviewedAt: tlT2,
			History: []reviewcontract.ArchitectureReviewRound{{Approved: false, ReviewedAt: tlT1}},
		}}, wantTimeline(nil,
			model.TimelineEvent{At: tlT1, Kind: "review", Subject: "architecture", Verdict: "not_approved", Round: 1},
			model.TimelineEvent{At: tlT2, Kind: "review", Subject: "architecture", Verdict: "approved", Round: 2})},
		{"review_plan", model.Record{DevilsAdvocateReview: &model.DevilsAdvocateReview{
			Verdict: "pass", RecordedAt: tlT2,
			History: []model.DevilsAdvocateRound{{Verdict: "revise", RecordedAt: tlT1}},
		}}, wantTimeline(nil,
			model.TimelineEvent{At: tlT1, Kind: "review", Subject: "plan", Verdict: "revise", Round: 1},
			model.TimelineEvent{At: tlT2, Kind: "review", Subject: "plan", Verdict: "pass", Round: 2})},
		{"review_implementation", model.Record{ImplementationReview: &model.ImplementationReview{Verdict: "pass", RecordedAt: tlT1}},
			wantTimeline(singleLimit, model.TimelineEvent{At: tlT1, Kind: "review", Subject: "implementation", Verdict: "pass"})},
		{"review_project_docs", model.Record{ProjectDocsReview: &model.ProjectDocsReview{Verdict: "no-change", RecordedAt: tlT1}},
			wantTimeline(singleLimit, model.TimelineEvent{At: tlT1, Kind: "review", Subject: "project_docs", Verdict: "no-change"})},
		{"review_schema_evidence_measured", model.Record{SchemaEvidence: &model.SchemaEvidence{RecordedAt: tlT1}},
			wantTimeline(singleLimit, model.TimelineEvent{At: tlT1, Kind: "review", Subject: "schema_evidence", Verdict: "measured"})},
		{"review_schema_evidence_waived", model.Record{SchemaEvidence: &model.SchemaEvidence{Waived: true, RecordedAt: tlT1}},
			wantTimeline(singleLimit, model.TimelineEvent{At: tlT1, Kind: "review", Subject: "schema_evidence", Verdict: "waived"})},
		{"ai_slop_cleaned", model.Record{AISlopCleanAt: tlT1},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "ai_slop_cleaned"})},
		{"skill_with_control_characters_becomes_placeholder", model.Record{RoutingTrace: []model.SkillRoutingEntry{{Phase: "plan", Skill: "\x1b[31mred", At: tlT1}}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "skill_routed", Phase: "plan", Subject: "?"})},
		{"skill_routed", model.Record{RoutingTrace: []model.SkillRoutingEntry{{Phase: "plan", Skill: "issueops-plan", At: tlT1}}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "skill_routed", Phase: "plan", Subject: "issueops-plan"})},
		{"lease_claimed", model.Record{Execution: lease(func(l *model.WriteLease) { l.ClaimedAt = tlT1 })},
			wantTimeline(leaseLimit, model.TimelineEvent{At: tlT1, Kind: "lease_claimed", Subject: "claude", Generation: 3})},
		{"lease_released", model.Record{Execution: lease(func(l *model.WriteLease) { l.ReleasedAt = tlT1 })},
			wantTimeline(leaseLimit, model.TimelineEvent{At: tlT1, Kind: "lease_released", Generation: 3})},
		{"lease_replaced", model.Record{Execution: lease(func(l *model.WriteLease) { l.ReplacedAt = tlT1; l.ReplacementReason = "free text" })},
			wantTimeline(leaseLimit, model.TimelineEvent{At: tlT1, Kind: "lease_replaced", Generation: 3})},
		{"lease_released_and_replaced_omit_holder", model.Record{Execution: lease(func(l *model.WriteLease) { l.ClaimedAt = tlT1; l.ReleasedAt = tlT2; l.ReplacedAt = tlT2 })},
			wantTimeline(leaseLimit,
				model.TimelineEvent{At: tlT1, Kind: "lease_claimed", Subject: "claude", Generation: 3},
				model.TimelineEvent{At: tlT2, Kind: "lease_released", Generation: 3},
				model.TimelineEvent{At: tlT2, Kind: "lease_replaced", Generation: 3})},
		{"lease_without_holder", model.Record{Execution: lease(func(l *model.WriteLease) { l.Holder = nil; l.ClaimedAt = tlT1 })},
			wantTimeline(leaseLimit, model.TimelineEvent{At: tlT1, Kind: "lease_claimed", Generation: 3})},
		{"pr_verified", model.Record{RemoteArtifact: &model.RemoteArtifactVerification{Kind: "pull_request", URL: "https://example.test/pr/1", VerifiedAt: tlT1}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "pr_verified", Subject: "pull_request"})},
		{"body_synced", model.Record{BodySyncs: []model.RemoteBodySync{{Kind: "issue", URL: "https://example.test/1", SyncedAt: tlT1}}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "body_synced", Subject: "issue"})},
		{"completed_current", model.Record{Execution: &model.Execution{Completion: &model.ExecutionCompletion{Generation: 4, CompletedAt: tlT1}}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "completed", Generation: 4})},
		{"completed_history", model.Record{Execution: &model.Execution{CompletionHistory: []model.ExecutionCompletionHistory{
			{Generation: 2, Completion: model.ExecutionCompletion{Generation: 9, CompletedAt: tlT1}, ReopenedAt: tlT2},
		}}}, wantTimeline(nil,
			model.TimelineEvent{At: tlT1, Kind: "completed", Generation: 2},
			model.TimelineEvent{At: tlT2, Kind: "completion_reopened", Generation: 2})},
		{"completion_reopened", model.Record{Execution: &model.Execution{CompletionHistory: []model.ExecutionCompletionHistory{
			{Generation: 2, Reason: "free text", Completion: model.ExecutionCompletion{CompletedAt: tlT1}, ReopenedAt: tlT2},
		}}}, wantTimeline(nil,
			model.TimelineEvent{At: tlT1, Kind: "completed", Generation: 2},
			model.TimelineEvent{At: tlT2, Kind: "completion_reopened", Generation: 2})},
		// A history element exists only because the completion happened and was
		// reopened, so an empty time is a lost timestamp, not an absent event.
		{"completion_history_empty_times_are_undated", model.Record{Execution: &model.Execution{CompletionHistory: []model.ExecutionCompletionHistory{
			{Generation: 2},
		}}}, model.CycleTimeline{Undated: 2, Events: []model.TimelineEvent{}}},
		{"base_synced", model.Record{Execution: &model.Execution{SyncBaseEvents: []model.ExecutionSyncBaseEvent{{Mode: "apply", At: tlT1}}}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "base_synced", Subject: "apply"})},
		{"execution_failed", model.Record{Execution: &model.Execution{Failure: &model.ExecutionFailure{Code: "lease_lost", Message: "free text", At: tlT1}}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "execution_failed", Subject: "lease_lost"})},
		{"remote_reflected", model.Record{RemoteCompletion: &model.RemoteCompletion{ReflectedAt: tlT1}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "remote_reflected"})},
		{"issue_closed", model.Record{RemoteCompletion: &model.RemoteCompletion{IssueClosedAt: tlT1}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "issue_closed"})},
		{"merged", model.Record{RemoteCompletion: &model.RemoteCompletion{MergedAt: tlT1}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "merged"})},
		{"child_created", model.Record{ChildCycles: []model.ChildCycleRef{{CycleID: "io-child", Title: "free title", CreatedAt: tlT1}}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "child_created", Ref: "io-child"})},
		{"child_validated", model.Record{ChildCycles: []model.ChildCycleRef{{CycleID: "io-child", CreatedAt: tlT1, ValidationVerdict: "accepted", ValidationReason: "free text", ValidatedAt: tlT2}}},
			wantTimeline(nil,
				model.TimelineEvent{At: tlT1, Kind: "child_created", Ref: "io-child"},
				model.TimelineEvent{At: tlT2, Kind: "child_validated", Subject: "accepted", Ref: "io-child"})},
		{"cleanup_failed_finish", model.Record{CleanupFinishFailure: &model.CleanupFinishFailure{Step: model.CleanupFailureStepWorktreeRemove, Message: "free text", At: tlT1}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "cleanup_failed", Subject: "worktree_remove", Ref: "finish"})},
		{"cleanup_failed_abandon", model.Record{CleanupAbandonFailure: &model.CleanupAbandonFailure{Step: model.CleanupFailureStepClosePR, Message: "free text", At: tlT1}},
			wantTimeline(nil, model.TimelineEvent{At: tlT1, Kind: "cleanup_failed", Subject: "close_pr", Ref: "abandon"})},
		// The applying step is the arm marker of a running abandon, not a failure.
		{"cleanup_abandon_arm_marker_is_not_a_failure", model.Record{CleanupAbandonFailure: &model.CleanupAbandonFailure{Step: model.CleanupFailureStepApplying, At: tlT1}},
			model.CycleTimeline{Events: []model.TimelineEvent{}}},

		// Boundaries.
		{"empty_record", model.Record{}, model.CycleTimeline{Events: []model.TimelineEvent{}}},
		{"nil_pointers_do_not_panic", model.Record{
			Execution: &model.Execution{}, RemoteCompletion: &model.RemoteCompletion{},
			ArchitectureReview: &reviewcontract.ArchitectureReview{}, DevilsAdvocateReview: &model.DevilsAdvocateReview{},
		}, model.CycleTimeline{Undated: 2, Events: []model.TimelineEvent{}}},
		{"garbage_is_undated", model.Record{CreatedAt: "garbage", Intent: &model.IntentContract{RecordedAt: tlT1}},
			model.CycleTimeline{EventsTotal: 1, EventsShown: 1, Undated: 1,
				Events: []model.TimelineEvent{{At: tlT1, Kind: "intent_recorded"}}}},
		{"offset_sorts_by_instant", model.Record{
			CreatedAt: tlT1, Intent: &model.IntentContract{RecordedAt: "2026-11-01T18:59:59+09:00"},
		}, wantTimeline(nil,
			model.TimelineEvent{At: "2026-11-01T09:59:59Z", Kind: "intent_recorded"},
			model.TimelineEvent{At: tlT1, Kind: "cycle_started"})},
		{"fraction_sorts_after_whole_second", model.Record{
			CreatedAt: "2026-11-01T10:00:00.5Z", Intent: &model.IntentContract{RecordedAt: tlT1},
		}, wantTimeline(nil,
			model.TimelineEvent{At: tlT1, Kind: "intent_recorded"},
			model.TimelineEvent{At: "2026-11-01T10:00:00.5Z", Kind: "cycle_started"})},
		{"tie_follows_kind_declaration_order", model.Record{
			ImplementationReview: &model.ImplementationReview{Verdict: "pass", RecordedAt: tlT1},
			PhaseLedger:          model.PhaseLedger{model.PhasePlan: {Phase: model.PhasePlan, EnteredAt: tlT1}},
		}, wantTimeline([]string{model.TimelineLimitPhaseLedger, model.TimelineLimitSingleReview},
			model.TimelineEvent{At: tlT1, Kind: "phase_entered", Phase: "plan"},
			model.TimelineEvent{At: tlT1, Kind: "review", Subject: "implementation", Verdict: "pass"})},
		{"limits_all_three_in_declaration_order", model.Record{
			PhaseLedger:          model.PhaseLedger{model.PhasePlan: {Phase: model.PhasePlan, EnteredAt: tlT1}},
			Execution:            lease(func(l *model.WriteLease) { l.ClaimedAt = tlT1 }),
			ImplementationReview: &model.ImplementationReview{Verdict: "pass", RecordedAt: tlT1},
		}, wantTimeline([]string{model.TimelineLimitPhaseLedger, model.TimelineLimitLeaseLastValue, model.TimelineLimitSingleReview},
			model.TimelineEvent{At: tlT1, Kind: "phase_entered", Phase: "plan"},
			model.TimelineEvent{At: tlT1, Kind: "review", Subject: "implementation", Verdict: "pass"},
			model.TimelineEvent{At: tlT1, Kind: "lease_claimed", Subject: "claude", Generation: 3})},
	}
	covered := map[string]bool{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildCycleTimeline(tc.record)
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("timeline mismatch\n got: %+v\nwant: %+v", got, tc.want)
			}
			if again := BuildCycleTimeline(tc.record); !reflect.DeepEqual(got, again) {
				t.Fatalf("not deterministic\nfirst: %+v\nsecond: %+v", got, again)
			}
			for _, event := range got.Events {
				covered[event.Kind] = true
			}
		})
	}
	for _, kind := range timelineKinds {
		if !covered[kind] {
			t.Errorf("kind %q has no table row", kind)
		}
	}
	if len(timelineKinds) != 26 {
		t.Errorf("timelineKinds has %d kinds, want 26", len(timelineKinds))
	}
	if len(timelineKindRank) != len(timelineKinds) {
		t.Errorf("timelineKindRank has %d kinds, timelineKinds has %d", len(timelineKindRank), len(timelineKinds))
	}

	t.Run("empty_record_marshals_empty_events_array", func(t *testing.T) {
		data, err := json.Marshal(BuildCycleTimeline(model.Record{}))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), `"events":[]`) || strings.Contains(string(data), `"limits"`) {
			t.Fatalf("json = %s", data)
		}
	})

	t.Run("no_limits_means_nil", func(t *testing.T) {
		if got := BuildCycleTimeline(model.Record{CreatedAt: tlT1}).Limits; got != nil {
			t.Fatalf("limits = %#v, want nil", got)
		}
	})

	t.Run("over_cap_keeps_newest_two_hundred", func(t *testing.T) {
		base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
		record := model.Record{}
		// Newest first, so the builder has to sort rather than trust input order.
		for i := 204; i >= 0; i-- {
			record.Decisions = append(record.Decisions, model.Decision{Kind: "scope", CreatedAt: base.Add(time.Duration(i) * time.Minute).Format(time.RFC3339)})
		}
		got := BuildCycleTimeline(record)
		if got.EventsTotal != 205 || got.EventsShown != 200 || len(got.Events) != 200 {
			t.Fatalf("total=%d shown=%d len=%d", got.EventsTotal, got.EventsShown, len(got.Events))
		}
		if want := base.Add(5 * time.Minute).Format(time.RFC3339Nano); got.Events[0].At != want {
			t.Fatalf("first kept = %s, want the sixth oldest %s", got.Events[0].At, want)
		}
		if want := base.Add(204 * time.Minute).Format(time.RFC3339Nano); got.Events[199].At != want {
			t.Fatalf("last kept = %s, want %s", got.Events[199].At, want)
		}
	})

	t.Run("free_text_never_appears", func(t *testing.T) {
		secret := "SECRET-FREE-TEXT"
		record := model.Record{
			Decisions:     []model.Decision{{Title: secret, Body: secret, Rationale: secret, Kind: "scope", CreatedAt: tlT1}},
			RegressEvents: []model.RegressEvent{{Reason: secret, FromPhase: model.PhasePlan, At: tlT1}},
			Execution: &model.Execution{
				Failure:           &model.ExecutionFailure{Code: "code", Message: secret, At: tlT1},
				CompletionHistory: []model.ExecutionCompletionHistory{{Reason: secret, ReopenedAt: tlT1}},
			},
			DevilsAdvocateReview: &model.DevilsAdvocateReview{Verdict: "pass", Findings: []string{secret}, WaiverRationale: secret, RecordedAt: tlT1},
			ChildCycles:          []model.ChildCycleRef{{CycleID: "c", Title: secret, ValidationReason: secret, ValidatedAt: tlT1}},
		}
		data, err := json.Marshal(BuildCycleTimeline(record))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), secret) {
			t.Fatalf("timeline leaked record prose: %s", data)
		}
	})

	t.Run("subject_and_ref_are_truncated", func(t *testing.T) {
		long := strings.Repeat("x", 200)
		got := BuildCycleTimeline(model.Record{ChildCycles: []model.ChildCycleRef{{CycleID: long, ValidationVerdict: long, ValidatedAt: tlT1}}})
		for _, event := range got.Events {
			if len(event.Subject) > model.MaxTimelineTextBytes || len(event.Ref) > model.MaxTimelineTextBytes {
				t.Fatalf("event not truncated: %d/%d bytes", len(event.Subject), len(event.Ref))
			}
		}
	})
}

func TestTimelineText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"short_ascii_unchanged", "abc", "abc"},
		{"exactly_limit_unchanged", strings.Repeat("a", 64), strings.Repeat("a", 64)},
		{"every_allowed_punctuation_unchanged", "io-1._:/x-Y", "io-1._:/x-Y"},
		{"long_ascii_cut_to_limit", strings.Repeat("a", 100), strings.Repeat("a", 64)},
		{"multibyte_after_ascii_is_placeholder", strings.Repeat("a", 63) + "한", "?"},
		{"hangul_is_placeholder", strings.Repeat("한", 30), "?"},
		{"json_escaped_characters_are_placeholder", strings.Repeat("<", 64), "?"},
		{"control_characters_are_placeholder", "\x1b[31mred", "?"},
		{"space_is_placeholder", "a b", "?"},
		{"invalid_utf8_is_placeholder", "ab\xffcd", "?"},
		{"empty", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := timelineText(tc.in); got != tc.want {
				t.Fatalf("timelineText(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
