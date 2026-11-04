package issueops

import (
	"strings"
	"testing"
)

func TestValidateRecordRejectsInvalidPersistedShapes(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Record)
		wantErr string
	}{
		{"schema version", func(r *Record) { r.SchemaVersion = SchemaVersion + 1 }, "schema version is invalid"},
		{"phase", func(r *Record) { r.Phase = "nonexistent" }, `phase "nonexistent" is invalid`},
		{"phase ledger key mismatch", func(r *Record) {
			r.PhaseLedger = PhaseLedger{PhasePlan: {Phase: PhasePlan + "-other"}}
		}, "phase ledger identity is invalid"},
		{"intent class", func(r *Record) { r.Intent = &IntentContract{IntentClass: "not-a-class"} }, "unknown intent_class"},
		{"feedback classification", func(r *Record) {
			r.Feedback = []FeedbackItem{{Classification: "bogus"}}
		}, "feedback classification is invalid"},
		{"feedback resolution", func(r *Record) {
			r.Feedback = []FeedbackItem{{Classification: "defect", Resolution: "bogus"}}
		}, "feedback resolution is invalid"},
		{"regress event phase", func(r *Record) {
			r.RegressEvents = []RegressEvent{{FromPhase: "nonexistent"}}
		}, "regress event phase is invalid"},
		{"routing phase", func(r *Record) {
			r.RoutingTrace = []SkillRoutingEntry{{Phase: "nonexistent", Skill: "s"}}
		}, "routing phase is invalid"},
		{"branch provider", func(r *Record) {
			r.BranchPrepare = &BranchPrepare{Provider: "bitbucket"}
		}, "branch provider is invalid"},
		{"remote artifact github mr", func(r *Record) {
			r.RemoteArtifact = &RemoteArtifactVerification{Provider: "github", Kind: "mr"}
		}, "remote artifact identity is invalid"},
		{"remote artifact gitlab pr", func(r *Record) {
			r.RemoteArtifact = &RemoteArtifactVerification{Provider: "gitlab", Kind: "pr"}
		}, "remote artifact identity is invalid"},
		{"child validation verdict", func(r *Record) {
			r.ChildCycles = []ChildCycleRef{{ValidationVerdict: "maybe"}}
		}, "child validation verdict is invalid"},
		{"cleanup attempt", func(r *Record) {
			r.CleanupAttempt = &CleanupAttempt{Operation: "purge"}
		}, "cleanup attempt operation is invalid"},
		{"cleanup finish failure step", func(r *Record) {
			r.CleanupFinishFailure = &CleanupFinishFailure{Step: CleanupFailureStepClosePR}
		}, "cleanup finish failure step is invalid"},
		{"cleanup abandon failure step", func(r *Record) {
			r.CleanupAbandonFailure = &CleanupAbandonFailure{Step: "bogus"}
		}, "cleanup abandon failure step is invalid"},
		{"child create operation", func(r *Record) {
			op := validChildCreateOperation("11111111111111111111111111111111")
			op.Origin = "automatic"
			r.ChildCreateOperations = []ChildCreateOperation{op}
		}, "invalid child create operation identity"},
		{"issue create intent", func(r *Record) {
			r.IssueCreateIntent = &IssueCreateIntent{}
		}, "issue create intent "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := Record{SchemaVersion: SchemaVersion, ID: "io-shape", Phase: PhasePlan}
			test.mutate(&record)
			err := ValidateRecord(record)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, test.wantErr)
			}
		})
	}
}

