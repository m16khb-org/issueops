package issueops

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/testsupport"
)

func TestMergeHandoffDeliveryObservationBindsEveryMutableIdentity(t *testing.T) {
	base := deliveryObservationFixture()
	base.InputAccepted = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceLauncherReceipt)

	tests := []struct {
		name string
		edit func(*issueopscontract.HandoffDeliveryObservation)
		want string
	}{
		{
			name: "stale restarted process cannot merge",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) {
				next.Target.ProcessIncarnation = "incarnation-2"
			},
			want: "delivery observation process identity changed",
		},
		{
			name: "changed prompt digest cannot reuse durable request",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) {
				next.PromptSHA256 = strings.Repeat("b", 64)
			},
			want: "delivery observation prompt digest changed",
		},
		{
			name: "changed launcher runtime cannot merge",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) {
				next.Launcher.RuntimeID = "runtime-2"
			},
			want: "delivery observation launcher identity changed",
		},
		{
			name: "changed source generation cannot merge",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) {
				next.SourceGeneration = 8
			},
			want: "delivery observation source generation changed",
		},
		{
			name: "different launcher duplicate attempt cannot merge",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) {
				next.Launcher.Name = "direct"
			},
			want: "delivery observation launcher identity changed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next := base
			test.edit(&next)
			_, decision := MergeHandoffDeliveryObservation(base, next)
			if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{test.want}) {
				t.Fatalf("decision=%+v", decision)
			}
		})
	}
}

func TestMergeHandoffDeliveryObservationKeepsStatesIndependentAndNonAuthoritative(t *testing.T) {
	base := deliveryObservationFixture()
	base.InputAccepted = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceLauncherReceipt)
	base.Ambiguous = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceTimeout)

	turnOnly := base
	turnOnly.NativeTurnObserved = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceNativeReceipt)
	merged, decision := MergeHandoffDeliveryObservation(base, turnOnly)
	if !decision.Accepted {
		t.Fatalf("turn observation must be evidence only: %+v", decision)
	}
	if merged.OwnerClaimed.Status != issueopscontract.HandoffDeliveryStateNotObserved ||
		merged.NativeTurnObserved.Status != issueopscontract.HandoffDeliveryStateObserved ||
		merged.Ambiguous.Status != issueopscontract.HandoffDeliveryStateObserved {
		t.Fatalf("states not independent: %+v", merged)
	}

	exactClaim := merged
	exactClaim.OwnerClaimed = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim)
	exactClaim.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{
		Claimed:    true,
		Generation: merged.SourceGeneration,
		Actor:      *merged.OwnerActor,
		ClaimedAt:  "2026-09-20T10:02:00Z",
	}
	claimed, decision := MergeHandoffDeliveryObservation(merged, exactClaim)
	if !decision.Accepted {
		t.Fatalf("claim observation must not grant authority or retry: %+v", decision)
	}
	if claimed.OwnerClaimed.Status != issueopscontract.HandoffDeliveryStateObserved {
		t.Fatalf("exact claim was not recorded: %+v", claimed.OwnerClaimed)
	}
}

func TestMergeHandoffDeliveryObservationRejectsBlindRetryAndInexactClaims(t *testing.T) {
	base := deliveryObservationFixture()
	base.Ambiguous = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceAgentPromptStalled)

	blindRetry := base
	blindRetry.Request.DurableID = "retry-1"
	_, decision := MergeHandoffDeliveryObservation(base, blindRetry)
	if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{"delivery observation request identity changed"}) {
		t.Fatalf("blind retry decision=%+v", decision)
	}

	cmuxBase := deliveryObservationFixture()
	cmuxBase.Launcher.Name = "cmux"
	cmuxBase.Launcher.RuntimeID = ""
	cmuxBase.Launcher.MachineID = ""
	cmuxBase.Launcher.ServerID = ""
	cmuxBase.Launcher.EndpointIncarnation = cmuxEndpointIncarnationFixture()
	cmuxBase.Target.WindowID = "window-1"
	cmuxBase.Target.WorkspaceID = "workspace-1"
	cmuxBase.Target.SurfaceID = "surface-1"
	cmuxBase.Target.CWD = testsupport.AbsPath("repo", "worktree")
	cmuxBase.NativeTurnObserved = deliveryState(issueopscontract.HandoffDeliveryStateNotObserved, "")
	inputFromCmux := cmuxBase
	inputFromCmux.InputAccepted = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceRawInput)
	inputFromCmux.NativeTurnObserved = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceRawInput)
	_, decision = MergeHandoffDeliveryObservation(cmuxBase, inputFromCmux)
	if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{"delivery observation native_turn_observed evidence is invalid"}) {
		t.Fatalf("cmux raw input cannot promote a turn: %+v", decision)
	}

	wrongOwner := base
	wrongOwner.OwnerClaimed = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim)
	wrongOwner.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{
		Claimed:    true,
		Generation: base.SourceGeneration,
		Actor: issueopscontract.NativeActor{
			Host: "codex", SessionID: "other", SessionProcess: &issueopscontract.NativeProcessReceipt{
				PID: 7070, StartedAt: "2026-09-20T10:00:00Z", Executable: "/usr/local/bin/codex",
			},
		},
		ClaimedAt: "2026-09-20T10:03:00Z",
	}
	_, decision = MergeHandoffDeliveryObservation(base, wrongOwner)
	if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{"delivery owner claim identity mismatch"}) {
		t.Fatalf("wrong owner decision=%+v", decision)
	}
}

