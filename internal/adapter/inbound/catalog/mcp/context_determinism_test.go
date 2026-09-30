package mcp

import (
	contextregion "issueops/internal/domain/contextregion"
	"testing"
)

func TestToolsContextIsByteDeterministic(t *testing.T) {
	stable, _, err := contextregion.ContextSerializationStable(func() any { return Build().Tools })
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatal("mcp tools immutable prefix is not byte-deterministic across builds")
	}
}
