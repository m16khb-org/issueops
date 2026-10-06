package testsupport_test

import (
	"testing"

	contract "issueops/internal/contract/qualitycatalog"
	"issueops/internal/testsupport"
)

// Teeth: a VerifyWith that is ONLY model self-critique must be REJECTED for a
// tool_signal candidate — including phrasings that carry no denylist keyword.
// If any of these pass, the guard is vacuous and this test goes red.
func TestVerifyWithGroundedRejectsSelfCritique(t *testing.T) {
	selfCritiqueOnly := [][]string{
		{"verified by inspection"},
		{"checked it by hand"},
		{"I judged the diff correct"},
		{"manually validated by reading"},
		{"agent reviewed the output and it seemed correct"},
		{"the reviewer confirms it reads well"},
		{"looks correct to me"},
		{"the model assessed the change as reasonable"},
	}
	for _, vw := range selfCritiqueOnly {
		if err := testsupport.VerifyWithGrounded(contract.ToolSignalKind, vw); err == nil {
			t.Fatalf("self-critique-only VerifyWith must be rejected for tool_signal: %v", vw)
		}
		if err := testsupport.VerifyWithGrounded(contract.DocArtifactKind, vw); err == nil {
			t.Fatalf("self-critique-only VerifyWith must be rejected for doc_artifact: %v", vw)
		}
	}

	for _, vw := range [][]string{
		{"go test ./internal/application/state ./internal/adapter/outbound/state -count=1"},
		{"response_contract golden"},
		{"markdown fixture lint"},
		{"temp HOME install smoke"},
		{"issueops self-verify --full --iterations=10 --target-score=95"},
	} {
		if err := testsupport.VerifyWithGrounded(contract.ToolSignalKind, vw); err != nil {
			t.Fatalf("real tool signal must pass: %v -> %v", vw, err)
		}
	}

	// Concrete documentary deliverables pass for doc_artifact but NOT for
	// tool_signal (a doc artifact is not an executable signal).
	for _, vw := range [][]string{
		{"ADR decision entry", "rollback criteria"},
		{"README install/update/rollback section"},
		{"dogfooding notes document", "Codex inspect/docs/state transcript"},
	} {
		if err := testsupport.VerifyWithGrounded(contract.DocArtifactKind, vw); err != nil {
			t.Fatalf("doc artifact must pass for doc_artifact: %v -> %v", vw, err)
		}
	}

	if err := testsupport.VerifyWithGrounded(contract.ToolSignalKind, nil); err == nil {
		t.Fatal("empty VerifyWith must be rejected")
	}
	if err := testsupport.VerifyWithGrounded(contract.ToolSignalKind, []string{"  "}); err == nil {
		t.Fatal("blank entry must be rejected")
	}
	if err := testsupport.VerifyWithGrounded(contract.VerificationKind("nonsense"), []string{"go test ./..."}); err == nil {
		t.Fatal("unknown verification kind must be rejected")
	}
}
