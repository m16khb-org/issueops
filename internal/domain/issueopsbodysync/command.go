package issueopsbodysync

import (
	"fmt"
	"strings"
)

func ValidateCommandBody(body string) error {
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("a replacement body is required: pass --body or --body-file")
	}
	return nil
}
