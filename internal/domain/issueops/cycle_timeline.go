package issueops

import (
	"cmp"
	"slices"
	"strings"
	"time"

	model "issueops/internal/contract/issueops"
)

// timelineKinds lists every timeline kind in declaration order. Events that
// share one instant sort by that position, so the result never depends on the
// order the record's fields happen to be read.
var timelineKinds = []string{
	model.TimelineKindCycleStarted,
	model.TimelineKindIntentRecorded,
	model.TimelineKindDelegated,
	model.TimelineKindIssueCreateStarted,
	model.TimelineKindPhaseEntered,
	model.TimelineKindPhaseCompleted,
	model.TimelineKindRegressed,
	model.TimelineKindDecision,
	model.TimelineKindReview,
	model.TimelineKindAISlopCleaned,
	model.TimelineKindSkillRouted,
	model.TimelineKindLeaseClaimed,
	model.TimelineKindLeaseReleased,
	model.TimelineKindLeaseReplaced,
	model.TimelineKindPRVerified,
	model.TimelineKindBodySynced,
	model.TimelineKindCompleted,
	model.TimelineKindCompletionReopened,
	model.TimelineKindBaseSynced,
	model.TimelineKindExecutionFailed,
	model.TimelineKindRemoteReflected,
	model.TimelineKindIssueClosed,
	model.TimelineKindMerged,
	model.TimelineKindChildCreated,
	model.TimelineKindChildValidated,
	model.TimelineKindCleanupFailed,
}

// timelineKindRank maps each kind in timelineKinds to its position.
var timelineKindRank = func() map[string]int {
	rank := make(map[string]int, len(timelineKinds))
	for i, kind := range timelineKinds {
		rank[kind] = i
	}
	return rank
}()

type datedTimelineEvent struct {
	at    time.Time
	event model.TimelineEvent
}

// timelineBuilder gathers the dated events of one record.
type timelineBuilder struct {
	events  []datedTimelineEvent
	undated int
}

// add records one event. raw is the source timestamp. When the source is an
// optional single value (a ledger entry, a lease time) an empty raw means the
// event never happened and is skipped silently; when the source object exists
// only because the event happened (a list element, a recorded review) an empty
// raw is an undated event. An unparseable raw is always undated.
func (b *timelineBuilder) add(raw string, sourceExists bool, event model.TimelineEvent) {
	if raw == "" {
		if sourceExists {
			b.undated++
		}
		return
	}
	at, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		b.undated++
		return
	}
	at = at.UTC()
	event.At = at.Format(time.RFC3339Nano)
	event.Phase = timelineText(event.Phase)
	event.Subject = timelineText(event.Subject)
	event.Ref = timelineText(event.Ref)
	event.Verdict = timelineText(event.Verdict)
	b.events = append(b.events, datedTimelineEvent{at: at, event: event})
}

// BuildCycleTimeline 함수는 record에 흩어진 이력 시각을 시각 순 이벤트 목록으로 모은다. 순수 함수이며
// record를 바꾸지 않는다. 자유 문자열(사유, 제목, 본문, 메시지)은 싣지 않고, 시각이 비었거나 파싱되지 않는
// 항목은 이벤트에서 빼고 Undated로 센다. 이벤트가 MaxTimelineEvents를 넘으면 가장 최근 것만 남긴다.
func BuildCycleTimeline(record model.Record) model.CycleTimeline {
	var b timelineBuilder
	b.collectIdentity(record)
	b.collectPhases(record)
	b.collectReviews(record)
	b.collectRouting(record)
	b.collectExecution(record)
	b.collectRemote(record)
	b.collectChildren(record)
	b.collectCleanupFailures(record)

	slices.SortStableFunc(b.events, compareTimelineEvents)

	timeline := model.CycleTimeline{
		EventsTotal: len(b.events),
		Undated:     b.undated,
		Limits:      timelineLimits(b.events),
	}
	kept := b.events
	if len(kept) > model.MaxTimelineEvents {
		kept = kept[len(kept)-model.MaxTimelineEvents:]
	}
	timeline.Events = make([]model.TimelineEvent, 0, len(kept))
	for _, dated := range kept {
		timeline.Events = append(timeline.Events, dated.event)
	}
	timeline.EventsShown = len(timeline.Events)
	return timeline
}

