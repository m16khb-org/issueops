package issueops

import (
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

// handoffMutPadTimestamp lengthens an RFC 3339 UTC timestamp to exactly length bytes with fractional-second zeros.
func handoffMutPadTimestamp(value string, length int) string {
	return strings.TrimSuffix(value, "Z") + "." + strings.Repeat("0", length-len(value)-1) + "Z"
}

func TestValidateHandoffDeliveryObservationFieldLimitIsInclusive(t *testing.T) {
	limit := handoffDeliveryFieldLimit
	claimed := func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, claimedAt string) {
		o.OwnerClaimed = deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateObserved, issueopscontract.IssueOpsHandoffDeliveryEvidenceIssueOpsClaim)
		o.OwnerClaim = issueopscontract.IssueOpsHandoffDeliveryOwnerClaim{
			Claimed: true, Generation: o.SourceGeneration, Actor: *o.OwnerActor, ClaimedAt: claimedAt,
		}
	}
	ancestry := func(startedAt, executable string) []issueopscontract.NativeProcessReceipt {
		return []issueopscontract.NativeProcessReceipt{{PID: 1, StartedAt: startedAt, Executable: executable}}
	}
	tests := []struct {
		name      string
		edit      func(*issueopscontract.IssueOpsHandoffDeliveryObservation, int)
		overError string
	}{
		{"attempt id", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.AttemptID = strings.Repeat("a", n)
		},
			"delivery observation attempt_id is required"},
		{"receipt location", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Receipt.Location = strings.Repeat("a", n)
		},
			"delivery observation receipt location is too large"},
		{"launcher version", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Launcher.Version = strings.Repeat("1", n)
		},
			"delivery observation launcher version is too large"},
		{"launcher runtime id", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Launcher.RuntimeID = strings.Repeat("r", n)
		},
			"delivery observation launcher runtime_id is too large"},
		{"target cwd", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Target.CWD = strings.Repeat("c", n)
		},
			"delivery observation cwd is too large"},
		{"target process started_at", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Target.Process.StartedAt = strings.Repeat("s", n)
		},
			"delivery observation process identity is too large"},
		{"target process executable", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Target.Process.Executable = strings.Repeat("e", n)
		},
			"delivery observation process identity is too large"},
		{"durable request id", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Request.DurableID = strings.Repeat("d", n)
		},
			"delivery observation durable request identity is invalid"},
		{"created_at", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.CreatedAt = handoffMutPadTimestamp(o.CreatedAt, n)
		},
			"delivery observation timestamps are too large"},
		{"updated_at", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.UpdatedAt = handoffMutPadTimestamp(o.UpdatedAt, n)
		},
			"delivery observation timestamps are too large"},
		{"state observed_at", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Ambiguous.ObservedAt = handoffMutPadTimestamp(o.Ambiguous.ObservedAt, n)
		}, "delivery observation ambiguous state is too large"},
		{"owner claim claimed_at", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			claimed(o, handoffMutPadTimestamp("2026-09-20T10:02:00Z", n))
		}, "delivery owner claim timestamp is too large"},
		{"actor session id", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.OwnerActor.SessionID = strings.Repeat("s", n)
		},
			"delivery observation actor session_id is too large"},
		{"actor agent id", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.OwnerActor.AgentID = strings.Repeat("a", n)
		},
			"delivery observation actor agent_id is too large"},
		{"actor session process started_at", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.OwnerActor.SessionProcess.StartedAt = strings.Repeat("s", n)
		}, "delivery observation actor process identity is too large"},
		{"actor session process executable", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.OwnerActor.SessionProcess.Executable = strings.Repeat("e", n)
		}, "delivery observation actor process identity is too large"},
		{"actor ancestry started_at", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.OwnerActor.ProcessAncestry = ancestry(strings.Repeat("s", n), "/bin/sh")
		}, "delivery observation actor process ancestry is too large"},
		{"actor ancestry executable", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.OwnerActor.ProcessAncestry = ancestry("2026-09-20T10:00:00Z", strings.Repeat("e", n))
		}, "delivery observation actor process ancestry is too large"},
		{"launcher path", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Launcher.Path = "/" + strings.Repeat("p", n-1)
		}, "delivery observation launcher path is too large"},
		{"launcher machine id", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Launcher.MachineID = strings.Repeat("m", n)
		}, "delivery observation launcher machine_id is too large"},
		{"launcher server id", func(o *issueopscontract.IssueOpsHandoffDeliveryObservation, n int) {
			o.Launcher.ServerID = strings.Repeat("v", n)
		}, "delivery observation launcher server_id is too large"},
	}
	for _, test := range tests {
		t.Run(test.name+" at limit is valid", func(t *testing.T) {
			observation := deliveryObservationFixture()
			test.edit(&observation, limit)
			if err := ValidateHandoffDeliveryObservation(observation); err != nil {
				t.Fatalf("exactly %d bytes must be accepted: %v", limit, err)
			}
		})
		t.Run(test.name+" over limit is rejected", func(t *testing.T) {
			observation := deliveryObservationFixture()
			test.edit(&observation, limit+1)
			if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != test.overError {
				t.Fatalf("limit+1 error=%v want %q", err, test.overError)
			}
		})
	}
}

