package issueops

import (
	"reflect"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestCmuxHandoffDeliveryStagesKnownWindowBeforeWorkspaceExists(t *testing.T) {
	observation := cmuxDeliveryObservationFixture()
	if err := ValidateHandoffDeliveryObservation(observation); err != nil {
		t.Fatalf("validate staged cmux observation: %v", err)
	}
	if observation.Target.WorkspaceID != "" || observation.Target.SurfaceID != "" || observation.Target.Process != nil {
		t.Fatalf("pre-create stage invented receiver identity: %+v", observation.Target)
	}
	if observation.Request.DurableID != "" {
		t.Fatalf("cmux 0.64.10 does not provide a durable request id: %+v", observation.Request)
	}
}

func TestCmuxHandoffDeliveryRequiresObservedSocketEndpointIncarnation(t *testing.T) {
	observation := cmuxDeliveryObservationFixture()
	observation.Launcher.EndpointIncarnation = nil
	if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != "cmux delivery observation socket endpoint incarnation is required" {
		t.Fatalf("missing endpoint error=%v", err)
	}
}

func TestCmuxHandoffDeliveryMonotonicallyEnrichesExactTargetAndTiming(t *testing.T) {
	staged := cmuxDeliveryObservationFixture()
	created := staged
	created.UpdatedAt = "2026-09-20T10:00:02Z"
	created.Target.WorkspaceID = "11111111-1111-4111-8111-111111111111"
	created.Target.SurfaceID = "22222222-2222-4222-8222-222222222222"
	created.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{
		PreflightMS: 12, WorkspaceCreateMS: 31, TargetResolveMS: 8,
	}
	created.Receipt = issueopscontract.IssueOpsHandoffDeliveryReceipt{Location: "audit/cmux/create.json", Digest: strings.Repeat("b", 64)}

	resolved, decision := MergeHandoffDeliveryObservation(staged, created)
	if !decision.Accepted {
		t.Fatalf("workspace enrichment rejected: %+v", decision)
	}

	accepted := resolved
	accepted.UpdatedAt = "2026-09-20T10:00:03Z"
	accepted.InputAccepted = deliveryStateAt(issueopscontract.IssueOpsHandoffDeliveryStateObserved, issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput, accepted.UpdatedAt)
	accepted.Target.Process = &issueopscontract.NativeProcessReceipt{
		PID: 8080, StartedAt: "2026-09-20T10:00:02Z", Executable: "/opt/local/bin/codex",
	}
	accepted.Target.ProcessIncarnation = "8080:2026-09-20T10:00:02Z"
	accepted.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{
		PreflightMS: 12, WorkspaceCreateMS: 31, TargetResolveMS: 8, InputSendMS: 6, ReceiverReceiptMS: 19,
	}
	accepted.Receipt = issueopscontract.IssueOpsHandoffDeliveryReceipt{Location: "audit/cmux/input.json", Digest: strings.Repeat("c", 64)}

	merged, decision := MergeHandoffDeliveryObservation(resolved, accepted)
	if !decision.Accepted || decision.OwnerAuthorized || decision.RetryAuthorized {
		t.Fatalf("input receipt must remain non-authoritative: %+v", decision)
	}
	if !reflect.DeepEqual(merged.Target, accepted.Target) || !reflect.DeepEqual(merged.Timing, accepted.Timing) {
		t.Fatalf("cmux enrichment was not preserved: target=%+v timing=%+v", merged.Target, merged.Timing)
	}
	if merged.NativeTurnObserved.Status != issueopscontract.IssueOpsHandoffDeliveryStateNotObserved ||
		merged.OwnerClaimed.Status != issueopscontract.IssueOpsHandoffDeliveryStateNotObserved {
		t.Fatalf("raw input promoted authority evidence: %+v", merged)
	}
}

func TestCmuxHandoffDeliveryRejectsTargetOrTimingRewrite(t *testing.T) {
	base := cmuxDeliveryObservationFixture()
	base.Target.WorkspaceID = "11111111-1111-4111-8111-111111111111"
	base.Target.SurfaceID = "22222222-2222-4222-8222-222222222222"
	base.Timing = &issueopscontract.IssueOpsHandoffDeliveryTiming{PreflightMS: 12, WorkspaceCreateMS: 31}

	tests := []struct {
		name string
		edit func(*issueopscontract.IssueOpsHandoffDeliveryObservation)
		want string
	}{
		{
			name: "window",
			edit: func(next *issueopscontract.IssueOpsHandoffDeliveryObservation) { next.Target.WindowID = "window-other" },
			want: "delivery observation target identity changed",
		},
		{
			name: "workspace",
			edit: func(next *issueopscontract.IssueOpsHandoffDeliveryObservation) {
				next.Target.WorkspaceID = "33333333-3333-4333-8333-333333333333"
			},
			want: "delivery observation target identity changed",
		},
		{
			name: "cwd",
			edit: func(next *issueopscontract.IssueOpsHandoffDeliveryObservation) { next.Target.CWD = "/tmp/wrong" },
			want: "delivery observation target identity changed",
		},
		{
			name: "timing",
			edit: func(next *issueopscontract.IssueOpsHandoffDeliveryObservation) { next.Timing.PreflightMS++ },
			want: "delivery observation timing changed",
		},
		{
			name: "endpoint incarnation",
			edit: func(next *issueopscontract.IssueOpsHandoffDeliveryObservation) {
				next.Launcher.EndpointIncarnation.Inode++
			},
			want: "delivery observation launcher identity changed",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			next := base
			timing := *base.Timing
			next.Timing = &timing
			endpoint := *base.Launcher.EndpointIncarnation
			next.Launcher.EndpointIncarnation = &endpoint
			test.edit(&next)
			_, decision := MergeHandoffDeliveryObservation(base, next)
			if decision.Accepted || !reflect.DeepEqual(decision.RejectReasons, []string{test.want}) {
				t.Fatalf("decision=%+v", decision)
			}
		})
	}
}

