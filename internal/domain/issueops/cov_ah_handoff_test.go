package issueops

import (
	"errors"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

const (
	covHandoffUUID1 = "11111111-1111-4111-8111-111111111111"
	covHandoffUUID2 = "22222222-2222-4222-8222-222222222222"
)

type covHandoffObservation = issueopscontract.IssueOpsHandoffDeliveryObservation

func covHandoffLong() string { return strings.Repeat("x", handoffDeliveryFieldLimit+1) }

func covHandoffObserved(evidence string) issueopscontract.IssueOpsHandoffDeliveryState {
	return deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateObserved, evidence)
}

// covHandoffClaimedFixture returns a valid orca observation that carries an exact owner claim.
func covHandoffClaimedFixture() covHandoffObservation {
	observation := deliveryObservationFixture()
	observation.OwnerClaimed = covHandoffObserved(issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim)
	observation.OwnerClaim = issueopscontract.IssueOpsHandoffDeliveryOwnerClaim{
		Claimed: true, Generation: observation.SourceGeneration, Actor: *observation.OwnerActor, ClaimedAt: "2026-09-20T10:02:00Z",
	}
	return observation
}

func TestCovHandoffValidateRejectsMalformedObservation(t *testing.T) {
	tests := []struct {
		name string
		base func() covHandoffObservation
		edit func(*covHandoffObservation)
		want string
	}{
		{"schema version", deliveryObservationFixture, func(o *covHandoffObservation) { o.SchemaVersion = 2 }, "unsupported delivery observation schema"},
		{"blank attempt", deliveryObservationFixture, func(o *covHandoffObservation) { o.AttemptID = "  " }, "attempt_id is required"},
		{"oversized lineage", deliveryObservationFixture, func(o *covHandoffObservation) { o.LineageID = covHandoffLong() }, "lineage_id is required"},
		{"blank lifecycle", deliveryObservationFixture, func(o *covHandoffObservation) { o.LifecycleID = "" }, "lifecycle_id is required"},
		{"uppercase material digest", deliveryObservationFixture, func(o *covHandoffObservation) { o.MaterialSHA256 = strings.Repeat("E", 64) }, "material digest is invalid"},
		{"oversized durable id", deliveryObservationFixture, func(o *covHandoffObservation) { o.Request.DurableID = covHandoffLong() }, "durable request identity is invalid"},
		{"zero source generation", deliveryObservationFixture, func(o *covHandoffObservation) { o.SourceGeneration = 0 }, "source generation is required"},
		{"created_at unparsable", deliveryObservationFixture, func(o *covHandoffObservation) { o.CreatedAt = "yesterday" }, "created_at is invalid"},
		{"updated_at unparsable", deliveryObservationFixture, func(o *covHandoffObservation) { o.UpdatedAt = "tomorrow" }, "updated_at is invalid"},
		{"oversized timestamps", deliveryObservationFixture, func(o *covHandoffObservation) { o.UpdatedAt = covHandoffLong() }, "timestamps are too large"},
		{"empty receipt location", deliveryObservationFixture, func(o *covHandoffObservation) { o.Receipt.Location = " " }, "receipt is invalid"},
		{"bad receipt digest", deliveryObservationFixture, func(o *covHandoffObservation) { o.Receipt.Digest = "short" }, "receipt is invalid"},
		{"oversized receipt location", deliveryObservationFixture, func(o *covHandoffObservation) { o.Receipt.Location = covHandoffLong() }, "receipt location is too large"},
		{"missing launcher version", deliveryObservationFixture, func(o *covHandoffObservation) { o.Launcher.Version = "" }, "launcher version is required"},
		{"oversized launcher path", deliveryObservationFixture, func(o *covHandoffObservation) { o.Launcher.Path = covHandoffLong() }, "launcher path is too large"},
		{"missing runtime id", deliveryObservationFixture, func(o *covHandoffObservation) { o.Launcher.RuntimeID = "" }, "launcher runtime_id is required"},
		{"oversized machine id", deliveryObservationFixture, func(o *covHandoffObservation) { o.Launcher.MachineID = covHandoffLong() }, "launcher machine_id is too large"},
		{"missing server id", deliveryObservationFixture, func(o *covHandoffObservation) { o.Launcher.ServerID = " " }, "launcher server_id is required"},
		{"endpoint on non-cmux launcher", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.Launcher.EndpointIncarnation = cmuxDeliveryObservationFixture().Launcher.EndpointIncarnation
		}, "endpoint incarnation is only valid for cmux"},
		{"target without terminal or pane", deliveryObservationFixture, func(o *covHandoffObservation) { o.Target.TerminalID, o.Target.PaneID = "", " " }, "terminal or pane identity is required"},
		{"oversized target cwd", deliveryObservationFixture, func(o *covHandoffObservation) { o.Target.CWD = covHandoffLong() }, "cwd is too large"},
		{"prompt generation without baseline", deliveryObservationFixture, func(o *covHandoffObservation) {
			generation := uint64(1)
			o.Target.PromptGeneration = &generation
		}, "prompt receipt counters are incomplete"},
		{"zero prompt generation", deliveryObservationFixture, func(o *covHandoffObservation) {
			generation, baseline := uint64(0), uint64(4)
			o.Target.PromptGeneration, o.Target.BaselineWorkingSequence = &generation, &baseline
		}, "prompt generation is invalid"},
		{"process pid not positive", deliveryObservationFixture, func(o *covHandoffObservation) { o.Target.Process.PID = 0 }, "process identity is invalid"},
		{"oversized process executable", deliveryObservationFixture, func(o *covHandoffObservation) { o.Target.Process.Executable = covHandoffLong() }, "process identity is too large"},
		{"timing beyond ten minutes", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: 10*60*1000 + 1}
		}, "preflight_ms is out of bounds"},
		{"unknown state status", deliveryObservationFixture, func(o *covHandoffObservation) { o.InputAccepted.Status = "maybe" }, "input_accepted state is invalid"},
		{"observed state without evidence", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.InputAccepted = issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: "2026-09-20T10:01:00Z"}
		}, "input_accepted evidence is required"},
		{"observed state timestamp unparsable", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.InputAccepted = issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: "soon", Evidence: issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt}
		}, "input_accepted timestamp is invalid"},
		{"observed state before creation", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.InputAccepted = issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateObserved, ObservedAt: "2026-09-20T09:00:00Z", Evidence: issueopscontract.IssueOpsHandoffDeliveryEvidenceLauncherReceipt}
		}, "input_accepted timestamp is not monotonic"},
		{"call_staged with wrong evidence", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.CallStaged = covHandoffObserved(issueopscontract.IssueOpsHandoffDeliveryEvidenceTimeout)
		}, "call_staged evidence is invalid"},
		{"orca dispatch evidence for omo owner", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.ExpectedOwnerHost = "omo"
			o.InputAccepted = covHandoffObserved(issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatch)
		}, "input_accepted evidence is invalid"},
		{"ambiguous with claim evidence", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.Ambiguous = covHandoffObserved(issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim)
		}, "ambiguous evidence is invalid"},
		{"omo send failure for non-omo owner", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.Ambiguous = covHandoffObserved(issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendFailed)
		}, "ambiguous evidence is invalid"},
		{"oversized owner actor host", deliveryObservationFixture, func(o *covHandoffObservation) { o.OwnerActor.Host = covHandoffLong() }, "actor host is too large"},
		{"oversized owner actor session", deliveryObservationFixture, func(o *covHandoffObservation) { o.OwnerActor.SessionID = covHandoffLong() }, "actor session_id is too large"},
		{"oversized owner actor agent", deliveryObservationFixture, func(o *covHandoffObservation) { o.OwnerActor.AgentID = covHandoffLong() }, "actor agent_id is too large"},
		{"oversized owner actor process", deliveryObservationFixture, func(o *covHandoffObservation) { o.OwnerActor.SessionProcess.StartedAt = covHandoffLong() }, "actor process identity is too large"},
		{"oversized owner actor ancestry", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.OwnerActor.ProcessAncestry = []issueopscontract.NativeProcessReceipt{{PID: 1, StartedAt: "t", Executable: covHandoffLong()}}
		}, "actor process ancestry is too large"},
		{"owner actor with unknown host", deliveryObservationFixture, func(o *covHandoffObservation) { o.OwnerActor.Host = "bash" }, "native actor host must be"},
		{"owner actor without session", deliveryObservationFixture, func(o *covHandoffObservation) { o.OwnerActor.SessionID = "" }, "native actor session_id is required"},
		{"unknown expected owner host", deliveryObservationFixture, func(o *covHandoffObservation) { o.ExpectedOwnerHost = "vim" }, "expected owner host is invalid"},
		{"no observed evidence at all", deliveryObservationFixture, func(o *covHandoffObservation) {
			o.Ambiguous = deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateNotObserved, "")
		}, "requires at least one observed evidence state"},
		{"owner claim timestamp oversized", covHandoffClaimedFixture, func(o *covHandoffObservation) { o.OwnerClaim.ClaimedAt = covHandoffLong() }, "owner claim timestamp is too large"},
		{"owner claim timestamp unparsable", covHandoffClaimedFixture, func(o *covHandoffObservation) { o.OwnerClaim.ClaimedAt = "later" }, "owner claim timestamp is invalid"},
		{"owner claim actor oversized", covHandoffClaimedFixture, func(o *covHandoffObservation) { o.OwnerClaim.Actor.SessionID = covHandoffLong() }, "actor session_id is too large"},
		{"owner claim for wrong generation", covHandoffClaimedFixture, func(o *covHandoffObservation) { o.OwnerClaim.Generation = o.SourceGeneration + 1 }, "owner claim identity mismatch"},
		{"owner claim by another actor", covHandoffClaimedFixture, func(o *covHandoffObservation) { o.OwnerClaim.Actor.SessionID = "session-other" }, "owner claim identity mismatch"},
		{"owner claim without owner actor", covHandoffClaimedFixture, func(o *covHandoffObservation) { o.OwnerActor = nil }, "owner claim identity mismatch"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation := test.base()
			test.edit(&observation)
			err := ValidateHandoffDeliveryObservation(observation)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want error containing %q, got %v", test.want, err)
			}
		})
	}
	if err := ValidateHandoffDeliveryObservation(covHandoffClaimedFixture()); err != nil {
		t.Fatalf("exact owner claim fixture must be valid: %v", err)
	}
}