func TestValidateHandoffDeliveryObservationStateStatusAtLimitIsInvalidNotTooLarge(t *testing.T) {
	observation := deliveryObservationFixture()
	observation.Ambiguous.Status = strings.Repeat("x", handoffDeliveryFieldLimit)
	if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != "delivery observation ambiguous state is invalid" {
		t.Fatalf("status at limit error=%v", err)
	}
	observation = deliveryObservationFixture()
	observation.Ambiguous.Evidence = strings.Repeat("x", handoffDeliveryFieldLimit)
	if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != "delivery observation ambiguous evidence is invalid" {
		t.Fatalf("evidence at limit error=%v", err)
	}
}

func TestValidateHandoffDeliveryObservationCmuxEndpoint(t *testing.T) {
	const parent = "/private/tmp"
	pathOfLength := func(n int) string { return parent + "/" + strings.Repeat("s", n-len(parent)-1) }
	t.Run("socket path at limit is valid", func(t *testing.T) {
		observation := cmuxDeliveryObservationFixture()
		observation.Launcher.EndpointIncarnation.Path = pathOfLength(handoffDeliveryFieldLimit)
		if err := ValidateHandoffDeliveryObservation(observation); err != nil {
			t.Fatalf("socket path at limit: %v", err)
		}
	})
	t.Run("socket path over limit is rejected", func(t *testing.T) {
		observation := cmuxDeliveryObservationFixture()
		observation.Launcher.EndpointIncarnation.Path = pathOfLength(handoffDeliveryFieldLimit + 1)
		if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != "cmux delivery observation socket endpoint incarnation is too large" {
			t.Fatalf("socket path over limit: %v", err)
		}
	})
	t.Run("zero ctime is invalid", func(t *testing.T) {
		observation := cmuxDeliveryObservationFixture()
		observation.Launcher.EndpointIncarnation.CTimeNS = 0
		if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != "cmux delivery observation socket endpoint incarnation is invalid" {
			t.Fatalf("zero ctime: %v", err)
		}
	})
}

func TestValidateHandoffDeliveryObservationTimingAtMaximumIsValid(t *testing.T) {
	const maximum = 10 * 60 * 1000
	observation := deliveryObservationFixture()
	observation.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{
		PreflightMS: maximum, WorkspaceCreateMS: maximum, TargetResolveMS: maximum, InputSendMS: maximum, ReceiverReceiptMS: maximum,
	}
	if err := ValidateHandoffDeliveryObservation(observation); err != nil {
		t.Fatalf("timing at maximum: %v", err)
	}
}

func TestValidateHandoffDeliveryObservationHerdrNativeReceiptIsValid(t *testing.T) {
	observation := deliveryObservationFixture()
	observation.Launcher.Name = issueopscontract.IssueOpsHandoffDeliveryLauncherHerdr
	observation.NativeTurnObserved = deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateObserved, issueopscontract.IssueOpsHandoffDeliveryEvidenceNativeReceipt)
	if err := ValidateHandoffDeliveryObservation(observation); err != nil {
		t.Fatalf("herdr native receipt must prove the native turn: %v", err)
	}
}

func TestMergeHandoffDeliveryObservationKeepsKnownWindowAndCWDWhenNextOmitsThem(t *testing.T) {
	current := deliveryObservationFixture()
	current.Target.WindowID = "window-1"
	current.Target.CWD = "/repo/worktree"
	next := current
	next.Target.WindowID = ""
	next.Target.CWD = ""
	next.UpdatedAt = "2026-09-20T10:06:00Z"
	merged, decision := MergeHandoffDeliveryObservation(current, next)
	if !decision.Accepted {
		t.Fatalf("merge rejected: %+v", decision)
	}
	if merged.Target.WindowID != "window-1" || merged.Target.CWD != "/repo/worktree" {
		t.Fatalf("known target identity was dropped: %+v", merged.Target)
	}

	current = deliveryObservationFixture()
	next = current
	next.Target.WindowID = "window-2"
	next.Target.CWD = "/repo/other"
	next.UpdatedAt = "2026-09-20T10:06:00Z"
	merged, decision = MergeHandoffDeliveryObservation(current, next)
	if !decision.Accepted || merged.Target.WindowID != "window-2" || merged.Target.CWD != "/repo/other" {
		t.Fatalf("missing target identity was not filled: decision=%+v target=%+v", decision, merged.Target)
	}
}
