// Package issueopsartifact holds the pure rules for staging cycle artifacts:
// allowed names, content validation, and when staging is permitted. It sits in
// the domain layer and performs no I/O.
package issueopsartifact

import (
	"fmt"
	"strings"

	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
	"issueops/internal/domain/secretdetection"
)

// NormalizeName trims name and accepts only plan, spec,
// verified-execution-loop, or architecture.
func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch name {
	case "plan", "spec", "verified-execution-loop", "architecture":
		return name, nil
	default:
		return "", fmt.Errorf("artifact name must be plan|spec|verified-execution-loop|architecture")
	}
}

// ValidateContent rejects empty content, content over the contract's MaxBytes,
// and content that contains a secret-like value.
func ValidateContent(content []byte) error {
	if len(content) == 0 {
		return fmt.Errorf("artifact content is empty")
	}
	if len(content) > issueopsartifactcontract.MaxBytes {
		return fmt.Errorf("artifact exceeds %d bytes", issueopsartifactcontract.MaxBytes)
	}
	if match, found := secretdetection.Find(string(content)); found {
		return fmt.Errorf("artifact content contains a secret-like value (%s) at line %d; redact it before staging", match.Kind, match.Line)
	}
	return nil
}

// CanStage reports whether record may accept a staged artifact: staging is
// closed once the cycle is done or an execution lease exists.
func CanStage(record issueopsartifactcontract.Record, name string) bool {
	if record.Phase == issueopsartifactcontract.PhaseDone {
		return false
	}
	return record.Execution == nil
}
