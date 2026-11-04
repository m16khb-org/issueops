package issueops

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
	"issueops/internal/testsupport"
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
	created.Timing = &issueopscontract.HandoffDeliveryTiming{
		PreflightMS: 12, WorkspaceCreateMS: 31, TargetResolveMS: 8,
	}
	created.Receipt = issueopscontract.HandoffDeliveryReceipt{Location: "audit/cmux/create.json", Digest: strings.Repeat("b", 64)}

	resolved, decision := MergeHandoffDeliveryObservation(staged, created)
	if !decision.Accepted {
		t.Fatalf("workspace enrichment rejected: %+v", decision)
	}

	accepted := resolved
	accepted.UpdatedAt = "2026-09-20T10:00:03Z"
	accepted.InputAccepted = deliveryStateAt(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceRawInput, accepted.UpdatedAt)
	accepted.Target.Process = &issueopscontract.NativeProcessReceipt{
		PID: 8080, StartedAt: "2026-09-20T10:00:02Z", Executable: "/opt/local/bin/codex",
	}
	accepted.Target.ProcessIncarnation = "8080:2026-09-20T10:00:02Z"
	accepted.Timing = &issueopscontract.HandoffDeliveryTiming{
		PreflightMS: 12, WorkspaceCreateMS: 31, TargetResolveMS: 8, InputSendMS: 6, ReceiverReceiptMS: 19,
	}
	accepted.Receipt = issueopscontract.HandoffDeliveryReceipt{Location: "audit/cmux/input.json", Digest: strings.Repeat("c", 64)}

	merged, decision := MergeHandoffDeliveryObservation(resolved, accepted)
	if !decision.Accepted {
		t.Fatalf("input receipt must remain non-authoritative: %+v", decision)
	}
	if !reflect.DeepEqual(merged.Target, accepted.Target) || !reflect.DeepEqual(merged.Timing, accepted.Timing) {
		t.Fatalf("cmux enrichment was not preserved: target=%+v timing=%+v", merged.Target, merged.Timing)
	}
	if merged.NativeTurnObserved.Status != issueopscontract.HandoffDeliveryStateNotObserved ||
		merged.OwnerClaimed.Status != issueopscontract.HandoffDeliveryStateNotObserved {
		t.Fatalf("raw input promoted authority evidence: %+v", merged)
	}
}

func TestCmuxHandoffDeliveryRejectsTargetOrTimingRewrite(t *testing.T) {
	base := cmuxDeliveryObservationFixture()
	base.Target.WorkspaceID = "11111111-1111-4111-8111-111111111111"
	base.Target.SurfaceID = "22222222-2222-4222-8222-222222222222"
	base.Timing = &issueopscontract.HandoffDeliveryTiming{PreflightMS: 12, WorkspaceCreateMS: 31}

	tests := []struct {
		name string
		edit func(*issueopscontract.HandoffDeliveryObservation)
		want string
	}{
		{
			name: "window",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) { next.Target.WindowID = "window-other" },
			want: "delivery observation target identity changed",
		},
		{
			name: "workspace",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) {
				next.Target.WorkspaceID = "33333333-3333-4333-8333-333333333333"
			},
			want: "delivery observation target identity changed",
		},
		{
			name: "cwd",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) {
				next.Target.CWD = testsupport.AbsPath("tmp", "wrong")
			},
			want: "delivery observation target identity changed",
		},
		{
			name: "timing",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) { next.Timing.PreflightMS++ },
			want: "delivery observation timing changed",
		},
		{
			name: "endpoint incarnation",
			edit: func(next *issueopscontract.HandoffDeliveryObservation) {
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
	observation.InputAccepted = deliveryStateAt(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceRawInput, observation.UpdatedAt)
	if err := ValidateHandoffDeliveryObservation(observation); err == nil || err.Error() != "cmux input receipt requires exact workspace and surface identity" {
		t.Fatalf("missing target error=%v", err)
	}
}

// cmuxTestParent is the private socket parent directory used by cmux fixtures.
var cmuxTestParent = testsupport.AbsPath("private", "tmp")

func cmuxEndpointIncarnationFixture() *issueopscontract.HandoffDeliveryEndpointIncarnation {
	return &issueopscontract.HandoffDeliveryEndpointIncarnation{
		Path: filepath.Join(cmuxTestParent, "cmux-test.sock"), Kind: "unix_socket", Device: 1, Inode: 2, CTimeNS: 3,
		OwnerUID: 501, OwnerGID: 20, Mode: 0o600, ParentPath: cmuxTestParent, ParentDevice: 1,
		ParentInode: 1, ParentOwnerUID: 0, ParentOwnerGID: 0, ParentMode: 0o1777,
	}
}

func cmuxDeliveryObservationFixture() issueopscontract.HandoffDeliveryObservation {
	return issueopscontract.HandoffDeliveryObservation{
		SchemaVersion:  issueopscontract.HandoffDeliverySchemaVersion,
		AttemptID:      "manual-direct:io-cmux:7:cmux:attempt",
		LineageID:      "manual-direct:generation:7:cmux:window:window-1:prompt:" + strings.Repeat("a", 64),
		LifecycleID:    "io-cmux",
		PromptSHA256:   strings.Repeat("a", 64),
		MaterialSHA256: strings.Repeat("e", 64),
		Launcher: issueopscontract.HandoffDeliveryLauncher{
			Name: issueopscontract.HandoffDeliveryLauncherCmux, Version: "0.64.10",
			Path:                "/Applications/cmux.app/Contents/Resources/bin/cmux",
			EndpointIncarnation: cmuxEndpointIncarnationFixture(),
		},
		Target: issueopscontract.HandoffDeliveryTarget{
			WindowID: "window-1", CWD: testsupport.AbsPath("repo", ".issueops-worktrees", "io-cmux"),
		},
		ExpectedOwnerHost: "codex",
		SourceGeneration:  7,
		CreatedAt:         "2026-09-20T10:00:01Z",
		UpdatedAt:         "2026-09-20T10:00:01Z",
		Receipt: issueopscontract.HandoffDeliveryReceipt{
			Location: "audit/cmux/staged.json", Digest: strings.Repeat("d", 64),
		},
		CallStaged:         deliveryStateAt(issueopscontract.HandoffDeliveryStateObserved, issueopscontract.HandoffDeliveryEvidenceExternalCallStaged, "2026-09-20T10:00:01Z"),
		InputAccepted:      deliveryState(issueopscontract.HandoffDeliveryStateNotObserved, ""),
		NativeTurnObserved: deliveryState(issueopscontract.HandoffDeliveryStateNotObserved, ""),
		OwnerClaimed:       deliveryState(issueopscontract.HandoffDeliveryStateNotObserved, ""),
		Ambiguous:          deliveryState(issueopscontract.HandoffDeliveryStateNotObserved, ""),
	}
}

func deliveryStateAt(status, evidence, observedAt string) issueopscontract.HandoffDeliveryState {
	state := issueopscontract.HandoffDeliveryState{Status: status}
	if evidence != "" {
		state.Evidence = evidence
		state.ObservedAt = observedAt
	}
	return state
}