func TestMergeHandoffDeliveryObservationModeSpecificRecoveryEvidence(t *testing.T) {
	tests := []struct {
		name   string
		base   func() issueopscontract.HandoffDeliveryObservation
		update func(issueopscontract.HandoffDeliveryObservation) issueopscontract.HandoffDeliveryObservation
		wantOK bool
		want   issueopscontract.HandoffDeliveryObservation
		reason string
	}{
		{
			name: "accepted response loss remains ambiguous without retry authority",
			base: deliveryObservationFixture,
			update: func(next issueopscontract.HandoffDeliveryObservation) issueopscontract.HandoffDeliveryObservation {
				next.InputAccepted = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceLauncherAccepted)
				next.Ambiguous = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceAcceptedResponseLost)
				return next
			},
			wantOK: true,
		},
		{
			name: "same durable request cannot carry changed process incarnation",
			base: deliveryObservationFixture,
			update: func(next issueopscontract.HandoffDeliveryObservation) issueopscontract.HandoffDeliveryObservation {
				next.Target.ProcessIncarnation = "incarnation-2"
				next.InputAccepted = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceLauncherAccepted)
				return next
			},
			reason: "delivery observation process identity changed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base := test.base()
			update := test.update(base)
			merged, decision := MergeHandoffDeliveryObservation(base, update)
			if test.wantOK {
				if !decision.Accepted {
					t.Fatalf("decision=%+v", decision)
				}
				if merged.OwnerClaimed.Status == issueopscontract.HandoffDeliveryStateObserved {
					t.Fatalf("delivery observation promoted owner claim without exact claim: %+v", merged)
				}
				return
			}
			if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{test.reason}) {
				t.Fatalf("decision=%+v", decision)
			}
		})
	}
}

func TestValidateHandoffDeliveryObservationFailClosedEnumsAndTimestamps(t *testing.T) {
	tests := []struct {
		name string
		edit func(*issueopscontract.HandoffDeliveryObservation)
		want string
	}{
		{
			name: "unknown launcher rejected",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.Launcher.Name = "other"
			},
			want: "delivery observation launcher is invalid",
		},
		{
			name: "unknown evidence rejected",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.InputAccepted = deliveryState(issueopscontract.HandoffDeliveryStateObserved, "freeform")
			},
			want: "delivery observation input_accepted evidence is invalid",
		},
		{
			name: "updated before created rejected",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.UpdatedAt = "2026-09-20T09:59:00Z"
			},
			want: "delivery observation timestamps are not monotonic",
		},
		{
			name: "observed after updated rejected",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.InputAccepted = issueopscontract.HandoffDeliveryState{
					Status:     issueopscontract.HandoffDeliveryStateObserved,
					ObservedAt: "2026-09-20T11:00:00Z",
					Evidence:   issueopscontract.HandoffDeliveryEvidenceLauncherReceipt,
				}
			},
			want: "delivery observation input_accepted timestamp is not monotonic",
		},
		{
			name: "Herdr wait state cannot prove native turn",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.Launcher.Name = issueopscontract.HandoffDeliveryLauncherHerdr
				observation.NativeTurnObserved = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceHerdrWaitState)
			},
			want: "delivery observation native_turn_observed evidence is invalid",
		},
		{
			name: "input accepted cannot use claim evidence",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.InputAccepted = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim)
			},
			want: "delivery observation input_accepted evidence is invalid",
		},
		{
			name: "native turn cannot use timeout evidence",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.NativeTurnObserved = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceTimeout)
			},
			want: "delivery observation native_turn_observed evidence is invalid",
		},
		{
			name: "input accepted cannot use unknown evidence",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.InputAccepted = deliveryState(issueopscontract.HandoffDeliveryStateObserved, "unknown_evidence")
			},
			want: "delivery observation input_accepted evidence is invalid",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation := deliveryObservationFixture()
			test.edit(&observation)
			if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != test.want {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestFoldHandoffDeliveryObservationsReadsAppendOnlyEvidence(t *testing.T) {
	first := deliveryObservationFixture()
	second := deliveryObservationFixture()
	second.InputAccepted = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceLauncherReceipt)
	third := deliveryObservationFixture()
	third.PromptSHA256 = strings.Repeat("b", 64)

	folded, decisions := FoldHandoffDeliveryObservations([]issueopscontract.HandoffDeliveryObservation{first, second, third})
	if len(decisions) != 3 || !decisions[0].Accepted || !decisions[1].Accepted || decisions[2].Accepted {
		t.Fatalf("decisions=%+v", decisions)
	}
	if !reflect.DeepEqual(decisions[2].RejectReasons, []string{"delivery observation prompt digest changed"}) {
		t.Fatalf("stale decision=%+v", decisions[2])
	}
	if got := folded[HandoffDeliveryFoldKey(first)]; got.InputAccepted.Status != issueopscontract.HandoffDeliveryStateObserved {
		t.Fatalf("folded=%+v", got)
	}
}