func TestCovHandoffValidateCmuxLauncherAndTargetIdentity(t *testing.T) {
	tests := []struct {
		name string
		edit func(*covHandoffObservation)
		want string
	}{
		{"runtime identity invented", func(o *covHandoffObservation) { o.Launcher.RuntimeID = "runtime-1" }, "cannot invent runtime, machine, or server identity"},
		{"machine identity invented", func(o *covHandoffObservation) { o.Launcher.MachineID = "machine-1" }, "cannot invent runtime, machine, or server identity"},
		{"missing endpoint incarnation", func(o *covHandoffObservation) { o.Launcher.EndpointIncarnation = nil }, "socket endpoint incarnation is required"},
		{"endpoint not unix socket", func(o *covHandoffObservation) { o.Launcher.EndpointIncarnation.Kind = "tcp" }, "socket endpoint incarnation is invalid"},
		{"endpoint parent mismatch", func(o *covHandoffObservation) { o.Launcher.EndpointIncarnation.ParentPath = "/private/var" }, "socket endpoint incarnation is invalid"},
		{"endpoint without inode", func(o *covHandoffObservation) { o.Launcher.EndpointIncarnation.Inode = 0 }, "socket endpoint incarnation is invalid"},
		{"endpoint oversized path", func(o *covHandoffObservation) {
			o.Launcher.EndpointIncarnation.Path = "/private/tmp/" + covHandoffLong()
		}, "socket endpoint incarnation is too large"},
		{"endpoint socket mode too open", func(o *covHandoffObservation) { o.Launcher.EndpointIncarnation.Mode = 0o660 }, "socket endpoint permissions are unsafe"},
		{"endpoint parent writable without sticky", func(o *covHandoffObservation) { o.Launcher.EndpointIncarnation.ParentMode = 0o777 }, "socket endpoint permissions are unsafe"},
		{"missing window", func(o *covHandoffObservation) { o.Target.WindowID = "" }, "requires exact window and cwd identity"},
		{"missing cwd", func(o *covHandoffObservation) { o.Target.CWD = " " }, "requires exact window and cwd identity"},
		{"input receipt without surface", func(o *covHandoffObservation) {
			o.Target.WorkspaceID = "workspace-1"
			o.InputAccepted = deliveryStateAt(issueopscontract.IssueOpsHandoffDeliveryStateObserved, issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput, "2026-09-20T10:00:01Z")
		}, "input receipt requires exact workspace and surface identity"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observation := cmuxDeliveryObservationFixture()
			test.edit(&observation)
			err := ValidateHandoffDeliveryObservation(observation)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("want error containing %q, got %v", test.want, err)
			}
		})
	}
	sticky := cmuxDeliveryObservationFixture()
	sticky.Launcher.EndpointIncarnation.ParentMode = 0o700
	if err := ValidateHandoffDeliveryObservation(sticky); err != nil {
		t.Fatalf("private parent directory must be accepted: %v", err)
	}
}

func TestCovHandoffFoldRejectsUnkeyedAndInvalidObservations(t *testing.T) {
	valid := deliveryObservationFixture()
	unkeyed := deliveryObservationFixture()
	unkeyed.LineageID = " "
	invalid := deliveryObservationFixture()
	invalid.LifecycleID = "io-other"
	invalid.SchemaVersion = 9
	folded, decisions := FoldHandoffDeliveryObservations([]covHandoffObservation{valid, unkeyed, invalid})
	if len(folded) != 1 || len(decisions) != 3 {
		t.Fatalf("folded=%d decisions=%+v", len(folded), decisions)
	}
	if !decisions[0].Accepted {
		t.Fatalf("valid observation rejected: %+v", decisions[0])
	}
	if decisions[1].Accepted || decisions[1].RejectReasons[0] != "delivery observation lineage identity is required" {
		t.Fatalf("unkeyed decision=%+v", decisions[1])
	}
	if decisions[2].Accepted || decisions[2].RejectReasons[0] != "unsupported delivery observation schema" {
		t.Fatalf("invalid decision=%+v", decisions[2])
	}
	if got := HandoffDeliveryFoldKey(unkeyed); got != "" {
		t.Fatalf("blank lineage produced key %q", got)
	}
	if got := HandoffDeliveryFoldKey(valid); got != "io-delivery\x00lineage-1" {
		t.Fatalf("key=%q", got)
	}
}

func TestCovHandoffMergeRejectsIdentityDrift(t *testing.T) {
	tests := []struct {
		name string
		edit func(*covHandoffObservation)
		want string
	}{
		{"attempt", func(o *covHandoffObservation) { o.AttemptID = "attempt-2" }, "attempt identity changed"},
		{"prompt digest", func(o *covHandoffObservation) { o.PromptSHA256 = strings.Repeat("b", 64) }, "prompt digest changed"},
		{"material digest", func(o *covHandoffObservation) { o.MaterialSHA256 = strings.Repeat("c", 64) }, "material digest changed"},
		{"durable request", func(o *covHandoffObservation) { o.Request.DurableID = "request-2" }, "request identity changed"},
		{"created at", func(o *covHandoffObservation) { o.CreatedAt = "2026-09-20T10:00:02Z" }, "created timestamp changed"},
		{"source generation", func(o *covHandoffObservation) { o.SourceGeneration = 8 }, "source generation changed"},
		{"launcher", func(o *covHandoffObservation) { o.Launcher.Version = "9.9.9" }, "launcher identity changed"},
		{"terminal", func(o *covHandoffObservation) { o.Target.TerminalID = "term-2" }, "process identity changed"},
		{"process incarnation", func(o *covHandoffObservation) { o.Target.ProcessIncarnation = "incarnation-2" }, "process identity changed"},
		{"process receipt", func(o *covHandoffObservation) { o.Target.Process.PID = 9090 }, "process identity changed"},
		{"prompt generation", func(o *covHandoffObservation) {
			generation := uint64(2)
			o.Target.PromptGeneration = &generation
		}, "process identity changed"},
		{"owner actor", func(o *covHandoffObservation) { o.OwnerActor.SessionID = "session-2" }, "owner identity changed"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			current := deliveryObservationFixture()
			next := deliveryObservationFixture()
			if test.name == "prompt generation" {
				generation, baseline := uint64(1), uint64(5)
				current.Target.PromptGeneration, current.Target.BaselineWorkingSequence = &generation, &baseline
				nextBaseline := baseline
				next.Target.BaselineWorkingSequence = &nextBaseline
			}
			test.edit(&next)
			merged, decision := MergeHandoffDeliveryObservation(current, next)
			if decision.Accepted || len(decision.RejectReasons) != 1 || !strings.Contains(decision.RejectReasons[0], test.want) {
				t.Fatalf("decision=%+v", decision)
			}
			if merged.UpdatedAt != current.UpdatedAt || merged.AttemptID != current.AttemptID {
				t.Fatalf("rejected merge must return the current observation unchanged: %+v", merged)
			}
		})
	}
}

func TestCovHandoffMergeRejectsDriftOnlyDetectedByLaterObservation(t *testing.T) {
	t.Run("lineage and lifecycle changes are caught before merge", func(t *testing.T) {
		current := deliveryObservationFixture()
		next := deliveryObservationFixture()
		next.LineageID = "lineage-2"
		_, decision := MergeHandoffDeliveryObservation(current, next)
		if decision.Accepted || decision.RejectReasons[0] != "delivery observation lineage identity changed" {
			t.Fatalf("lineage decision=%+v", decision)
		}
		next = deliveryObservationFixture()
		next.LifecycleID = "io-other"
		_, decision = MergeHandoffDeliveryObservation(current, next)
		if decision.Accepted || decision.RejectReasons[0] != "delivery observation lifecycle identity changed" {
			t.Fatalf("lifecycle decision=%+v", decision)
		}
	})
	t.Run("invalid current observation blocks merge", func(t *testing.T) {
		current := deliveryObservationFixture()
		current.SchemaVersion = 0
		_, decision := MergeHandoffDeliveryObservation(current, deliveryObservationFixture())
		if decision.Accepted || decision.RejectReasons[0] != "unsupported delivery observation schema" {
			t.Fatalf("decision=%+v", decision)
		}
	})
	t.Run("invalid next observation blocks merge", func(t *testing.T) {
		next := deliveryObservationFixture()
		next.Receipt.Digest = "bad"
		_, decision := MergeHandoffDeliveryObservation(deliveryObservationFixture(), next)
		if decision.Accepted || decision.RejectReasons[0] != "delivery observation receipt is invalid" {
			t.Fatalf("decision=%+v", decision)
		}
	})
	t.Run("timing drift", func(t *testing.T) {
		current := deliveryObservationFixture()
		current.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{InputSendMS: 10}
		next := deliveryObservationFixture()
		next.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{InputSendMS: 11}
		_, decision := MergeHandoffDeliveryObservation(current, next)
		if decision.Accepted || decision.RejectReasons[0] != "delivery observation timing changed" {
			t.Fatalf("decision=%+v", decision)
		}
	})
	t.Run("cmux target drift", func(t *testing.T) {
		current := cmuxDeliveryObservationFixture()
		next := cmuxDeliveryObservationFixture()
		next.Target.WindowID = "window-2"
		_, decision := MergeHandoffDeliveryObservation(current, next)
		if decision.Accepted || decision.RejectReasons[0] != "delivery observation target identity changed" {
			t.Fatalf("decision=%+v", decision)
		}
	})
}

