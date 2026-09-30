package nativeactivation

import (
	"testing"

	activationcontract "issueops/internal/contract/nativeactivation"
)

func TestPendingAndReceiptTransitionDecisions(t *testing.T) {
	staged := BinaryIdentity{Executable: "/repo/bin/.issueops.activate-1", SHA256: "digest", Mode: 0700, Size: 12, Device: 1, Inode: 2}
	active := staged
	active.Executable = "/repo/bin/issueops"
	facts := TransitionFacts{StateRoot: "/state", IssueOpsRoot: "/repo", TargetBinary: active.Executable, TransitionID: "transition", Active: active, CatalogSHA256: "catalog", Evidence: []activationcontract.Evidence{{Host: "codex", Surface: "mcp", Path: "/codex"}}}
	pending := PendingSnapshot{SchemaVersion: RecordSchemaVersion, StateRoot: facts.StateRoot, IssueOpsRoot: facts.IssueOpsRoot, TargetBinary: facts.TargetBinary, Candidate: staged, TransitionID: facts.TransitionID, StartedAt: "time"}
	if !PendingMatches(pending, facts) {
		t.Fatal("staged candidate with identical physical content must seal")
	}
	drift := facts
	drift.Active.Inode++
	if PendingMatches(pending, drift) {
		t.Fatal("binary drift must reject pending transition")
	}
	wrong := pending
	wrong.SchemaVersion++
	if PendingMatches(wrong, facts) {
		t.Fatal("unsupported record schema must reject")
	}
	wrong = pending
	wrong.TransitionID = "other"
	if PendingMatches(wrong, facts) {
		t.Fatal("different transition must reject")
	}
	receipt := ReceiptSnapshot{SchemaVersion: RecordSchemaVersion, StateRoot: facts.StateRoot, IssueOpsRoot: facts.IssueOpsRoot, TargetBinary: facts.TargetBinary, Binary: active, TransitionID: facts.TransitionID, CatalogSHA256: facts.CatalogSHA256, Evidence: facts.Evidence, SealedAt: "time"}
	if !ReceiptMatches(receipt, facts) {
		t.Fatal("identical receipt must be reusable")
	}
	changed := facts
	changed.Evidence = []activationcontract.Evidence{{Host: "claude", Surface: "mcp", Path: "/claude"}}
	if ReceiptMatches(receipt, changed) {
		t.Fatal("changed readback must reject receipt reuse")
	}
	changed = facts
	changed.Active.Executable = staged.Executable
	if ReceiptMatches(receipt, changed) {
		t.Fatal("receipt reuse requires exact binary identity")
	}
}
