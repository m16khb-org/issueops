package issueopslease

import (
	"strings"
	"testing"

	leasecontract "issueops/internal/contract/issueopslease"
)

func TestSealedClaimContextRejectsIdentityAndBodyDrift(t *testing.T) {
	record := leasecontract.Record{ID: "cycle", IssueURL: "https://example.test/issues/1", Execution: &leasecontract.Execution{Mode: "orca"}}
	record.Execution.Lease.Generation = 2
	record.Execution.Workspace.Branch = "feature"
	record.Execution.Workspace.BaseHead = "base"
	// SHA-256 of "abc", independent of the production digest helper.
	digest := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	packet := leasecontract.ClaimContextPacket{SchemaVersion: leasecontract.SchemaVersion, LifecycleID: "cycle", Mode: "orca", Branch: "feature", BaseHead: "base", LeaseGeneration: 2, Issue: leasecontract.ClaimPacketIssue{URL: record.IssueURL, Body: "abc", BodySHA256: digest}}
	if err := ValidateClaimPacketIdentity(record, packet, true, true, digest); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name   string
		change func(*leasecontract.ClaimContextPacket)
		want   string
	}{
		{"generation", func(p *leasecontract.ClaimContextPacket) { p.LeaseGeneration++ }, "identity mismatch"},
		{"branch", func(p *leasecontract.ClaimContextPacket) { p.Branch = "other" }, "identity mismatch"},
		{"lifecycle", func(p *leasecontract.ClaimContextPacket) { p.LifecycleID = "other" }, "identity mismatch"},
		{"schema", func(p *leasecontract.ClaimContextPacket) { p.SchemaVersion = 0 }, "identity mismatch"},
		{"issue", func(p *leasecontract.ClaimContextPacket) { p.Issue.URL = "other" }, "identity mismatch"},
		{"digest", func(p *leasecontract.ClaimContextPacket) { p.Issue.BodySHA256 = "other" }, "body digest mismatch"},
		{"body", func(p *leasecontract.ClaimContextPacket) { p.Issue.Body = "modified" }, "does not hash"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			changed := packet
			tc.change(&changed)
			if err := ValidateClaimPacketIdentity(record, changed, true, true, digest); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("drift accepted: %v", err)
			}
		})
	}
	if err := ValidateClaimPacketIdentity(record, packet, false, true, digest); err == nil {
		t.Fatal("source mismatch accepted")
	}
	if err := ValidateClaimPacketIdentity(record, packet, true, false, digest); err == nil {
		t.Fatal("worktree mismatch accepted")
	}
	if !RequiresSealedClaimContext(record, 2) || RequiresSealedClaimContext(record, 0) || RequiresSealedClaimContext(record, 1) {
		t.Fatal("generation applicability changed")
	}
	if err := ValidateClaimIssueSnapshot(record.IssueURL, record.IssueURL, "modified", digest); err == nil {
		t.Fatal("remote body drift accepted")
	}
	if err := ValidateClaimIssueSnapshot(record.IssueURL, "other", "abc", digest); err == nil {
		t.Fatal("remote identity drift accepted")
	}
	if err := ValidateClaimIssueSnapshot(record.IssueURL, record.IssueURL, "abc", digest); err != nil {
		t.Fatal(err)
	}
}