func TestCovHandoffMergeAdoptsMissingTargetTimingReceiptAndOwner(t *testing.T) {
	current := deliveryObservationFixture()
	current.Target = issueopscontract.IssueOpsHandoffDeliveryTarget{TerminalID: "term-1", PaneID: "pane-1"}
	current.OwnerActor = nil
	current.Request.DurableID = ""
	current.Timing = nil

	next := deliveryObservationFixture()
	generation, baseline := uint64(3), uint64(11)
	next.Target.PromptGeneration, next.Target.BaselineWorkingSequence = &generation, &baseline
	next.Target.PaneID = "pane-rotated"
	next.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: 5, InputSendMS: 7}
	next.Receipt = issueopscontract.IssueOpsHandoffDeliveryReceipt{Location: "audit/orca/receipt-2.json", Digest: strings.Repeat("f", 64)}
	next.UpdatedAt = "2026-09-20T10:06:00Z"

	merged, decision := MergeHandoffDeliveryObservation(current, next)
	if !decision.Accepted {
		t.Fatalf("decision=%+v", decision)
	}
	if merged.Request.DurableID != "request-1" || merged.OwnerActor == nil || merged.OwnerActor.SessionID != "session-1" {
		t.Fatalf("durable id / owner not adopted: %+v %+v", merged.Request, merged.OwnerActor)
	}
	if merged.OwnerActor == next.OwnerActor {
		t.Fatal("owner actor must be copied, not aliased")
	}
	if merged.Target.PaneID != "pane-rotated" || merged.Target.ProcessIncarnation != "incarnation-1" ||
		merged.Target.PromptGeneration == nil || *merged.Target.PromptGeneration != 3 ||
		merged.Target.BaselineWorkingSequence == nil || *merged.Target.BaselineWorkingSequence != 11 ||
		merged.Target.Process == nil || merged.Target.Process.PID != 8080 {
		t.Fatalf("target not enriched: %+v", merged.Target)
	}
	if merged.Target.PromptGeneration == next.Target.PromptGeneration || merged.Target.Process == next.Target.Process {
		t.Fatal("target pointers must be copied, not aliased")
	}
	if merged.Timing == nil || merged.Timing.PreflightMS != 5 || merged.Timing.InputSendMS != 7 {
		t.Fatalf("timing not adopted: %+v", merged.Timing)
	}
	if merged.Timing == next.Timing {
		t.Fatal("timing must be copied, not aliased")
	}
	if merged.Receipt.Location != "audit/orca/receipt-2.json" || merged.UpdatedAt != "2026-09-20T10:06:00Z" {
		t.Fatalf("receipt/updated_at not advanced: %+v %s", merged.Receipt, merged.UpdatedAt)
	}
}

func TestCovHandoffMergeFillsOnlyZeroTimingFields(t *testing.T) {
	current := deliveryObservationFixture()
	current.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: 1}
	next := deliveryObservationFixture()
	next.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: 1, WorkspaceCreateMS: 2, TargetResolveMS: 3, InputSendMS: 4, ReceiverReceiptMS: 5}
	merged, decision := MergeHandoffDeliveryObservation(current, next)
	if !decision.Accepted {
		t.Fatalf("decision=%+v", decision)
	}
	if *merged.Timing != *next.Timing {
		t.Fatalf("timing=%+v", *merged.Timing)
	}
	if *current.Timing != (issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: 1}) {
		t.Fatalf("merge mutated the current timing: %+v", *current.Timing)
	}
	// A later observation without timing must keep the established timing.
	again, decision := MergeHandoffDeliveryObservation(merged, deliveryObservationFixture())
	if !decision.Accepted || again.Timing == nil || again.Timing.ReceiverReceiptMS != 5 {
		t.Fatalf("timing lost: %+v %+v", again.Timing, decision)
	}
}

func TestCovHandoffMergeCmuxRawInputCannotProveNativeTurn(t *testing.T) {
	current := cmuxDeliveryObservationFixture()
	next := cmuxDeliveryObservationFixture()
	next.UpdatedAt = "2026-09-20T10:00:05Z"
	next.NativeTurnObserved = deliveryStateAt(issueopscontract.IssueOpsHandoffDeliveryStateObserved, issueopscontract.IssueOpsHandoffDeliveryEvidenceNativeReceipt, "2026-09-20T10:00:03Z")
	if _, decision := MergeHandoffDeliveryObservation(current, next); !decision.Accepted {
		t.Fatalf("native receipt on cmux must merge: %+v", decision)
	}
}

func TestCovHandoffMergeOwnerClaimRequiresMatchingActor(t *testing.T) {
	current := deliveryObservationFixture()
	next := covHandoffClaimedFixture()
	next.OwnerActor = nil
	next.OwnerClaim.Actor = *current.OwnerActor
	// next carries no owner actor of its own, so it is invalid on its own.
	if _, decision := MergeHandoffDeliveryObservation(current, next); decision.Accepted {
		t.Fatalf("claim without owner actor accepted: %+v", decision)
	}
	next = covHandoffClaimedFixture()
	merged, decision := MergeHandoffDeliveryObservation(current, next)
	if !decision.Accepted || !merged.OwnerClaim.Claimed || merged.OwnerClaimed.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved {
		t.Fatalf("exact claim rejected: %+v %+v", merged.OwnerClaim, decision)
	}
}

func TestCovHandoffManualOwnerClaimGenerationRule(t *testing.T) {
	manual := cmuxDeliveryObservationFixture()
	manual.AttemptID = handoffDeliveryManualLineagePrefix + manual.LifecycleID + ":7:cmux:abc"
	actor := issueopscontract.NativeActor{Host: "codex", SessionID: "s", SessionProcess: &issueopscontract.NativeProcessReceipt{PID: 5, StartedAt: "t", Executable: "/bin/codex"}}
	good := issueopscontract.IssueOpsHandoffDeliveryOwnerClaim{Claimed: true, Generation: 8, Actor: actor, ClaimedAt: "2026-09-20T10:00:02Z"}
	manual.OwnerActor = &actor
	if err := validateHandoffDeliveryOwnerClaim(manual, good); err != nil {
		t.Fatalf("immediate reseed generation rejected: %v", err)
	}
	same := good
	same.Generation = 7
	if err := validateHandoffDeliveryOwnerClaim(manual, same); err == nil {
		t.Fatal("manual claim in the source generation accepted")
	}
	late := good
	late.Generation = 9
	if err := validateHandoffDeliveryOwnerClaim(manual, late); err == nil {
		t.Fatal("manual claim skipping a generation accepted")
	}
	first := manual
	first.SourceGeneration = 0
	zero := good
	zero.Generation = 1
	if err := validateHandoffDeliveryOwnerClaim(first, zero); err == nil {
		t.Fatal("manual claim for generation 1 accepted")
	}
}

func TestCovHandoffAttemptIdentityHelpers(t *testing.T) {
	attempt := DeliveryAttempt{
		OperationID: " op ", Stage: "dispatch", LifecycleID: " io-1 ", Host: " omo ",
		PromptSHA256: " " + strings.Repeat("a", 64) + " ", MaterialSHA256: strings.Repeat("b", 64), SourceGeneration: 4,
	}
	if got := attempt.AttemptID("prompt"); got != "op:dispatch:prompt" {
		t.Fatalf("attempt id=%q", got)
	}
	wantLineage := "generation:4:prompt:" + strings.Repeat("a", 64) + ":material:" + strings.Repeat("b", 64) + ":call:prompt"
	if got := attempt.LineageID(" prompt "); got != wantLineage {
		t.Fatalf("lineage=%q", got)
	}
	launcher := issueopscontract.IssueOpsHandoffDeliveryLauncher{Name: "orca"}
	probe := attempt.Probe(launcher, "dispatch")
	if probe.LifecycleID != "io-1" || probe.ExpectedOwnerHost != "omo" || probe.PromptSHA256 != strings.Repeat("a", 64) ||
		probe.SourceGeneration != 4 || probe.Launcher != launcher || probe.LineageID != attempt.LineageID("dispatch") {
		t.Fatalf("probe=%+v", probe)
	}

	for _, test := range []struct {
		stage     string
		hasLaunch bool
		want      bool
	}{{"dispatch", true, true}, {"dispatch", false, false}, {"preflight", true, false}} {
		if got := (DeliveryAttempt{Stage: test.stage, HasLaunch: test.hasLaunch}).IsDelivery(); got != test.want {
			t.Fatalf("IsDelivery(%+v)=%v", test, got)
		}
	}

	for _, test := range []struct {
		host, stage string
		failure     *DeliveryFailure
		want        string
	}{
		{"omo", "dispatch", &DeliveryFailure{CallPhase: "terminal_send"}, "prompt"},
		{"codex", "dispatch", &DeliveryFailure{CallPhase: "terminal_send"}, "prompt"},
		{"omo", "dispatch", &DeliveryFailure{CallPhase: "other"}, "dispatch"},
		{"omo", "dispatch", nil, "dispatch"},
		{"codex", "dispatch", nil, "dispatch"},
		{"omo", "preflight", nil, "preflight"},
		{"codex", "assign", nil, "assign"},
	} {
		if got := (DeliveryAttempt{Host: test.host, Stage: test.stage}).CallKind(test.failure); got != test.want {
			t.Fatalf("CallKind(%+v)=%q", test, got)
		}
	}
}

func TestCovHandoffRetryRequestIDsValidatedPerCall(t *testing.T) {
	for _, test := range []struct {
		name            string
		dispatch, input string
		want            string
	}{
		{"both empty", "", "", ""},
		{"both valid", covHandoffUUID1, covHandoffUUID2, ""},
		{"bad dispatch", "nope", covHandoffUUID2, "Orca dispatch retry request ID is invalid"},
		{"bad prompt", covHandoffUUID1, "nope", "Orca prompt retry request ID is invalid"},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateDeliveryRetryRequestIDs(DeliveryAttempt{RetryRequestID: test.dispatch, PromptRetryRequestID: test.input})
			if (test.want == "") != (err == nil) || err != nil && err.Error() != test.want {
				t.Fatalf("err=%v", err)
			}
		})
	}
	if _, err := DeliveryDurableRequestID("", "bad", true, "prompt"); err == nil || err.Error() != "Orca prompt delivery observation has an invalid durable request ID" {
		t.Fatalf("invalid observed id err=%v", err)
	}
	if _, err := DeliveryDurableRequestID(covHandoffUUID1, covHandoffUUID2, true, "prompt"); err == nil || err.Error() != "Orca prompt retry request has no exact delivery observation" {
		t.Fatalf("mismatched retry err=%v", err)
	}
}

