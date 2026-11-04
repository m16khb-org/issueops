package issueopsapp

import (
	"strconv"
	"strings"
	"time"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

func handoffDeliveryEventClock(now func() time.Time) func() time.Time {
	timestamp := now()
	return func() time.Time { return timestamp }
}

func handoffDeliveryObserved(now func() time.Time, evidence string) issueopscontract.HandoffDeliveryState {
	return issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateObserved, ObservedAt: now().UTC().Format(time.RFC3339Nano), Evidence: evidence}
}

func foldHandoffAuditForTest(stateRoot, lifecycleID, lineageID string) (map[string]issueopscontract.HandoffDeliveryObservation, []issueopscontract.HandoffDeliveryDecision, error) {
	observations, err := (newHandoffDeliveryAudit(stateRoot)).ReadFor(lifecycleID, lineageID)
	folded, decisions := issueopsdomain.FoldHandoffDeliveryObservations(observations)
	return folded, decisions, err
}

// manualHerdrHandoffObservation은 trace handoff-delivery가 기록하는 것과 같은 모양의
// Herdr launcher manual 관찰을 만든다. 처음에는 call_staged만 관찰된 상태다.
func manualHerdrHandoffObservation(lifecycleID string, generation uint64, attempt string, at time.Time) issueopscontract.HandoffDeliveryObservation {
	created := at.UTC().Format(time.RFC3339Nano)
	generationText := strconv.FormatUint(generation, 10)
	return issueopscontract.HandoffDeliveryObservation{
		SchemaVersion:  issueopscontract.HandoffDeliverySchemaVersion,
		AttemptID:      "manual-direct:" + lifecycleID + ":" + generationText + ":herdr:" + attempt,
		LineageID:      "manual-direct:generation:" + generationText + ":herdr:" + attempt,
		LifecycleID:    lifecycleID,
		PromptSHA256:   strings.Repeat("a", 64),
		MaterialSHA256: strings.Repeat("b", 64),
		Launcher: issueopscontract.HandoffDeliveryLauncher{
			Name: issueopscontract.HandoffDeliveryLauncherHerdr, Version: "0.5.0", Path: "/usr/local/bin/herdr",
			RuntimeID: "runtime-herdr", MachineID: "machine-herdr", ServerID: "server-herdr",
		},
		Target:             issueopscontract.HandoffDeliveryTarget{TerminalID: "terminal-herdr", PaneID: "pane-herdr"},
		ExpectedOwnerHost:  "codex",
		SourceGeneration:   generation,
		CreatedAt:          created,
		UpdatedAt:          created,
		Receipt:            issueopscontract.HandoffDeliveryReceipt{Location: "audit/herdr.json", Digest: strings.Repeat("d", 64)},
		CallStaged:         issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateObserved, ObservedAt: created, Evidence: issueopscontract.HandoffDeliveryEvidenceExternalCallStaged},
		InputAccepted:      issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateNotObserved},
		NativeTurnObserved: issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateNotObserved},
		OwnerClaimed:       issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateNotObserved},
		Ambiguous:          issueopscontract.HandoffDeliveryState{Status: issueopscontract.HandoffDeliveryStateNotObserved},
	}
}
