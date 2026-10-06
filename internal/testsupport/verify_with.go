package testsupport

import (
	"fmt"
	"strings"

	contract "issueops/internal/contract/qualitycatalog"
)

// toolSignalMarkers are CONCRETE executable-verification tokens (runnable
// commands and test/contract artifact kinds). They deliberately EXCLUDE generic
// verbs (verify/validate/inspect/check/review) and generic nouns
// (notes/document/criteria/references/section) that self-critique prose uses as
// readily as a real tool signal: the burden of proof is on NAMING a concrete
// external mechanism, not on dodging a forbidden-phrase denylist.
var toolSignalMarkers = []string{
	"go test", "go build", "go vet", "issueops", "harness ", "skill ",
	"issueops install", "install tests", "npm ", "./",
	"_test", "test", "golden", "contract", "fixture", "smoke", "lint",
	"coverage", "-cover", "-race", "-count", "--json", "roundtrip",
	"benchmark", "schema", "self-verify", "self-augment", "quick_validate",
	"policy_check", "policy audit", "redaction audit", "qa gate",
	"internal/", "cmd/", "mcp",
}

// docArtifactMarkers are concrete documentary deliverables a DocArtifact
// candidate must name (a file/section/record), not bare self-critique.
var docArtifactMarkers = []string{
	"adr", "readme", "checklist", "matrix", "transcript", "decision entry",
	"decision record", "notes document", "dogfooding notes", ".md",
}

// VerifyWithGrounded enforces the self-correction guardrail (inherits v1 S5/S6):
// a candidate's VerifyWith must NAME at least one external verification
// mechanism appropriate to its kind, never model self-critique. This is catalog
// hygiene (the string names a mechanism); whether the mechanism exists and
// PASSES is the separate execution gate enforced by `issueops self-verify`
// / CI.
func VerifyWithGrounded(kind contract.VerificationKind, verifyWith []string) error {
	if len(verifyWith) == 0 {
		return fmt.Errorf("verify_with is empty")
	}
	for _, entry := range verifyWith {
		if strings.TrimSpace(entry) == "" {
			return fmt.Errorf("verify_with has a blank entry")
		}
	}
	switch kind {
	case contract.ToolSignalKind:
		if !verifyWithNamesAny(verifyWith, toolSignalMarkers) {
			return fmt.Errorf("tool_signal candidate must name an executable verification (go test / golden / contract / smoke / lint / self-verify / ...), not self-critique: %v", verifyWith)
		}
	case contract.DocArtifactKind:
		if !verifyWithNamesAny(verifyWith, docArtifactMarkers) {
			return fmt.Errorf("doc_artifact candidate must name a concrete deliverable (ADR / README / checklist / matrix / transcript), not self-critique: %v", verifyWith)
		}
	default:
		return fmt.Errorf("unknown verification kind %q", kind)
	}
	return nil
}

func verifyWithNamesAny(verifyWith, markers []string) bool {
	for _, entry := range verifyWith {
		lower := strings.ToLower(entry)
		for _, marker := range markers {
			if strings.Contains(lower, marker) {
				return true
			}
		}
	}
	return false
}
