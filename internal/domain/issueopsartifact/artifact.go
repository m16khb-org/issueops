package issueopsartifact

import (
	"fmt"
	"strings"

	issueopsartifactcontract "issueops/internal/contract/issueopsartifact"
	"issueops/internal/domain/secretdetection"
)

func NormalizeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	switch name {
	case "plan", "spec", "verified-execution-loop", "architecture":
		return name, nil
	default:
		return "", fmt.Errorf("artifact name must be plan|spec|verified-execution-loop|architecture")
	}
}

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

func CanStage(record issueopsartifactcontract.Record, name string) bool {
	if record.Phase == issueopsartifactcontract.PhaseDone {
		return false
	}
	return record.Execution == nil
}