func TestCovHandoffResponseIdentityRejection(t *testing.T) {
	tests := []struct {
		name    string
		request DeliveryAttempt
		failure *DeliveryFailure
		want    bool
	}{
		{"no typed failure", DeliveryAttempt{}, nil, false},
		{"identity mismatch code without observed id", DeliveryAttempt{}, &DeliveryFailure{Code: "dispatch_request_identity_mismatch"}, true},
		{"prompt mismatch code without observed id", DeliveryAttempt{}, &DeliveryFailure{Code: "terminal_prompt_request_identity_mismatch"}, true},
		{"receipt invalid code without observed id", DeliveryAttempt{}, &DeliveryFailure{Code: "terminal_prompt_receipt_invalid"}, true},
		{"unrelated code without observed id", DeliveryAttempt{}, &DeliveryFailure{Code: "timeout"}, false},
		{"invalid observed id", DeliveryAttempt{}, &DeliveryFailure{OrchestrationRequestID: "nope"}, true},
		{"observed id differs from sealed", DeliveryAttempt{RetryRequestID: covHandoffUUID1}, &DeliveryFailure{OrchestrationRequestID: covHandoffUUID2}, true},
		{"observed id equals sealed", DeliveryAttempt{RetryRequestID: covHandoffUUID1}, &DeliveryFailure{OrchestrationRequestID: covHandoffUUID1}, false},
		{"observed id without seal", DeliveryAttempt{}, &DeliveryFailure{OrchestrationRequestID: covHandoffUUID1}, false},
		{"terminal send checks prompt seal", DeliveryAttempt{RetryRequestID: covHandoffUUID1, PromptRetryRequestID: covHandoffUUID2}, &DeliveryFailure{CallPhase: "terminal_send", OrchestrationRequestID: covHandoffUUID1}, true},
		{"terminal send matching prompt seal", DeliveryAttempt{RetryRequestID: covHandoffUUID1, PromptRetryRequestID: covHandoffUUID2}, &DeliveryFailure{CallPhase: "terminal_send", OrchestrationRequestID: covHandoffUUID2}, false},
		{"terminal send invalid dispatch id", DeliveryAttempt{}, &DeliveryFailure{CallPhase: "terminal_send", DispatchRequestID: "nope"}, true},
		{"terminal send dispatch id differs from seal", DeliveryAttempt{RetryRequestID: covHandoffUUID1}, &DeliveryFailure{CallPhase: "terminal_send", DispatchRequestID: covHandoffUUID2}, true},
		{"terminal send dispatch id matches seal", DeliveryAttempt{RetryRequestID: covHandoffUUID1}, &DeliveryFailure{CallPhase: "terminal_send", DispatchRequestID: covHandoffUUID1}, false},
		{"terminal send receipt invalid", DeliveryAttempt{}, &DeliveryFailure{CallPhase: "terminal_send", Code: "terminal_prompt_receipt_invalid"}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := DeliveryResponseIdentityRejected(test.request, test.failure); got != test.want {
				t.Fatalf("got %v want %v", got, test.want)
			}
		})
	}
	for _, test := range []struct{ sealed, observed, want string }{
		{"", covHandoffUUID1, covHandoffUUID1},
		{covHandoffUUID1, " " + covHandoffUUID1 + " ", covHandoffUUID1},
		{covHandoffUUID1, covHandoffUUID2, ""},
		{"", "nope", ""},
		{"", "", ""},
	} {
		if got := DeliveryAcceptedResponseID(test.sealed, test.observed); got != test.want {
			t.Fatalf("DeliveryAcceptedResponseID(%q,%q)=%q", test.sealed, test.observed, got)
		}
	}
}