func (b *timelineBuilder) collectIdentity(record model.Record) {
	b.add(record.CreatedAt, false, model.TimelineEvent{Kind: model.TimelineKindCycleStarted})
	if intent := record.Intent; intent != nil {
		b.add(intent.RecordedAt, true, model.TimelineEvent{Kind: model.TimelineKindIntentRecorded})
	}
	if delegation := record.Delegation; delegation != nil {
		b.add(delegation.DelegatedAt, true, model.TimelineEvent{Kind: model.TimelineKindDelegated, Ref: delegation.ParentCycleID})
	}
	if intent := record.IssueCreateIntent; intent != nil {
		b.add(intent.StartedAt, true, model.TimelineEvent{Kind: model.TimelineKindIssueCreateStarted, Subject: intent.Provider})
	}
	for _, event := range record.RegressEvents {
		b.add(event.At, true, model.TimelineEvent{Kind: model.TimelineKindRegressed, Phase: string(event.FromPhase)})
	}
	for _, decision := range record.Decisions {
		b.add(decision.CreatedAt, true, model.TimelineEvent{Kind: model.TimelineKindDecision, Subject: decision.Kind})
	}
	b.add(record.AISlopCleanAt, false, model.TimelineEvent{Kind: model.TimelineKindAISlopCleaned})
}

// collectPhases walks the ledger in Phases order, never map order.
func (b *timelineBuilder) collectPhases(record model.Record) {
	for _, phase := range model.Phases {
		entry, ok := record.PhaseLedger[phase]
		if !ok {
			continue
		}
		b.add(entry.EnteredAt, false, model.TimelineEvent{Kind: model.TimelineKindPhaseEntered, Phase: string(phase)})
		b.add(entry.CompletedAt, false, model.TimelineEvent{Kind: model.TimelineKindPhaseCompleted, Phase: string(phase)})
	}
}

func (b *timelineBuilder) collectReviews(record model.Record) {
	if review := record.ArchitectureReview; review != nil {
		for index, round := range review.History {
			b.add(round.ReviewedAt, true, reviewEvent(model.TimelineReviewArchitecture, approvalVerdict(round.Approved), index+1))
		}
		b.add(review.ReviewedAt, true, reviewEvent(model.TimelineReviewArchitecture, approvalVerdict(review.Approved), len(review.History)+1))
	}
	if review := record.DevilsAdvocateReview; review != nil {
		for index, round := range review.History {
			b.add(round.RecordedAt, true, reviewEvent(model.TimelineReviewPlan, round.Verdict, index+1))
		}
		b.add(review.RecordedAt, true, reviewEvent(model.TimelineReviewPlan, review.Verdict, len(review.History)+1))
	}
	if review := record.ImplementationReview; review != nil {
		b.add(review.RecordedAt, true, reviewEvent(model.TimelineReviewImplementation, review.Verdict, 0))
	}
	if review := record.ProjectDocsReview; review != nil {
		b.add(review.RecordedAt, true, reviewEvent(model.TimelineReviewProjectDocs, review.Verdict, 0))
	}
	if evidence := record.SchemaEvidence; evidence != nil {
		verdict := "measured"
		if evidence.Waived {
			verdict = "waived"
		}
		b.add(evidence.RecordedAt, true, reviewEvent(model.TimelineReviewSchemaEvidence, verdict, 0))
	}
}

func reviewEvent(subject, verdict string, round int) model.TimelineEvent {
	return model.TimelineEvent{Kind: model.TimelineKindReview, Subject: subject, Verdict: verdict, Round: round}
}

func approvalVerdict(approved bool) string {
	if approved {
		return "approved"
	}
	return "not_approved"
}

func (b *timelineBuilder) collectRouting(record model.Record) {
	for _, entry := range record.RoutingTrace {
		// A routing entry's At is optional in the record, so an empty value is
		// a trace that was never stamped rather than a lost timestamp.
		b.add(entry.At, false, model.TimelineEvent{Kind: model.TimelineKindSkillRouted, Phase: entry.Phase, Subject: entry.Skill})
	}
}

func (b *timelineBuilder) collectExecution(record model.Record) {
	execution := record.Execution
	if execution == nil {
		return
	}
	lease := execution.Lease
	holder := ""
	if lease.Holder != nil {
		holder = lease.Holder.Host
	}
	// Only the claim names the holder: the lease keeps no earlier holder, so
	// after a release or replacement the current one is not who acted.
	b.add(lease.ClaimedAt, false, model.TimelineEvent{Kind: model.TimelineKindLeaseClaimed, Subject: holder, Generation: lease.Generation})
	b.add(lease.ReleasedAt, false, model.TimelineEvent{Kind: model.TimelineKindLeaseReleased, Generation: lease.Generation})
	b.add(lease.ReplacedAt, false, model.TimelineEvent{Kind: model.TimelineKindLeaseReplaced, Generation: lease.Generation})
	if completion := execution.Completion; completion != nil {
		b.add(completion.CompletedAt, true, model.TimelineEvent{Kind: model.TimelineKindCompleted, Generation: completion.Generation})
	}
	for _, history := range execution.CompletionHistory {
		b.add(history.Completion.CompletedAt, true, model.TimelineEvent{Kind: model.TimelineKindCompleted, Generation: history.Generation})
		b.add(history.ReopenedAt, true, model.TimelineEvent{Kind: model.TimelineKindCompletionReopened, Generation: history.Generation})
	}
	for _, event := range execution.SyncBaseEvents {
		b.add(event.At, true, model.TimelineEvent{Kind: model.TimelineKindBaseSynced, Subject: event.Mode})
	}
	if failure := execution.Failure; failure != nil {
		b.add(failure.At, true, model.TimelineEvent{Kind: model.TimelineKindExecutionFailed, Subject: failure.Code})
	}
}