func TestHandoffDeliveryObservationUsesDurableRequestForRetryWithoutRetryAuthority(t *testing.T) {
	current := deliveryObservationFixture()
	next := current
	next.Ambiguous = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceAcceptedResponseLost)

	merged, decision := MergeHandoffDeliveryObservation(current, next)
	if !decision.Accepted {
		t.Fatalf("retry identity must be evidence only: %+v", decision)
	}
	if merged.Request.DurableID != current.Request.DurableID {
		t.Fatalf("merge must preserve the original durable request identity: %+v", merged.Request)
	}
}

func TestHandoffDeliveryRetryCannotChangeDurableRequest(t *testing.T) {
	current := deliveryObservationFixture()
	next := current
	next.Request.DurableID = "retry-request-2"
	_, decision := MergeHandoffDeliveryObservation(current, next)
	if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{"delivery observation request identity changed"}) {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestHandoffDeliveryExactClaimIsIdempotentAndOtherOwnerRejected(t *testing.T) {
	base := deliveryObservationFixture()
	claimed := base
	claimed.OwnerClaimed = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim)
	claimed.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{
		Claimed:    true,
		Generation: base.SourceGeneration,
		Actor:      *base.OwnerActor,
		ClaimedAt:  "2026-09-20T10:02:00Z",
	}
	merged, decision := MergeHandoffDeliveryObservation(base, claimed)
	if !decision.Accepted {
		t.Fatalf("exact claim observation must be evidence only: %+v", decision)
	}
	_, decision = MergeHandoffDeliveryObservation(merged, claimed)
	if !decision.Accepted {
		t.Fatalf("duplicate exact claim should be idempotent evidence: %+v", decision)
	}

	other := claimed
	other.OwnerClaim.Actor.SessionID = "other-session"
	_, decision = MergeHandoffDeliveryObservation(merged, other)
	if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{"delivery owner claim identity mismatch"}) {
		t.Fatalf("different owner decision=%+v", decision)
	}
}

func TestHandoffDeliveryManualClaimRequiresImmediateReseedGeneration(t *testing.T) {
	for _, test := range []struct {
		name       string
		manual     bool
		generation uint64
		wantValid  bool
	}{
		{name: "standard exact generation", generation: 7, wantValid: true},
		{name: "standard next generation", generation: 8},
		{name: "manual source generation", manual: true, generation: 7},
		{name: "manual immediate reseed generation", manual: true, generation: 8, wantValid: true},
		{name: "manual future generation", manual: true, generation: 9},
	} {
		t.Run(test.name, func(t *testing.T) {
			observation := deliveryObservationFixture()
			if test.manual {
				observation.AttemptID = "manual-direct:" + observation.LifecycleID + ":7:herdr:attempt-1"
				observation.LineageID = "manual-direct:" + observation.LineageID
			}
			observation.OwnerClaimed = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim)
			observation.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{
				Claimed: true, Generation: test.generation, Actor: *observation.OwnerActor, ClaimedAt: "2026-09-20T10:02:00Z",
			}
			err := ValidateHandoffDeliveryObservation(observation)
			if (err == nil) != test.wantValid {
				t.Fatalf("generation=%d valid=%v err=%v", test.generation, test.wantValid, err)
			}
		})
	}
}

