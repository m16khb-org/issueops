package mcp

import (
	"issueops/internal/testsupport"
	"testing"
)

func TestToolsContextIsByteDeterministic(t *testing.T) {
	stable, _, err := testsupport.ContextSerializationStable(func() any { return Build().Tools })
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatal("mcp tools immutable prefix is not byte-deterministic across builds")
	}
}