func TestCovHandoffDispatchReceiptMustMatchDurableIdentity(t *testing.T) {
	receipt := DeliveryReceiptIdentity{RequestID: covHandoffUUID1, TaskID: "task", DispatchID: "dispatch", TerminalID: "term", TerminalHandle: "handle"}
	if err := ValidateDeliveryDispatchReceipt(receipt, " task ", "term", "handle ", covHandoffUUID1); err != nil {
		t.Fatalf("exact receipt rejected: %v", err)
	}
	for name, mutate := range map[string]func(*DeliveryReceiptIdentity){
		"request":  func(r *DeliveryReceiptIdentity) { r.RequestID = covHandoffUUID2 },
		"task":     func(r *DeliveryReceiptIdentity) { r.TaskID = "other" },
		"dispatch": func(r *DeliveryReceiptIdentity) { r.DispatchID = "  " },
		"terminal": func(r *DeliveryReceiptIdentity) { r.TerminalID = "other" },
		"handle":   func(r *DeliveryReceiptIdentity) { r.TerminalHandle = "other" },
	} {
		t.Run(name, func(t *testing.T) {
			mutated := receipt
			mutate(&mutated)
			err := ValidateDeliveryDispatchReceipt(mutated, "task", "term", "handle", covHandoffUUID1)
			if err == nil || err.Error() != "Orca dispatch candidate does not match the durable request and delivery terminal identity" {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestCovHandoffRecoverDeliveryObservation(t *testing.T) {
	stored := deliveryObservationFixture()
	probe := deliveryObservationFixture()
	probe.Request = issueopscontract.IssueOpsHandoffDeliveryRequest{}

	t.Run("exact match is returned", func(t *testing.T) {
		got, ok, err := RecoverDeliveryObservation(probe, "term-1", []covHandoffObservation{stored}, nil)
		if err != nil || !ok || got.Request.DurableID != "request-1" {
			t.Fatalf("got=%+v ok=%v err=%v", got, ok, err)
		}
	})
	t.Run("observation without terminal binds to any terminal", func(t *testing.T) {
		unbound := stored
		unbound.Target = issueopscontract.IssueOpsHandoffDeliveryTarget{PaneID: "pane-1"}
		_, ok, err := RecoverDeliveryObservation(probe, "term-9", []covHandoffObservation{unbound}, nil)
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("no stored observation", func(t *testing.T) {
		_, ok, err := RecoverDeliveryObservation(probe, "term-1", nil, nil)
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("probe without lineage identity", func(t *testing.T) {
		blank := probe
		blank.LineageID = ""
		_, ok, err := RecoverDeliveryObservation(blank, "term-1", []covHandoffObservation{stored}, nil)
		if err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("lookup error passes through", func(t *testing.T) {
		lookup := errors.New("ledger unreadable")
		_, ok, err := RecoverDeliveryObservation(probe, "term-1", []covHandoffObservation{stored}, lookup)
		if ok || !errors.Is(err, lookup) || strings.Contains(err.Error(), "rejected") {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("rejected evidence names the reason", func(t *testing.T) {
		broken := stored
		broken.Receipt.Digest = "bad"
		_, ok, err := RecoverDeliveryObservation(probe, "term-1", []covHandoffObservation{broken}, nil)
		if ok || err == nil || err.Error() != "handoff delivery recovery evidence rejected: delivery observation receipt is invalid" {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	t.Run("rejected evidence wraps lookup error", func(t *testing.T) {
		broken := stored
		broken.Receipt.Digest = "bad"
		lookup := errors.New("ledger unreadable")
		_, ok, err := RecoverDeliveryObservation(probe, "term-1", []covHandoffObservation{broken}, lookup)
		if ok || !errors.Is(err, lookup) || !strings.Contains(err.Error(), "recovery evidence rejected") {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
	})
	for name, mutate := range map[string]func(*covHandoffObservation, *string){
		"prompt digest":   func(p *covHandoffObservation, _ *string) { p.PromptSHA256 = strings.Repeat("b", 64) },
		"material digest": func(p *covHandoffObservation, _ *string) { p.MaterialSHA256 = strings.Repeat("c", 64) },
		"generation":      func(p *covHandoffObservation, _ *string) { p.SourceGeneration = 8 },
		"launcher":        func(p *covHandoffObservation, _ *string) { p.Launcher.Path = "/other/orca" },
		"owner host":      func(p *covHandoffObservation, _ *string) { p.ExpectedOwnerHost = "claude" },
		"terminal":        func(_ *covHandoffObservation, terminal *string) { *terminal = "term-2" },
	} {
		t.Run("conflict on "+name, func(t *testing.T) {
			conflicting := probe
			terminal := "term-1"
			mutate(&conflicting, &terminal)
			_, ok, err := RecoverDeliveryObservation(conflicting, terminal, []covHandoffObservation{stored}, nil)
			if ok || err == nil || err.Error() != "handoff delivery recovery evidence conflicts with current request identity" {
				t.Fatalf("ok=%v err=%v", ok, err)
			}
		})
	}
}

func TestCovHandoffDeliveryRequestStatus(t *testing.T) {
	for _, status := range []string{"completed", "pending", " pending "} {
		got, err := DeliveryRequestStatus(covHandoffUUID1, "orchestration.dispatch", "rt", covHandoffUUID1, "orchestration.dispatch", " rt ", status)
		if err != nil || got != strings.TrimSpace(status) {
			t.Fatalf("status %q => %q, %v", status, got, err)
		}
	}
	mismatch := "Orca request observation identity mismatch"
	for name, args := range map[string][3]string{
		"request": {covHandoffUUID2, "orchestration.dispatch", "rt"},
		"method":  {covHandoffUUID1, "orchestration.other", "rt"},
		"runtime": {covHandoffUUID1, "orchestration.dispatch", "rt-2"},
	} {
		_, err := DeliveryRequestStatus(covHandoffUUID1, "orchestration.dispatch", "rt", args[0], args[1], args[2], "completed")
		if err == nil || err.Error() != mismatch {
			t.Fatalf("%s mismatch accepted: %v", name, err)
		}
	}
	_, err := DeliveryRequestStatus(covHandoffUUID1, "m", "rt", covHandoffUUID1, "m", "rt", "absent")
	if err == nil || err.Error() != "Orca durable request is absent; recovery remains ambiguous" {
		t.Fatalf("absent err=%v", err)
	}
	_, err = DeliveryRequestStatus(covHandoffUUID1, "m", "rt", covHandoffUUID1, "m", "rt", "weird")
	if err == nil || err.Error() != "Orca request observation state is invalid" {
		t.Fatalf("unknown status err=%v", err)
	}
}

func TestCovHandoffClaimQueryFor(t *testing.T) {
	holder := &issueopscontract.NativeActor{Host: "codex", SessionID: "s"}
	active := func(execution issueopscontract.Execution) issueopscontract.ExecutionResult {
		execution.Lease = issueopscontract.WriteLease{Generation: 6, Status: issueopscontract.LeaseStatusActive, Holder: holder}
		return issueopscontract.ExecutionResult{OK: true, ID: "io-claim", Execution: execution}
	}
	if got := HandoffDeliveryClaimQueryFor(issueopscontract.ExecutionResult{ID: "io-claim", Execution: issueopscontract.Execution{Lease: issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusActive, Holder: holder}}}); got != (HandoffDeliveryClaimQuery{}) {
		t.Fatalf("not-ok result queried: %+v", got)
	}
	released := active(issueopscontract.Execution{})
	released.Execution.Lease.Status = issueopscontract.LeaseStatusReleased
	if got := HandoffDeliveryClaimQueryFor(released); got != (HandoffDeliveryClaimQuery{}) {
		t.Fatalf("released lease queried: %+v", got)
	}
	holderless := active(issueopscontract.Execution{})
	holderless.Execution.Lease.Holder = nil
	if got := HandoffDeliveryClaimQueryFor(holderless); got != (HandoffDeliveryClaimQuery{}) {
		t.Fatalf("holderless lease queried: %+v", got)
	}
	direct := active(issueopscontract.Execution{Mode: issueopscontract.ExecutionModeDirect})
	if got := HandoffDeliveryClaimQueryFor(direct); got != (HandoffDeliveryClaimQuery{Observe: true, Manual: true, LifecycleID: "io-claim"}) {
		t.Fatalf("direct query=%+v", got)
	}
	nonDirect := active(issueopscontract.Execution{Mode: "other"})
	if got := HandoffDeliveryClaimQueryFor(nonDirect); got != (HandoffDeliveryClaimQuery{Manual: true, LifecycleID: "io-claim"}) {
		t.Fatalf("non-direct query=%+v", got)
	}
	binding := &issueopscontract.OrcaBinding{OwnerHost: "codex", OwnerPromptSHA256: " p ", ContextPacketSHA256: " m "}
	orca := active(issueopscontract.Execution{Orca: binding})
	if got := HandoffDeliveryClaimQueryFor(orca); got != (HandoffDeliveryClaimQuery{Observe: true, LifecycleID: "io-claim", LineageID: "generation:6:prompt:p:material:m:call:dispatch"}) {
		t.Fatalf("orca query=%+v", got)
	}
	binding.OwnerHost = "omo"
	if got := HandoffDeliveryClaimQueryFor(orca); got.LineageID != "generation:6:prompt:p:material:m:call:prompt" || got.Manual {
		t.Fatalf("omo query=%+v", got)
	}
}

func covHandoffOrcaClaimResult(observation covHandoffObservation) issueopscontract.ExecutionResult {
	holder := *observation.OwnerActor
	return issueopscontract.ExecutionResult{OK: true, ID: observation.LifecycleID, Execution: issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeOrca,
		Lease: issueopscontract.WriteLease{
			Generation: observation.SourceGeneration, Status: issueopscontract.LeaseStatusActive, Holder: &holder, ClaimedAt: "2026-09-20T10:06:00Z",
		},
		Orca: &issueopscontract.OrcaBinding{
			RuntimeID: observation.Launcher.RuntimeID, OwnerHost: "codex", TerminalPTYID: observation.Target.TerminalID,
			OwnerPromptSHA256: observation.PromptSHA256, ContextPacketSHA256: observation.MaterialSHA256,
		},
	}}
}

func covHandoffOrcaClaimObservation() covHandoffObservation {
	observation := deliveryObservationFixture()
	observation.ExpectedOwnerHost = "codex"
	observation.InputAccepted = covHandoffObserved(issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatch)
	observation.LineageID = "generation:7:prompt:" + observation.PromptSHA256 + ":material:" + observation.MaterialSHA256 + ":call:dispatch"
	return observation
}

func TestCovHandoffOwnerClaimOrcaLineage(t *testing.T) {
	observation := covHandoffOrcaClaimObservation()
	result := covHandoffOrcaClaimResult(observation)

	t.Run("claims the exact lineage", func(t *testing.T) {
		claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{observation})
		if err != nil || claimed == nil {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
		if claimed.OwnerClaimed.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved ||
			claimed.OwnerClaimed.Evidence != issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim ||
			claimed.OwnerClaimed.ObservedAt != "2026-09-20T10:06:00Z" || claimed.UpdatedAt != "2026-09-20T10:06:00Z" ||
			!claimed.OwnerClaim.Claimed || claimed.OwnerClaim.Generation != 7 || claimed.OwnerClaim.ClaimedAt != "2026-09-20T10:06:00Z" ||
			claimed.OwnerActor == nil || claimed.OwnerActor.SessionID != "session-1" {
			t.Fatalf("claim fields wrong: %+v", claimed)
		}
		if claimed.InputAccepted.Status != issueopscontract.IssueOpsHandoffDeliveryStateObserved {
			t.Fatalf("existing evidence dropped: %+v", claimed.InputAccepted)
		}
	})
	t.Run("inactive lease observes nothing", func(t *testing.T) {
		released := result
		released.Execution.Lease.Status = issueopscontract.LeaseStatusReleased
		claimed, err := OwnerClaimHandoffDeliveryObservation(released, []covHandoffObservation{observation})
		if claimed != nil || err != nil {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
	})
	t.Run("different lineage is not found", func(t *testing.T) {
		other := observation
		other.LineageID = "lineage-other"
		claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{other})
		if claimed != nil || err != nil {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
	})
	t.Run("invalid stored evidence fails closed", func(t *testing.T) {
		broken := observation
		broken.Receipt.Location = ""
		claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{broken})
		if claimed != nil || err == nil || err.Error() != "handoff delivery claim evidence is invalid: delivery observation receipt is invalid" {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
	})
	for name, mutate := range map[string]func(*issueopscontract.ExecutionResult){
		"generation": func(r *issueopscontract.ExecutionResult) { r.Execution.Lease.Generation = 8 },
		"host":       func(r *issueopscontract.ExecutionResult) { r.Execution.Lease.Holder.Host = "claude" },
		"runtime":    func(r *issueopscontract.ExecutionResult) { r.Execution.Orca.RuntimeID = "runtime-2" },
		"terminal":   func(r *issueopscontract.ExecutionResult) { r.Execution.Orca.TerminalPTYID = "term-2" },
	} {
		t.Run("committed lease mismatch on "+name, func(t *testing.T) {
			mismatched := covHandoffOrcaClaimResult(observation)
			mutate(&mismatched)
			// Keep the lineage query identical so only the lease comparison can reject.
			mismatched.Execution.Orca.OwnerPromptSHA256 = observation.PromptSHA256
			stored := observation
			stored.LineageID = HandoffDeliveryClaimQueryFor(mismatched).LineageID
			claimed, err := OwnerClaimHandoffDeliveryObservation(mismatched, []covHandoffObservation{stored})
			if claimed != nil || err == nil || err.Error() != "handoff delivery claim does not match the committed lease" {
				t.Fatalf("claimed=%+v err=%v", claimed, err)
			}
		})
	}
	t.Run("unbound pty accepts any observed terminal", func(t *testing.T) {
		unbound := covHandoffOrcaClaimResult(observation)
		unbound.Execution.Orca.TerminalPTYID = ""
		claimed, err := OwnerClaimHandoffDeliveryObservation(unbound, []covHandoffObservation{observation})
		if err != nil || claimed == nil {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
	})
	t.Run("claim failing observation validation is returned", func(t *testing.T) {
		badClaim := covHandoffOrcaClaimResult(observation)
		badClaim.Execution.Lease.ClaimedAt = "2026-09-20T09:00:00Z"
		claimed, err := OwnerClaimHandoffDeliveryObservation(badClaim, []covHandoffObservation{observation})
		if claimed != nil || err == nil {
			t.Fatalf("claim before creation accepted: %+v", claimed)
		}
	})
}

func covHandoffManualClaimFixture() (issueopscontract.ExecutionResult, covHandoffObservation) {
	observation := cmuxDeliveryObservationFixture()
	observation.Target.WorkspaceID, observation.Target.SurfaceID = "workspace-1", "surface-1"
	process := &issueopscontract.NativeProcessReceipt{PID: 4242, StartedAt: "2026-09-20T10:00:00Z", Executable: "/usr/local/bin/codex"}
	observation.Target.Process = process
	observation.InputAccepted = deliveryStateAt(issueopscontract.IssueOpsHandoffDeliveryStateObserved, issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput, "2026-09-20T10:00:02Z")
	observation.UpdatedAt = "2026-09-20T10:00:02Z"
	observation.AttemptID = handoffDeliveryManualLineagePrefix + observation.LifecycleID + ":7:cmux:abc"
	observation.LineageID = handoffDeliveryManualLineagePrefix + "generation:7:cmux:window:window-1"
	holderProcess := *process
	holder := issueopscontract.NativeActor{Host: "codex", SessionID: "session-9", SessionProcess: &holderProcess}
	return issueopscontract.ExecutionResult{OK: true, ID: observation.LifecycleID, Execution: issueopscontract.Execution{
		Mode: issueopscontract.ExecutionModeDirect,
		Lease: issueopscontract.WriteLease{
			Generation: 8, Status: issueopscontract.LeaseStatusActive, Holder: &holder, ClaimedAt: "2026-09-20T10:10:00Z",
		},
	}}, observation
}

func TestCovHandoffOwnerClaimManualLineage(t *testing.T) {
	t.Run("claims immediate reseed from the same holder process", func(t *testing.T) {
		result, observation := covHandoffManualClaimFixture()
		claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{observation})
		if err != nil || claimed == nil {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
		if !claimed.OwnerClaim.Claimed || claimed.OwnerClaim.Generation != 8 || claimed.OwnerActor.SessionID != "session-9" ||
			claimed.UpdatedAt != "2026-09-20T10:10:00Z" || claimed.Launcher.Name != issueopscontract.IssueOpsHandoffDeliveryLauncherCmux {
			t.Fatalf("claim=%+v", claimed)
		}
	})
	t.Run("native turn evidence also counts as accepted", func(t *testing.T) {
		result, observation := covHandoffManualClaimFixture()
		observation.InputAccepted = deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateNotObserved, "")
		observation.NativeTurnObserved = deliveryStateAt(issueopscontract.IssueOpsHandoffDeliveryStateObserved, issueopscontract.IssueOpsHandoffDeliveryEvidenceNativeReceipt, "2026-09-20T10:00:02Z")
		claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{observation})
		if err != nil || claimed == nil {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
	})
	t.Run("staged-only observation is not claimed", func(t *testing.T) {
		result, observation := covHandoffManualClaimFixture()
		observation.InputAccepted = deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateNotObserved, "")
		claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{observation})
		if claimed != nil || err != nil {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
	})
	for name, mutate := range map[string]func(*issueopscontract.ExecutionResult, *covHandoffObservation){
		"holder pid": func(r *issueopscontract.ExecutionResult, _ *covHandoffObservation) {
			r.Execution.Lease.Holder.SessionProcess.PID = 1
		},
		"holder start": func(r *issueopscontract.ExecutionResult, _ *covHandoffObservation) {
			r.Execution.Lease.Holder.SessionProcess.StartedAt = "later"
		},
		"holder executable": func(r *issueopscontract.ExecutionResult, _ *covHandoffObservation) {
			r.Execution.Lease.Holder.SessionProcess.Executable = "/bin/other"
		},
		"holder without proc": func(r *issueopscontract.ExecutionResult, _ *covHandoffObservation) {
			r.Execution.Lease.Holder.SessionProcess = nil
		},
		"target without proc": func(_ *issueopscontract.ExecutionResult, o *covHandoffObservation) { o.Target.Process = nil },
		"lease generation 1":  func(r *issueopscontract.ExecutionResult, _ *covHandoffObservation) { r.Execution.Lease.Generation = 1 },
		"skipped generation":  func(r *issueopscontract.ExecutionResult, _ *covHandoffObservation) { r.Execution.Lease.Generation = 9 },
		"other lifecycle":     func(r *issueopscontract.ExecutionResult, _ *covHandoffObservation) { r.ID = "io-other" },
		"other owner host": func(r *issueopscontract.ExecutionResult, _ *covHandoffObservation) {
			r.Execution.Lease.Holder.Host = "claude"
		},
		"non-manual lineage": func(_ *issueopscontract.ExecutionResult, o *covHandoffObservation) { o.LineageID = "generation:7:cmux" },
	} {
		t.Run("no claim for "+name, func(t *testing.T) {
			result, observation := covHandoffManualClaimFixture()
			mutate(&result, &observation)
			claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{observation})
			if claimed != nil || err != nil {
				t.Fatalf("claimed=%+v err=%v", claimed, err)
			}
		})
	}
	t.Run("two manual lineages are ambiguous", func(t *testing.T) {
		result, observation := covHandoffManualClaimFixture()
		second := observation
		second.LineageID += ":second"
		claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{observation, second})
		if claimed != nil || err == nil || err.Error() != "manual handoff delivery claim evidence is ambiguous" {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
	})
	t.Run("invalid manual candidate fails closed", func(t *testing.T) {
		result, observation := covHandoffManualClaimFixture()
		observation.Receipt.Digest = "bad"
		claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{observation})
		if claimed != nil || err == nil || err.Error() != "manual handoff delivery claim evidence is invalid: delivery observation receipt is invalid" {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
	})
	t.Run("non-direct execution without orca binding observes nothing", func(t *testing.T) {
		result, observation := covHandoffManualClaimFixture()
		result.Execution.Mode = "other"
		claimed, err := OwnerClaimHandoffDeliveryObservation(result, []covHandoffObservation{observation})
		if claimed != nil || err != nil {
			t.Fatalf("claimed=%+v err=%v", claimed, err)
		}
	})
}

func TestCovHandoffCompleteDeliveryObservation(t *testing.T) {
	const at = "2026-09-20T10:03:00Z"
	base := deliveryObservationFixture()
	baseline := uint64(9)
	prompt := &DeliveryPrompt{RequestID: covHandoffUUID2, ProcessIncarnation: " inc-2 ", Generation: 4, BaselineWorkingSequence: &baseline, Stages: []string{"input_accepted", "turn_started"}}

	got, err := CompleteDeliveryObservation(base, "codex", "dispatch", at, nil)
	if err != nil || got.InputAccepted != DeliveryObserved(at, issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatch) ||
		got.Ambiguous != base.Ambiguous {
		t.Fatalf("non-omo dispatch: %+v err=%v", got, err)
	}
	got, err = CompleteDeliveryObservation(base, "omo", "dispatch", at, nil)
	if err != nil || got.Ambiguous != DeliveryObserved(at, issueopscontract.IssueOpsHandoffDeliveryEvidenceOrcaDispatchReceipt) ||
		got.InputAccepted != base.InputAccepted {
		t.Fatalf("omo dispatch must stay ambiguous: %+v err=%v", got, err)
	}
	if _, err = CompleteDeliveryObservation(base, "omo", "prompt", at, nil); err == nil || err.Error() != "Omo prompt completion is missing its durable receipt" {
		t.Fatalf("prompt without receipt err=%v", err)
	}
	got, err = CompleteDeliveryObservation(base, "omo", "prompt", at, prompt)
	if err != nil {
		t.Fatal(err)
	}
	if got.InputAccepted != DeliveryObserved(at, issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendAccepted) ||
		got.NativeTurnObserved != DeliveryObserved(at, issueopscontract.IssueOpsHandoffDeliveryEvidenceNativeReceipt) ||
		got.Target.ProcessIncarnation != "inc-2" || got.Target.PromptGeneration == nil || *got.Target.PromptGeneration != 4 ||
		got.Target.BaselineWorkingSequence == nil || *got.Target.BaselineWorkingSequence != 9 {
		t.Fatalf("prompt completion: %+v", got)
	}
	baseline = 100
	if *got.Target.BaselineWorkingSequence != 9 {
		t.Fatal("baseline pointer aliased the prompt receipt")
	}
	prompt.Stages = []string{"input_accepted"}
	got, err = CompleteDeliveryObservation(base, "omo", "prompt", at, prompt)
	if err != nil || got.NativeTurnObserved != base.NativeTurnObserved {
		t.Fatalf("native turn recorded without turn_started: %+v err=%v", got.NativeTurnObserved, err)
	}
	prompt.BaselineWorkingSequence = nil
	got, err = CompleteDeliveryObservation(base, "omo", "prompt", at, prompt)
	if err != nil || got.Target.BaselineWorkingSequence != nil {
		t.Fatalf("nil baseline invented: %+v err=%v", got.Target.BaselineWorkingSequence, err)
	}
}

func TestCovHandoffDeliveryCompletionID(t *testing.T) {
	prompt := &DeliveryPrompt{RequestID: " " + covHandoffUUID2 + " "}
	for _, test := range []struct {
		dispatch, kind string
		prompt         *DeliveryPrompt
		want           string
	}{
		{" " + covHandoffUUID1 + " ", "dispatch", prompt, covHandoffUUID1},
		{covHandoffUUID1, "prompt", prompt, covHandoffUUID2},
		{covHandoffUUID1, "prompt", nil, covHandoffUUID1},
	} {
		if got := DeliveryCompletionID(test.dispatch, test.kind, test.prompt); got != test.want {
			t.Fatalf("DeliveryCompletionID(%+v)=%q", test, got)
		}
	}
}

func TestCovHandoffDeliveryFailurePlan(t *testing.T) {
	attempt := DeliveryAttempt{RetryRequestID: covHandoffUUID1, PromptRetryRequestID: covHandoffUUID2}
	lost := issueopscontract.IssueOpsHandoffDeliveryEvidenceAcceptedResponseLost
	if got := DeliveryFailurePlan(attempt, nil); got != (DeliveryFailureDisposition{Evidence: lost}) {
		t.Fatalf("nil failure plan=%+v", got)
	}
	for name, failure := range map[string]*DeliveryFailure{
		"never invoked":           {Invoked: false, OrchestrationRequestID: covHandoffUUID1},
		"observation failed":      {Invoked: true, Code: "delivery_observation_failed", OrchestrationRequestID: covHandoffUUID1},
		"response id mismatched":  {Invoked: true, OrchestrationRequestID: covHandoffUUID2},
		"identity mismatch code":  {Invoked: true, Code: "dispatch_request_identity_mismatch"},
		"terminal dispatch drift": {Invoked: true, CallPhase: "terminal_send", DispatchRequestID: covHandoffUUID2, OrchestrationRequestID: covHandoffUUID2},
	} {
		if got := DeliveryFailurePlan(attempt, failure); got != (DeliveryFailureDisposition{Skip: true}) {
			t.Fatalf("%s: plan=%+v", name, got)
		}
	}
	got := DeliveryFailurePlan(attempt, &DeliveryFailure{Invoked: true, OrchestrationRequestID: covHandoffUUID1})
	if got != (DeliveryFailureDisposition{DurableID: covHandoffUUID1, Evidence: lost}) {
		t.Fatalf("dispatch response lost plan=%+v", got)
	}
	got = DeliveryFailurePlan(DeliveryAttempt{}, &DeliveryFailure{Invoked: true})
	if got != (DeliveryFailureDisposition{Evidence: lost}) {
		t.Fatalf("response lost without ids plan=%+v", got)
	}
	got = DeliveryFailurePlan(attempt, &DeliveryFailure{Invoked: true, CallPhase: "terminal_send", OrchestrationRequestID: covHandoffUUID2, DispatchRequestID: covHandoffUUID1})
	want := DeliveryFailureDisposition{DurableID: covHandoffUUID2, DispatchID: covHandoffUUID1, Evidence: issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendResponseLost}
	if got != want {
		t.Fatalf("terminal send plan=%+v want %+v", got, want)
	}
}

func TestCovHandoffNewDeliveryObservation(t *testing.T) {
	attempt := DeliveryAttempt{
		OperationID: "op", Stage: "dispatch", LifecycleID: "io-new", Host: "codex", TerminalID: "term-attempt",
		PromptSHA256: strings.Repeat("a", 64), MaterialSHA256: strings.Repeat("e", 64), SourceGeneration: 7,
	}
	launcher := deliveryObservationFixture().Launcher
	input := DeliveryObservationInput{Attempt: attempt, Launcher: launcher, DurableID: " " + covHandoffUUID1 + " ", CallKind: "dispatch", Timestamp: "2026-09-20T11:00:00Z"}

	fresh := NewDeliveryObservation(input, nil)
	if fresh.SchemaVersion != issueopscontract.IssueOpsHandoffDeliverySchemaVersion || fresh.AttemptID != "op:dispatch:dispatch" ||
		fresh.LineageID != attempt.LineageID("dispatch") || fresh.Request.DurableID != covHandoffUUID1 ||
		fresh.CreatedAt != input.Timestamp || fresh.UpdatedAt != input.Timestamp || fresh.ExpectedOwnerHost != "codex" ||
		fresh.Target.TerminalID != "term-attempt" || fresh.Target.PaneID != "" {
		t.Fatalf("fresh=%+v", fresh)
	}
	for name, state := range map[string]issueopscontract.IssueOpsHandoffDeliveryState{
		"input": fresh.InputAccepted, "native": fresh.NativeTurnObserved, "claimed": fresh.OwnerClaimed, "ambiguous": fresh.Ambiguous,
	} {
		if state != (issueopscontract.IssueOpsHandoffDeliveryState{Status: issueopscontract.IssueOpsHandoffDeliveryStateNotObserved}) {
			t.Fatalf("%s state=%+v", name, state)
		}
	}
	if fresh.CallStaged.Status != "" {
		t.Fatalf("call_staged must be left for the caller: %+v", fresh.CallStaged)
	}

	input.Target = issueopscontract.IssueOpsHandoffDeliveryTarget{TerminalID: "term-target", PaneID: "pane-target"}
	if got := NewDeliveryObservation(input, nil).Target; got.TerminalID != "term-target" || got.PaneID != "pane-target" {
		t.Fatalf("target precedence=%+v", got)
	}
	input.ReceiptTarget = issueopscontract.IssueOpsHandoffDeliveryTarget{TerminalID: " term-receipt ", PaneID: "pane-receipt"}
	if got := NewDeliveryObservation(input, nil).Target; got.TerminalID != "term-receipt" || got.PaneID != "pane-receipt" {
		t.Fatalf("receipt target precedence=%+v", got)
	}

	existing := deliveryObservationFixture()
	existing.LifecycleID = "io-new"
	existing.LineageID = attempt.LineageID("dispatch")
	existing.CreatedAt = "2026-09-20T10:00:01Z"
	got := NewDeliveryObservation(input, []covHandoffObservation{existing})
	if got.CreatedAt != "2026-09-20T10:00:01Z" || got.UpdatedAt != "2026-09-20T11:00:00Z" {
		t.Fatalf("created_at must be preserved from the folded lineage: %+v", got)
	}
	existing.SchemaVersion = 0
	got = NewDeliveryObservation(input, []covHandoffObservation{existing})
	if got.CreatedAt != "2026-09-20T11:00:00Z" {
		t.Fatalf("invalid prior evidence must not donate created_at: %s", got.CreatedAt)
	}
}

func TestCovHandoffPlanDeliveryRecovery(t *testing.T) {
	dispatch := covHandoffObservation{Request: issueopscontract.IssueOpsHandoffDeliveryRequest{DurableID: covHandoffUUID1}}
	prompt := covHandoffObservation{Request: issueopscontract.IssueOpsHandoffDeliveryRequest{DurableID: covHandoffUUID2}}
	none := covHandoffObservation{}

	plan, err := PlanDeliveryRecovery(DeliveryAttempt{Host: "codex"}, none, none, false, false)
	if err != nil || plan != (DeliveryRecoveryPlan{}) {
		t.Fatalf("nothing recovered plan=%+v err=%v", plan, err)
	}
	plan, err = PlanDeliveryRecovery(DeliveryAttempt{Host: "omo"}, dispatch, prompt, true, true)
	if err != nil || plan != (DeliveryRecoveryPlan{DispatchID: covHandoffUUID1, PromptID: covHandoffUUID2, Active: true}) {
		t.Fatalf("plan=%+v err=%v", plan, err)
	}
	plan, err = PlanDeliveryRecovery(DeliveryAttempt{Host: "codex"}, dispatch, none, true, false)
	if err != nil || !plan.Active || plan.DispatchID != covHandoffUUID1 || plan.PromptID != "" {
		t.Fatalf("dispatch-only plan=%+v err=%v", plan, err)
	}
	plan, err = PlanDeliveryRecovery(DeliveryAttempt{Host: "codex"}, none, none, false, true)
	if err != nil || !plan.Active {
		t.Fatalf("non-omo prompt without id plan=%+v err=%v", plan, err)
	}
	for _, test := range []struct {
		name               string
		attempt            DeliveryAttempt
		dispatch, prompt   covHandoffObservation
		dispatchF, promptF bool
		want               string
	}{
		{"invalid dispatch retry id", DeliveryAttempt{RetryRequestID: "bad"}, none, none, false, false, "Orca dispatch retry request ID is invalid"},
		{"dispatch without durable id", DeliveryAttempt{Host: "codex"}, none, none, true, false, "Orca dispatch delivery is ambiguous without a durable request ID"},
		{"invalid prompt retry id", DeliveryAttempt{PromptRetryRequestID: "bad"}, none, none, false, false, "Orca prompt retry request ID is invalid"},
		{"prompt retry without observation", DeliveryAttempt{PromptRetryRequestID: covHandoffUUID2}, none, none, false, false, "Orca prompt retry request has no exact delivery observation"},
		{"omo prompt without durable id", DeliveryAttempt{Host: "omo"}, dispatch, none, true, true, "Omo prompt delivery is ambiguous without a durable request ID"},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := PlanDeliveryRecovery(test.attempt, test.dispatch, test.prompt, test.dispatchF, test.promptF)
			if err == nil || err.Error() != test.want {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestCovHandoffInspectDeliveryDispatch(t *testing.T) {
	for _, test := range []struct {
		host   string
		found  bool
		status string
		want   bool
		fail   bool
	}{
		{"codex", false, "completed", true, false},
		{"codex", false, "pending", true, false},
		{"claude", true, "absent", false, false},
		{"codex", true, "", false, false},
		{"omo", true, "absent", true, false},
		{"omo", false, "completed", false, true},
	} {
		got, err := InspectDeliveryDispatch(test.host, test.found, test.status)
		if got != test.want || (err != nil) != test.fail {
			t.Fatalf("InspectDeliveryDispatch(%+v)=%v,%v", test, got, err)
		}
	}
}

func TestCovHandoffFinishDeliveryRecovery(t *testing.T) {
	baseline, generation := uint64(12), uint64(3)
	accepted := covHandoffObservation{
		Request:       issueopscontract.IssueOpsHandoffDeliveryRequest{DurableID: covHandoffUUID2},
		Target:        issueopscontract.IssueOpsHandoffDeliveryTarget{ProcessIncarnation: "inc", PromptGeneration: &generation, BaselineWorkingSequence: &baseline},
		InputAccepted: covHandoffObserved(issueopscontract.IssueOpsHandoffDeliveryEvidenceOmoSendAccepted),
	}

	for _, test := range []struct {
		name   string
		exists bool
		want   DeliveryRecoveryMode
	}{{"existing dispatch becomes candidate", true, DeliveryRecoveryCandidate}, {"missing dispatch replays", false, DeliveryRecoveryReplay}} {
		got, err := FinishDeliveryRecovery("codex", DeliveryRecoveryPlan{}, "completed", test.exists, covHandoffObservation{}, false)
		if err != nil || got.Mode != test.want || got.Prompt != nil {
			t.Fatalf("%s: %+v err=%v", test.name, got, err)
		}
	}

	got, err := FinishDeliveryRecovery("omo", DeliveryRecoveryPlan{}, "", true, accepted, true)
	if err != nil || got.Mode != DeliveryRecoveryCandidate || got.Prompt == nil {
		t.Fatalf("accepted prompt: %+v err=%v", got, err)
	}
	if got.Prompt.RequestID != covHandoffUUID2 || got.Prompt.ProcessIncarnation != "inc" || got.Prompt.Generation != 3 ||
		len(got.Prompt.Stages) != 1 || got.Prompt.Stages[0] != "input_accepted" ||
		got.Prompt.BaselineWorkingSequence == nil || *got.Prompt.BaselineWorkingSequence != 12 {
		t.Fatalf("prompt=%+v", got.Prompt)
	}
	if got.Prompt.BaselineWorkingSequence == &baseline {
		t.Fatal("baseline must be copied")
	}

	turn := accepted
	turn.NativeTurnObserved = covHandoffObserved(issueopscontract.IssueOpsHandoffDeliveryEvidenceNativeReceipt)
	got, err = FinishDeliveryRecovery("omo", DeliveryRecoveryPlan{}, "", true, turn, true)
	if err != nil || len(got.Prompt.Stages) != 2 || got.Prompt.Stages[1] != "turn_started" {
		t.Fatalf("native turn prompt: %+v err=%v", got.Prompt, err)
	}

	claimedOnly := accepted
	claimedOnly.InputAccepted = deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateNotObserved, "")
	claimedOnly.OwnerClaimed = covHandoffObserved(issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim)
	got, err = FinishDeliveryRecovery("omo", DeliveryRecoveryPlan{}, "", true, claimedOnly, true)
	if err != nil || got.Mode != DeliveryRecoveryCandidate || got.Prompt == nil {
		t.Fatalf("claimed prompt: %+v err=%v", got, err)
	}

	incomplete := map[string]func() (covHandoffObservation, bool){
		"dispatch missing": func() (covHandoffObservation, bool) { return accepted, false },
		"no durable id":    func() (covHandoffObservation, bool) { o := accepted; o.Request.DurableID = " "; return o, true },
		"no incarnation":   func() (covHandoffObservation, bool) { o := accepted; o.Target.ProcessIncarnation = ""; return o, true },
		"no generation":    func() (covHandoffObservation, bool) { o := accepted; o.Target.PromptGeneration = nil; return o, true },
		"no baseline count": func() (covHandoffObservation, bool) {
			o := accepted
			o.Target.BaselineWorkingSequence = nil
			return o, true
		},
	}
	for name, build := range incomplete {
		t.Run(name, func(t *testing.T) {
			observation, dispatchExists := build()
			got, err := FinishDeliveryRecovery("omo", DeliveryRecoveryPlan{}, "completed", dispatchExists, observation, true)
			if err == nil || err.Error() != "Omo prompt delivery recovery evidence is incomplete" || got.Prompt != nil {
				t.Fatalf("got=%+v err=%v", got, err)
			}
		})
	}

	for _, test := range []struct {
		name   string
		plan   DeliveryRecoveryPlan
		status string
		want   DeliveryRecoveryMode
	}{
		{"prompt id replays through terminal send", DeliveryRecoveryPlan{PromptID: covHandoffUUID2}, "absent", DeliveryRecoveryReplay},
		{"completed dispatch replays", DeliveryRecoveryPlan{DispatchID: covHandoffUUID1}, "completed", DeliveryRecoveryReplay},
		{"pending dispatch replays", DeliveryRecoveryPlan{DispatchID: covHandoffUUID1}, "pending", DeliveryRecoveryReplay},
		{"absent dispatch delegates", DeliveryRecoveryPlan{DispatchID: covHandoffUUID1}, "absent", DeliveryRecoveryDelegate},
		{"nothing recovered delegates", DeliveryRecoveryPlan{}, "completed", DeliveryRecoveryDelegate},
	} {
		got, err := FinishDeliveryRecovery("omo", test.plan, test.status, true, covHandoffObservation{}, false)
		if err != nil || got.Mode != test.want || got.Prompt != nil {
			t.Fatalf("%s: %+v err=%v", test.name, got, err)
		}
	}
	notAccepted := accepted
	notAccepted.InputAccepted = deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateNotObserved, "")
	got, err = FinishDeliveryRecovery("omo", DeliveryRecoveryPlan{}, "", true, notAccepted, true)
	if err != nil || got.Mode != DeliveryRecoveryDelegate {
		t.Fatalf("unobserved prompt must not be resumed: %+v err=%v", got, err)
	}
}

func TestCovHandoffCmuxDigestAndNULValidation(t *testing.T) {
	for _, test := range []struct {
		value string
		want  bool
	}{
		{strings.Repeat("a", 64), true},
		{strings.Repeat("0", 64), true},
		{strings.Repeat("A", 64), false},
		{strings.Repeat("a", 63), false},
		{strings.Repeat("a", 65), false},
		{strings.Repeat("g", 64), false},
		{"", false},
	} {
		if got := ValidCmuxDigest(test.value); got != test.want {
			t.Fatalf("ValidCmuxDigest(%q)=%v", test.value, got)
		}
	}
	clean := issueopscontract.ExecutionCmuxHandoffRequest{ID: "io", CWD: "/repo", PromptFile: "/repo/p.md"}
	if CmuxRequestContainsNUL(clean) {
		t.Fatal("clean request flagged")
	}
	fields := map[string]func(*issueopscontract.ExecutionCmuxHandoffRequest){
		"id":         func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.ID = "a\x00b" },
		"executable": func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.CmuxExecutable = "\x00" },
		"version":    func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.CmuxVersion = "1\x00" },
		"build":      func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.CmuxBuild = "\x00b" },
		"socket":     func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.SocketPath = "/s\x00" },
		"window":     func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.WindowID = "w\x00" },
		"cwd":        func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.CWD = "/r\x00" },
		"host":       func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.Host = "c\x00" },
		"host exe":   func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.HostExecutable = "\x00" },
		"model":      func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.Model = "m\x00" },
		"effort":     func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.Effort = "\x00" },
		"prompt":     func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.PromptFile = "/p\x00" },
		"prompt sha": func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.PromptSHA256 = "\x00" },
		"material":   func(r *issueopscontract.ExecutionCmuxHandoffRequest) { r.MaterialSHA256 = "\x00" },
	}
	for name, mutate := range fields {
		request := clean
		mutate(&request)
		if !CmuxRequestContainsNUL(request) {
			t.Fatalf("NUL in %s not detected", name)
		}
	}
}

func TestCovHandoffCmuxCanonicalPromptPath(t *testing.T) {
	const root = "/real/worktree"
	for _, test := range []struct {
		name    string
		prompt  string
		aliases []string
		want    string
		wantErr string
	}{
		{"inside canonical root", "/real/worktree/.issueops/prompt.md", nil, "/real/worktree/.issueops/prompt.md", ""},
		{"alias is rewritten to canonical root", "/alias/wt/.issueops/prompt.md", []string{"/alias/wt"}, "/real/worktree/.issueops/prompt.md", ""},
		{"later alias is tried", "/b/wt/p.md", []string{"/a/wt", "/b/wt"}, "/real/worktree/p.md", ""},
		{"relative path", "prompt.md", nil, "", "must be a clean path inside the canonical worktree"},
		{"unclean path", "/real/worktree/a/../prompt.md", nil, "", "must be a clean path inside the canonical worktree"},
		{"root itself", root, nil, "", "must be inside the canonical worktree"},
		{"outside root", "/elsewhere/prompt.md", nil, "", "must be inside the canonical worktree"},
		{"sibling sharing a prefix", "/real/worktree-two/prompt.md", nil, "", "must be inside the canonical worktree"},
		{"unclean alias is ignored", "/alias/wt/p.md", []string{"/alias/wt/"}, "", "must be inside the canonical worktree"},
		{"relative alias is ignored", "/alias/wt/p.md", []string{"alias/wt"}, "", "must be inside the canonical worktree"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := CmuxCanonicalPromptPath(root, test.prompt, test.aliases...)
			if test.wantErr == "" {
				if err != nil || got != test.want {
					t.Fatalf("got=%q err=%v", got, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), test.wantErr) || got != "" {
				t.Fatalf("got=%q err=%v", got, err)
			}
		})
	}
	if _, err := CmuxCanonicalPromptPath("relative/root", "/real/worktree/p.md"); err == nil {
		t.Fatal("non-absolute canonical root must not authorize any path")
	}
}

func TestCovHandoffCmuxReleasedRecordRejections(t *testing.T) {
	released := func() issueopscontract.IssueOpsRecord {
		return issueopscontract.IssueOpsRecord{Execution: &issueopscontract.Execution{
			Mode: issueopscontract.ExecutionModeDirect, Lease: issueopscontract.WriteLease{Status: issueopscontract.LeaseStatusReleased, Generation: 3},
		}}
	}
	for name, mutate := range map[string]func(*issueopscontract.IssueOpsRecord){
		"no execution": func(r *issueopscontract.IssueOpsRecord) { r.Execution = nil },
		"orca mode":    func(r *issueopscontract.IssueOpsRecord) { r.Execution.Mode = issueopscontract.ExecutionModeOrca },
		"active lease": func(r *issueopscontract.IssueOpsRecord) {
			r.Execution.Lease.Status = issueopscontract.LeaseStatusActive
		},
		"other lease gen": func(r *issueopscontract.IssueOpsRecord) { r.Execution.Lease.Generation = 2 },
	} {
		record := released()
		mutate(&record)
		err := ValidateCmuxReleasedRecord(record, 3)
		if err == nil || err.Error() != "cmux handoff requires the exact released direct execution generation" {
			t.Fatalf("%s: err=%v", name, err)
		}
	}
}

func TestCovHandoffCmuxResultCopiesObservationFields(t *testing.T) {
	observation := cmuxDeliveryObservationFixture()
	observation.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{InputSendMS: 4}
	observation.InputAccepted = CmuxObserved("2026-09-20T10:00:02Z", issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput)
	observation.NativeTurnObserved = CmuxObserved("2026-09-20T10:00:03Z", issueopscontract.IssueOpsHandoffDeliveryEvidenceNativeReceipt)
	observation.OwnerClaimed = CmuxObserved("2026-09-20T10:00:04Z", issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim)
	request := issueopscontract.ExecutionCmuxHandoffRequest{ID: "io-cmux", Generation: 7}
	got := CmuxResult(request, observation, "delivered", "/tmp/recovery")
	if !got.OK || got.ID != "io-cmux" || got.Generation != 7 || got.Status != "delivered" || got.RecoveryArtifactDir != "/tmp/recovery" ||
		!got.InputAccepted || !got.NativeTurnObserved || !got.OwnerClaimed ||
		got.Launcher.Name != "cmux" || got.Target.WindowID != "window-1" || got.Timing == nil || got.Timing.InputSendMS != 4 ||
		got.ObservationReceipt != observation.Receipt {
		t.Fatalf("result=%+v", got)
	}
	staged := CmuxResult(request, cmuxDeliveryObservationFixture(), "staged", "")
	if staged.OK || staged.InputAccepted {
		t.Fatalf("staged-only observation reported as accepted: %+v", staged)
	}
	if got := CmuxNotObserved(); got.Status != issueopscontract.IssueOpsHandoffDeliveryStateNotObserved || got.ObservedAt != "" || got.Evidence != "" {
		t.Fatalf("CmuxNotObserved=%+v", got)
	}
}

func TestCovHandoffMergeRejectsPaneRotationWithoutTerminalAndBaselineDrift(t *testing.T) {
	current := deliveryObservationFixture()
	current.Target.TerminalID = ""
	next := deliveryObservationFixture()
	next.Target.TerminalID = ""
	next.Target.PaneID = "pane-2"
	_, decision := MergeHandoffDeliveryObservation(current, next)
	if decision.Accepted || decision.RejectReasons[0] != "delivery observation process identity changed" {
		t.Fatalf("pane rotation without a PTY identity accepted: %+v", decision)
	}

	generation, baseline, otherBaseline := uint64(2), uint64(5), uint64(6)
	current = deliveryObservationFixture()
	current.Target.PromptGeneration, current.Target.BaselineWorkingSequence = &generation, &baseline
	next = deliveryObservationFixture()
	next.Target.PromptGeneration, next.Target.BaselineWorkingSequence = &generation, &otherBaseline
	_, decision = MergeHandoffDeliveryObservation(current, next)
	if decision.Accepted || decision.RejectReasons[0] != "delivery observation process identity changed" {
		t.Fatalf("baseline drift accepted: %+v", decision)
	}

	// With a stable PTY the transient pane handle may rotate.
	next = deliveryObservationFixture()
	next.Target.PaneID = "pane-2"
	merged, decision := MergeHandoffDeliveryObservation(deliveryObservationFixture(), next)
	if !decision.Accepted || merged.Target.PaneID != "pane-2" {
		t.Fatalf("pane rotation on stable PTY rejected: %+v %+v", merged.Target, decision)
	}
}

func TestCovHandoffMergeFillsZeroPreflightTimingAndAdoptsClaimOwner(t *testing.T) {
	current := deliveryObservationFixture()
	current.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{InputSendMS: 1}
	next := deliveryObservationFixture()
	next.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: 3, InputSendMS: 1}
	merged, decision := MergeHandoffDeliveryObservation(current, next)
	if !decision.Accepted || merged.Timing.PreflightMS != 3 || merged.Timing.InputSendMS != 1 {
		t.Fatalf("timing=%+v decision=%+v", merged.Timing, decision)
	}

	ownerless := deliveryObservationFixture()
	ownerless.OwnerActor = nil
	claimed := covHandoffClaimedFixture()
	merged, decision = MergeHandoffDeliveryObservation(ownerless, claimed)
	if !decision.Accepted || merged.OwnerActor == nil || merged.OwnerActor.SessionID != "session-1" || !merged.OwnerClaim.Claimed {
		t.Fatalf("claim by the next observation's owner rejected: %+v %+v", merged.OwnerActor, decision)
	}
}
