package issueopsrecord

import (
	"errors"
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
	statecontract "issueops/internal/contract/state"
)

func TestLeaseCodecPreservesStrictShapeAndDomainRejection(t *testing.T) {
	valid := leasecontract.Record{SchemaVersion: leasecontract.SchemaVersion, ID: "io-lease-codec", Execution: &leasecontract.Execution{
		Mode:      "direct",
		Workspace: leasecontract.Workspace{SourceRoot: "/source", Root: "/worktree", Branch: "branch", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "2026-08-03T00:00:00Z"},
		Lease:     leasecontract.Lease{Generation: 1, Status: "released"},
		Selection: leaseSelectionFixture("direct"),
	}}
	encoded, err := EncodeLease(valid)
	if err != nil {
		t.Fatalf("encode valid lease: %v", err)
	}
	if _, err := DecodeLease(valid.ID, encoded); err != nil {
		t.Fatalf("decode valid lease: %v", err)
	}
	invalid := valid
	execution := *valid.Execution
	execution.Lease.Generation = 0
	invalid.Execution = &execution
	if _, err := EncodeLease(invalid); err == nil || err.Error() != "lease generation must start at 1" {
		t.Fatalf("encode invariant error=%v", err)
	}
	if _, err := DecodeLease(valid.ID, []byte(strings.Replace(string(encoded), `"generation": 1`, `"generation": 0`, 1))); !errors.Is(err, statecontract.ErrInvalidState) || err.Error() != "invalid state" {
		t.Fatalf("decode invariant error=%v", err)
	}
	if _, err := DecodeLease(valid.ID, append(encoded[:len(encoded)-1], []byte(`,"unknown":1}`)...)); !errors.Is(err, statecontract.ErrInvalidState) {
		t.Fatalf("unknown field error=%v", err)
	}
}
