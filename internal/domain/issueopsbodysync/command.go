package issueopsbodysync

import (
	"fmt"
	"strings"
)

// ValidateCommandBody rejects a replacement body that is empty or only whitespace.
func ValidateCommandBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("a replacement body is required: pass --body or --body-file")
	}
	return nil
}