func TestCmuxInputAcceptedRequiresExactCreatedTarget(t *testing.T) {
	observation := cmuxDeliveryObservationFixture()
	observation.UpdatedAt = "2026-09-20T10:00:03Z"
	observation.InputAccepted = deliveryStateAt(issueopscontract.IssueOpsHandoffDeliveryStateObserved, issueopscontract.IssueOpsHandoffDeliveryEvidenceRawInput, observation.UpdatedAt)
	if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != "cmux input receipt requires exact workspace and surface identity" {
		t.Fatalf("missing target error=%v", err)
	}
}

func cmuxDeliveryObservationFixture() issueopscontract.IssueOpsHandoffDeliveryObservation {
	return issueopscontract.IssueOpsHandoffDeliveryObservation{
		SchemaVersion:  issueopscontract.IssueOpsHandoffDeliverySchemaVersion,
		AttemptID:      "manual-direct:io-cmux:7:cmux:attempt",
		LineageID:      "manual-direct:generation:7:cmux:window:window-1:prompt:" + strings.Repeat("a", 64),
		LifecycleID:    "io-cmux",
		PromptSHA256:   strings.Repeat("a", 64),
		MaterialSHA256: strings.Repeat("e", 64),
		Launcher: issueopscontract.IssueOpsHandoffDeliveryLauncher{
			Name: issueopscontract.IssueOpsHandoffDeliveryLauncherCmux, Version: "0.64.10",
			Path: "/Applications/cmux.app/Contents/Resources/bin/cmux",
			EndpointIncarnation: &issueopscontract.IssueOpsHandoffDeliveryEndpointIncarnation{
				Path: "/private/tmp/cmux-test.sock", Kind: "unix_socket", Device: 1, Inode: 2, CTimeNS: 3,
				OwnerUID: 501, OwnerGID: 20, Mode: 0o600, ParentPath: "/private/tmp", ParentDevice: 1,
				ParentInode: 1, ParentOwnerUID: 0, ParentOwnerGID: 0, ParentMode: 0o1777,
			},
		},
		Target: issueopscontract.IssueOpsHandoffDeliveryTarget{
			WindowID: "window-1", CWD: "/repo/.issueops-worktrees/io-cmux",
		},
		ExpectedOwnerHost: "codex",
		SourceGeneration:  7,
		CreatedAt:         "2026-09-20T10:00:01Z",
		UpdatedAt:         "2026-09-20T10:00:01Z",
		Receipt: issueopscontract.IssueOpsHandoffDeliveryReceipt{
			Location: "audit/cmux/staged.json", Digest: strings.Repeat("d", 64),
		},
		CallStaged:         deliveryStateAt(issueopscontract.IssueOpsHandoffDeliveryStateObserved, issueopscontract.IssueOpsHandoffDeliveryEvidenceExternalCallStaged, "2026-09-20T10:00:01Z"),
		InputAccepted:      deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateNotObserved, ""),
		NativeTurnObserved: deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateNotObserved, ""),
		OwnerClaimed:       deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateNotObserved, ""),
		Ambiguous:          deliveryState(issueopscontract.IssueOpsHandoffDeliveryStateNotObserved, ""),
	}
}

func deliveryStateAt(status, evidence, observedAt string) issueopscontract.IssueOpsHandoffDeliveryState {
	state := issueopscontract.IssueOpsHandoffDeliveryState{Status: status}
	if evidence != "" {
		state.Evidence = evidence
		state.ObservedAt = observedAt
	}
	return state
}