func TestValidateHandoffDeliveryOwnerClaimConsistency(t *testing.T) {
	tests := []struct {
		name string
		edit func(*issueopscontract.HandoffDeliveryObservation)
		want string
	}{
		{
			name: "observed owner claim requires claimed payload",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.OwnerClaimed = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim)
			},
			want: "delivery owner claim state is inconsistent",
		},
		{
			name: "claimed payload requires observed state",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{
					Claimed:    true,
					Generation: observation.SourceGeneration,
					Actor:      *observation.OwnerActor,
					ClaimedAt:  "2026-09-20T10:02:00Z",
				}
			},
			want: "delivery owner claim state is inconsistent",
		},
		{
			name: "owner claim evidence must be issueops claim",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.OwnerClaimed = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceNativeReceipt)
				observation.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{
					Claimed:    true,
					Generation: observation.SourceGeneration,
					Actor:      *observation.OwnerActor,
					ClaimedAt:  "2026-09-20T10:02:00Z",
				}
			},
			want: "delivery observation owner_claimed evidence is invalid",
		},
		{
			name: "owner claim timestamp must be inside observation window",
			edit: func(observation *issueopscontract.HandoffDeliveryObservation) {
				observation.OwnerClaimed = deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceIssueOpsClaim)
				observation.OwnerClaim = issueopscontract.HandoffDeliveryOwnerClaim{
					Claimed:    true,
					Generation: observation.SourceGeneration,
					Actor:      *observation.OwnerActor,
					ClaimedAt:  "2026-09-20T11:02:00Z",
				}
			},
			want: "delivery owner claim timestamp is not monotonic",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation := deliveryObservationFixture()
			test.edit(&observation)
			if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != test.want {
				t.Fatalf("error=%v", err)
			}
		})
	}
}

func TestValidateHandoffDeliveryNotObservedRequiresEmptyPayload(t *testing.T) {
	for _, mutate := range []func(*issueopscontract.HandoffDeliveryObservation){
		func(observation *issueopscontract.HandoffDeliveryObservation) {
			observation.NativeTurnObserved.ObservedAt = "2026-09-20T10:01:00Z"
		},
		func(observation *issueopscontract.HandoffDeliveryObservation) {
			observation.NativeTurnObserved.Evidence = issueopscontract.HandoffDeliveryEvidenceNativeReceipt
		},
	} {
		observation := deliveryObservationFixture()
		mutate(&observation)
		if err := ValidateHandoffDeliveryObservation(observation); err == nil || !strings.Contains(err.Error(), "not_observed payload must be empty") {
			t.Fatalf("not_observed payload accepted: observation=%+v err=%v", observation.NativeTurnObserved, err)
		}
	}
}

func TestValidateHandoffDeliveryBoundsStateAndProcessTimestamps(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*issueopscontract.HandoffDeliveryObservation)
	}{
		{name: "state status", mutate: func(observation *issueopscontract.HandoffDeliveryObservation) {
			observation.NativeTurnObserved.Status = strings.Repeat("x", handoffDeliveryFieldLimit+1)
		}},
		{name: "state observed at", mutate: func(observation *issueopscontract.HandoffDeliveryObservation) {
			observation.InputAccepted.ObservedAt = strings.Repeat("x", handoffDeliveryFieldLimit+1)
		}},
		{name: "state evidence", mutate: func(observation *issueopscontract.HandoffDeliveryObservation) {
			observation.InputAccepted.Evidence = strings.Repeat("x", handoffDeliveryFieldLimit+1)
		}},
		{name: "process started at", mutate: func(observation *issueopscontract.HandoffDeliveryObservation) {
			observation.Target.Process.StartedAt = strings.Repeat("x", handoffDeliveryFieldLimit+1)
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation := deliveryObservationFixture()
			test.mutate(&observation)
			if err := ValidateHandoffDeliveryObservation(observation); err == nil || !strings.Contains(err.Error(), "too large") {
				t.Fatalf("unbounded %s accepted: %v", test.name, err)
			}
		})
	}
}

