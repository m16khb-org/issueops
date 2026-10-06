package contractcli

import (
	"issueops/internal/testsupport"
	"testing"
)

func TestCompatibilityContractContextIsByteDeterministic(t *testing.T) {
	stable, _, err := testsupport.ContextSerializationStable(func() any { return testCompatibilityContract() })
	if err != nil {
		t.Fatal(err)
	}
	if !stable {
		t.Fatal("contract_schema immutable prefix is not byte-deterministic across builds")
	}
}
