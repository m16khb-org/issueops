package agentmodel

import (
	"strings"
	"testing"
)

// Probe: `model resolve` without --host read as unknown host "".
func TestResolveNamesAMissingHost(t *testing.T) {
	if _, err := Resolve(ResolveInput{Role: "plan-review"}); err == nil || !strings.HasPrefix(err.Error(), "--host is required") {
		t.Fatalf("err = %v", err)
	}
	if _, err := Resolve(ResolveInput{Host: "vim", Role: "plan-review"}); err == nil || !strings.HasPrefix(err.Error(), `unknown host "vim"`) {
		t.Fatalf("err = %v", err)
	}
}
