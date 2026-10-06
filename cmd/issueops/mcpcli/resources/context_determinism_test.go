package resources

import (
	mcpcatalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/testsupport"
	"testing"
)

func TestResourcesContextIsByteDeterministic(t *testing.T) {
	stable, _, err := testsupport.ContextSerializationStable(func() any { return mcpcatalog.Build().Resources })
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatal("mcp resources immutable prefix is not byte-deterministic across builds")
	}
}