func (b *timelineBuilder) collectRemote(record model.Record) {
	if artifact := record.RemoteArtifact; artifact != nil {
		b.add(artifact.VerifiedAt, true, model.TimelineEvent{Kind: model.TimelineKindPRVerified, Subject: artifact.Kind})
	}
	for _, sync := range record.BodySyncs {
		b.add(sync.SyncedAt, true, model.TimelineEvent{Kind: model.TimelineKindBodySynced, Subject: sync.Kind})
	}
	if completion := record.RemoteCompletion; completion != nil {
		b.add(completion.ReflectedAt, false, model.TimelineEvent{Kind: model.TimelineKindRemoteReflected})
		b.add(completion.IssueClosedAt, false, model.TimelineEvent{Kind: model.TimelineKindIssueClosed})
		b.add(completion.MergedAt, false, model.TimelineEvent{Kind: model.TimelineKindMerged})
	}
}

func (b *timelineBuilder) collectChildren(record model.Record) {
	for _, child := range record.ChildCycles {
		b.add(child.CreatedAt, true, model.TimelineEvent{Kind: model.TimelineKindChildCreated, Ref: child.CycleID})
		b.add(child.ValidatedAt, false, model.TimelineEvent{Kind: model.TimelineKindChildValidated, Subject: child.ValidationVerdict, Ref: child.CycleID})
	}
}

// collectCleanupFailures adds the stopped cleanup applies. An abandon receipt
// whose step is applying is skipped: the applying step is the arm marker of a
// running cleanup, not a failure. Finish arms without writing a receipt.
func (b *timelineBuilder) collectCleanupFailures(record model.Record) {
	if failure := record.CleanupFinishFailure; failure != nil {
		b.add(failure.At, true, model.TimelineEvent{Kind: model.TimelineKindCleanupFailed, Subject: failure.Step, Ref: "finish"})
	}
	if failure := record.CleanupAbandonFailure; failure != nil && failure.Step != model.CleanupFailureStepApplying {
		b.add(failure.At, true, model.TimelineEvent{Kind: model.TimelineKindCleanupFailed, Subject: failure.Step, Ref: "abandon"})
	}
}

// compareTimelineEvents orders by instant, then kind declaration order,
// subject, ref and round, so equal instants sort the same on every run.
func compareTimelineEvents(a, b datedTimelineEvent) int {
	if c := a.at.Compare(b.at); c != 0 {
		return c
	}
	if c := cmp.Compare(timelineKindRank[a.event.Kind], timelineKindRank[b.event.Kind]); c != 0 {
		return c
	}
	if c := strings.Compare(a.event.Subject, b.event.Subject); c != 0 {
		return c
	}
	if c := strings.Compare(a.event.Ref, b.event.Ref); c != 0 {
		return c
	}
	return cmp.Compare(a.event.Round, b.event.Round)
}

// timelineLimits names, in declaration order, where the record keeps only the
// latest value for the event kinds present. It returns nil when none apply.
func timelineLimits(events []datedTimelineEvent) []string {
	var phase, lease, single bool
	for _, dated := range events {
		switch dated.event.Kind {
		case model.TimelineKindPhaseEntered, model.TimelineKindPhaseCompleted:
			phase = true
		case model.TimelineKindLeaseClaimed, model.TimelineKindLeaseReleased, model.TimelineKindLeaseReplaced:
			lease = true
		case model.TimelineKindReview:
			switch dated.event.Subject {
			case model.TimelineReviewImplementation, model.TimelineReviewProjectDocs, model.TimelineReviewSchemaEvidence:
				single = true
			}
		}
	}
	var limits []string
	if phase {
		limits = append(limits, model.TimelineLimitPhaseLedger)
	}
	if lease {
		limits = append(limits, model.TimelineLimitLeaseLastValue)
	}
	if single {
		limits = append(limits, model.TimelineLimitSingleReview)
	}
	return limits
}

// timelineText makes a subject, ref, phase or verdict safe to retain. A value with any byte
// outside [A-Za-z0-9._:/-] becomes "?" whole, because the retained row outlives
// the record and must not carry control characters, secrets or bytes that JSON
// escapes to several times their width. Any other value is cut to
// MaxTimelineTextBytes; the allowed set is ASCII, so the cut never splits a
// character.
func timelineText(s string) string {
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		case c == '.', c == '_', c == ':', c == '/', c == '-':
		default:
			return "?"
		}
	}
	if len(s) > model.MaxTimelineTextBytes {
		return s[:model.MaxTimelineTextBytes]
	}
	return s
}
