package issueopslease

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	leasecontract "issueops/internal/contract/issueopslease"
)

func RequiresSealedClaimContext(record leasecontract.Record, generation uint64) bool {
	return record.Execution != nil && record.Execution.Mode == "orca" && generation != 0 && generation == record.Execution.Lease.Generation
}
func ClaimContextDigests(issue, packet string) (string, string, error) {
	issue, packet = strings.ToLower(strings.TrimSpace(issue)), strings.ToLower(strings.TrimSpace(packet))
	if !claimSHA256(issue) || !claimSHA256(packet) {
		return "", "", fmt.Errorf("Orca claim requires sealed issue and context packet digests")
	}
	return issue, packet, nil
}
func ValidateSealedClaimGeneration(record leasecontract.Record) error {
	if record.Execution == nil || record.Execution.Mode != "orca" || record.Execution.Lease.Generation == 0 {
		return fmt.Errorf("sealed owner context no longer matches an Orca execution generation")
	}
	return nil
}
func ValidateClaimPacketDigest(data []byte, expected, path string) error {
	if observed := ClaimContextDigest(data); observed != expected {
		return fmt.Errorf("sealed context packet digest mismatch: expected=%s observed=%s path=%s", expected, observed, path)
	}
	return nil
}
func ValidateClaimPacketIdentity(record leasecontract.Record, packet leasecontract.ClaimContextPacket, sourceMatches, worktreeMatches bool, issueDigest string) error {
	execution := record.Execution
	if packet.SchemaVersion != leasecontract.SchemaVersion || packet.LifecycleID != record.ID || packet.Mode != execution.Mode ||
		!sourceMatches || !worktreeMatches || packet.Branch != execution.Workspace.Branch || packet.BaseHead != execution.Workspace.BaseHead ||
		packet.LeaseGeneration != execution.Lease.Generation || packet.Issue.URL != record.IssueURL {
		return fmt.Errorf("sealed context packet execution identity mismatch: packet_generation=%d expected_generation=%d", packet.LeaseGeneration, execution.Lease.Generation)
	}
	if packet.Issue.BodySHA256 != issueDigest {
		return fmt.Errorf("sealed context packet issue body digest mismatch: expected=%s observed=%s", issueDigest, packet.Issue.BodySHA256)
	}
	if observed := ClaimContextDigest([]byte(packet.Issue.Body)); observed != issueDigest {
		return fmt.Errorf("sealed context packet issue body does not hash to its sealed digest: expected=%s observed=%s", issueDigest, observed)
	}
	return nil
}
func ValidateClaimArtifact(name string, data []byte, expected string) error {
	if ClaimContextDigest(data) != expected {
		return fmt.Errorf("sealed artifact %s digest mismatch", name)
	}
	return nil
}
func ValidateClaimIssueSnapshot(expectedURL, observedURL, body, issueDigest string) error {
	if strings.TrimSpace(observedURL) != strings.TrimSpace(expectedURL) {
		return fmt.Errorf("remote issue snapshot url does not match the linked issue: observed=%s expected=%s", strings.TrimSpace(observedURL), strings.TrimSpace(expectedURL))
	}
	if observed := ClaimContextDigest([]byte(body)); observed != issueDigest {
		return fmt.Errorf("remote issue body digest drifted from the sealed owner context: expected=%s observed=%s; reseal with `issueops execution replace --reseed` after confirming the revision is intended", issueDigest, observed)
	}
	return nil
}
func ClaimContextDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func claimSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
