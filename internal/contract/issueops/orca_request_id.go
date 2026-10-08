package issueops

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
)

var orcaRequestUUIDPattern = sync.OnceValue(func() *regexp.Regexp {
	return regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
})

func ValidateOrcaRequestID(value string) error {
	if !orcaRequestUUIDPattern().MatchString(strings.TrimSpace(value)) {
		return fmt.Errorf("Orca durable request UUID is invalid")
	}
	return nil
}

func ValidateOrcaRetryRequestID(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return ValidateOrcaRequestID(value)
}
