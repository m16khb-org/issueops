package issueops

// CycleTimeline is a bounded, time-ordered list of the dated events one cycle
// record carries. It holds only machine-readable identifiers (kinds, phases,
// short subjects and refs), never free text, so it can outlive the record in a
// retained CycleOutcome without bloating or leaking record prose.
type CycleTimeline struct {
	// EventsTotal counts every dated event before the cap; EventsShown counts
	// the events kept after it.
	EventsTotal int `json:"events_total"`
	EventsShown int `json:"events_shown"`
	// Undated counts record timestamps that were present but empty or
	// unparseable. They are left out of Events instead of being guessed.
	Undated int `json:"undated"`
	// Limits names the places where the record keeps less history than the
	// timeline implies, so a reader does not mistake absence for "never".
	Limits []string        `json:"limits,omitempty"`
	Events []TimelineEvent `json:"events"`
}

// TimelineEvent is one dated cycle event. At is RFC 3339 in UTC; the other
// fields are filled only when the event's source carries them.
type TimelineEvent struct {
	At         string `json:"at"`
	Kind       string `json:"kind"`
	Phase      string `json:"phase,omitempty"`
	Subject    string `json:"subject,omitempty"`
	Ref        string `json:"ref,omitempty"`
	Verdict    string `json:"verdict,omitempty"`
	Round      int    `json:"round,omitempty"`
	Generation uint64 `json:"generation,omitempty"`
}

// The TimelineEvent kinds. The declaration order is the tie-break order for
// events that share one instant, so new kinds are appended, not inserted.
const (
	TimelineKindCycleStarted       = "cycle_started"
	TimelineKindIntentRecorded     = "intent_recorded"
	TimelineKindDelegated          = "delegated"
	TimelineKindIssueCreateStarted = "issue_create_started"
	TimelineKindPhaseEntered       = "phase_entered"
	TimelineKindPhaseCompleted     = "phase_completed"
	TimelineKindRegressed          = "regressed"
	TimelineKindDecision           = "decision"
	TimelineKindReview             = "review"
	TimelineKindAISlopCleaned      = "ai_slop_cleaned"
	TimelineKindSkillRouted        = "skill_routed"
	TimelineKindLeaseClaimed       = "lease_claimed"
	TimelineKindLeaseReleased      = "lease_released"
	TimelineKindLeaseReplaced      = "lease_replaced"
	TimelineKindPRVerified         = "pr_verified"
	TimelineKindBodySynced         = "body_synced"
	TimelineKindCompleted          = "completed"
	TimelineKindCompletionReopened = "completion_reopened"
	TimelineKindBaseSynced         = "base_synced"
	TimelineKindExecutionFailed    = "execution_failed"
	TimelineKindRemoteReflected    = "remote_reflected"
	TimelineKindIssueClosed        = "issue_closed"
	TimelineKindMerged             = "merged"
	TimelineKindChildCreated       = "child_created"
	TimelineKindChildValidated     = "child_validated"
	TimelineKindCleanupFailed      = "cleanup_failed"
)

// The review subjects a TimelineKindReview event can carry.
const (
	TimelineReviewArchitecture   = "architecture"
	TimelineReviewPlan           = "plan"
	TimelineReviewImplementation = "implementation"
	TimelineReviewProjectDocs    = "project_docs"
	TimelineReviewSchemaEvidence = "schema_evidence"
)

// The CycleTimeline limit names: places where the record keeps only the latest
// value, so the timeline shows less history than actually happened.
const (
	// TimelineLimitPhaseLedger: the ledger keeps the first entry and the latest
	// completion of each phase.
	TimelineLimitPhaseLedger = "phase_ledger_first_entry_last_completion"
	// TimelineLimitLeaseLastValue: the lease keeps its last claim, release and
	// replacement only.
	TimelineLimitLeaseLastValue = "lease_last_value_only"
	// TimelineLimitSingleReview: implementation, project-docs and schema
	// reviews keep their latest record only.
	TimelineLimitSingleReview = "single_review_latest_only"
	// TimelineLimitRetainedWithoutTimeline: a retained outcome written before
	// timelines existed has none.
	TimelineLimitRetainedWithoutTimeline = "retained_without_timeline"
)

// Timeline bounds. MaxTimelineEvents keeps the newest events; subject and ref
// text is cut to MaxTimelineTextBytes so a retained outcome row stays small.
const (
	MaxTimelineEvents    = 200
	MaxTimelineTextBytes = 64
)