func TestValidateRecordAcceptsEveryKnownVocabularyValue(t *testing.T) {
	base := func() Record {
		return Record{SchemaVersion: SchemaVersion, ID: "io-vocab", Phase: PhasePlan}
	}
	accepts := func(t *testing.T, name string, record Record) {
		t.Helper()
		if err := ValidateRecord(record); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}

	for _, phase := range Phases {
		record := base()
		record.RegressEvents = []RegressEvent{{FromPhase: phase}}
		record.RoutingTrace = []SkillRoutingEntry{{Phase: string(phase), Skill: "s"}}
		accepts(t, "phase "+string(phase), record)
	}
	for _, classification := range []string{"", "contract_change", "defect", "question", "noise", "valid_review", "stale_review", "rollout_evidence_missing", "environment_debt"} {
		record := base()
		record.Feedback = []FeedbackItem{{Classification: classification}}
		accepts(t, "classification "+classification, record)
	}
	for _, resolution := range []string{"", "valid-defect", "question-answered", "noise-dismissed"} {
		record := base()
		record.Feedback = []FeedbackItem{{Resolution: resolution}}
		accepts(t, "resolution "+resolution, record)
	}
	for _, verdict := range []string{"", "accepted", "rejected", "dropped"} {
		record := base()
		record.ChildCycles = []ChildCycleRef{{ValidationVerdict: verdict}}
		accepts(t, "child verdict "+verdict, record)
	}
	for _, provider := range []string{"github", "gitlab"} {
		record := base()
		record.BranchPrepare = &BranchPrepare{Provider: provider}
		accepts(t, "branch provider "+provider, record)
	}
	for _, artifact := range []RemoteArtifactVerification{{Provider: "github", Kind: "pr"}, {Provider: "gitlab", Kind: "mr"}} {
		record := base()
		record.RemoteArtifact = &artifact
		accepts(t, "remote artifact "+artifact.Provider+"/"+artifact.Kind, record)
	}
	for _, step := range []string{
		CleanupFailureStepWorkspaceProcessesStop, CleanupFailureStepWorktreeRemove,
		CleanupFailureStepBranchDelete, CleanupFailureStepRecordDelete,
	} {
		record := base()
		record.CleanupFinishFailure = &CleanupFinishFailure{Step: step}
		accepts(t, "finish step "+step, record)
	}
	for _, step := range []string{
		CleanupFailureStepApplying, CleanupFailureStepClosePR, CleanupFailureStepCloseIssue,
		CleanupFailureStepRemoteBranchDelete, CleanupFailureStepWorkspaceProcessesStop,
		CleanupFailureStepWorktreeRemove, CleanupFailureStepBranchDelete, CleanupFailureStepRecordDelete,
	} {
		record := base()
		record.CleanupAbandonFailure = &CleanupAbandonFailure{Step: step}
		accepts(t, "abandon step "+step, record)
	}
}

func TestValidateRecordReportsFirstFailureInValidatorOrder(t *testing.T) {
	record := Record{
		SchemaVersion: SchemaVersion,
		ID:            "io-order",
		Phase:         PhasePlan,
		Feedback:      []FeedbackItem{{Classification: "bogus"}},
		BranchPrepare: &BranchPrepare{Provider: "bitbucket"},
	}
	err := ValidateRecord(record)
	if err == nil || !strings.Contains(err.Error(), "feedback classification") {
		t.Fatalf("error = %v, want the earlier feedback failure", err)
	}
}

func TestKnownFeedbackVocabularies(t *testing.T) {
	for _, classification := range []string{"", "contract_change", "defect", "question", "noise", "valid_review", "stale_review", "rollout_evidence_missing", "environment_debt"} {
		if !KnownFeedbackClassification(classification) {
			t.Errorf("classification %q rejected", classification)
		}
	}
	for _, classification := range []string{"Defect", "unknown", " defect"} {
		if KnownFeedbackClassification(classification) {
			t.Errorf("classification %q accepted", classification)
		}
	}
	for _, resolution := range []string{"", "valid-defect", "question-answered", "noise-dismissed"} {
		if !KnownFeedbackResolution(resolution) {
			t.Errorf("resolution %q rejected", resolution)
		}
	}
	for _, resolution := range []string{"valid_defect", "defect", "unknown"} {
		if KnownFeedbackResolution(resolution) {
			t.Errorf("resolution %q accepted", resolution)
		}
	}
}