func TestMergeHandoffDeliveryObservationRejectsBackwardUpdateTimestamp(t *testing.T) {
	current := deliveryObservationFixture()
	next := current
	next.UpdatedAt = "2026-09-20T10:00:30Z"
	next.Ambiguous.ObservedAt = "2026-09-20T10:00:20Z"
	_, decision := MergeHandoffDeliveryObservation(current, next)
	if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{"delivery observation update timestamp moved backward"}) {
		t.Fatalf("decision=%+v", decision)
	}
}

func TestMergeHandoffDeliveryObservationBindsCreatedAtAndOwnerActor(t *testing.T) {
	current := deliveryObservationFixture()
	changedCreated := current
	changedCreated.CreatedAt = "2026-09-20T10:00:02Z"
	_, decision := MergeHandoffDeliveryObservation(current, changedCreated)
	if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{"delivery observation created timestamp changed"}) {
		t.Fatalf("created decision=%+v", decision)
	}

	changedOwner := current
	owner := *current.OwnerActor
	owner.SessionID = "other-session"
	changedOwner.OwnerActor = &owner
	_, decision = MergeHandoffDeliveryObservation(current, changedOwner)
	if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{"delivery observation owner identity changed"}) {
		t.Fatalf("owner decision=%+v", decision)
	}
}

func TestHandoffDeliveryObservationRejectsPromptTextAndTokenLeakage(t *testing.T) {
	observation := deliveryObservationFixture()
	encoded, err := json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"prompt_text", "old_token", "claim_token"} {
		if strings.Contains(strings.ToLower(string(encoded)), forbidden) {
			t.Fatalf("delivery observation leaked forbidden key %q: %s", forbidden, encoded)
		}
	}

	observation.PromptSHA256 = "not-a-digest"
	if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != "delivery observation prompt digest is invalid" {
		t.Fatalf("invalid digest error=%v", err)
	}
}

func deliveryObservationFixture() issueopscontract.HandoffDeliveryObservation {
	return issueopscontract.HandoffDeliveryObservation{
		SchemaVersion:  issueopscontract.HandoffDeliverySchemaVersion,
		AttemptID:      "attempt-1",
		LineageID:      "lineage-1",
		LifecycleID:    "io-delivery",
		PromptSHA256:   strings.Repeat("a", 64),
		MaterialSHA256: strings.Repeat("e", 64),
		Request: issueopscontract.HandoffDeliveryRequest{
			DurableID: "request-1",
		},
		Launcher: issueopscontract.HandoffDeliveryLauncher{
			Name: "herdr", Version: "1.4.200", Path: "/usr/local/bin/herdr", RuntimeID: "runtime-1", MachineID: "machine-1", ServerID: "server-1",
		},
		Target: issueopscontract.HandoffDeliveryTarget{
			TerminalID: "term-1", PaneID: "pane-1", ProcessIncarnation: "incarnation-1", Process: &issueopscontract.NativeProcessReceipt{
				PID: 8080, StartedAt: "2026-09-20T10:00:00Z", Executable: "/usr/local/bin/codex",
			},
		},
		OwnerActor: &issueopscontract.NativeActor{
			Host: "codex", SessionID: "session-1", SessionProcess: &issueopscontract.NativeProcessReceipt{
				PID: 8080, StartedAt: "2026-09-20T10:00:00Z", Executable: "/usr/local/bin/codex",
			},
		},
		SourceGeneration: 7,
		CreatedAt:        "2026-09-20T10:00:01Z",
		UpdatedAt:        "2026-09-20T10:05:00Z",
		Receipt: issueopscontract.HandoffDeliveryReceipt{
			Location: "audit/herdr/receipt-1.json", Digest: strings.Repeat("d", 64),
		},
		InputAccepted:      deliveryState(issueopscontract.HandoffDeliveryStateNotObserved, ""),
		NativeTurnObserved: deliveryState(issueopscontract.HandoffDeliveryStateNotObserved, ""),
		OwnerClaimed:       deliveryState(issueopscontract.HandoffDeliveryStateNotObserved, ""),
		Ambiguous:          deliveryState(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceAgentPromptStalled),
	}
}

func deliveryState(status, evidence string) issueopscontract.HandoffDeliveryState {
	state := issueopscontract.HandoffDeliveryState{Status: status}
	if evidence != "" {
		state.Evidence = evidence
		state.ObservedAt = "2026-09-20T10:01:00Z"
	}
	return state
}
