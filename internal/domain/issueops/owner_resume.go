package issueops

import (
	"crypto/sha256"
	"fmt"
	"issueops/internal/contract/issueops"
	"strconv"
	"strings"
)

func CompleteOwnerArtifactIdentity(binding *issueops.OrcaBinding) bool {
	return binding != nil && binding.ArtifactIdentityVersion == issueops.OrcaArtifactIdentityVersion &&
		ValidOwnerDigest(binding.IssueBodySHA256) &&
		ValidOwnerDigest(binding.ContextPacketSHA256) && ValidOwnerDigest(binding.OwnerPromptSHA256)
}

func OwnerResumeNextCommand(id string, generation uint64, _ string, issueBodySHA256, contextPacketSHA256 string) string {
	return "issueops execution claim --id " + quoteReplacementArg(id) +
		" --generation " + strconv.FormatUint(generation, 10) +
		" --claim-current-token" +
		" --issue-body-sha256 " + issueBodySHA256 +
		" --context-packet-sha256 " + contextPacketSHA256
}

func ReplacementPreviewCommand(id string, generation uint64) string {
	return "issueops execution replace --id " + quoteReplacementArg(id) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --preview"
}
func OwnerResumePlanRequired(record issueops.IssueOpsRecord) error {
	typed := &OwnerPlanRequiredError{}
	if record.Execution != nil && record.Execution.Lease.Generation > 0 {
		typed.NextCommand = ReplacementPreviewCommand(record.ID, record.Execution.Lease.Generation)
	}
	return typed
}
func ParseOwnerResumeToken(data []byte) (string, error) {
	if len(data) > 256 {
		return "", fmt.Errorf("claim token file is oversized")
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("claim token file is empty")
	}
	return token, nil
}
func ValidateOwnerResumePacket(record issueops.IssueOpsRecord, packet issueops.OwnerContextPacket, issueDigest string, sourceMatches, rootMatches bool) error {
	generation := record.Execution.Lease.Generation
	if !OwnerResumePacketPrefixMatches(record, packet) ||
		!sourceMatches || !rootMatches || packet.Branch != record.Execution.Workspace.Branch || packet.BaseHead != record.Execution.Workspace.BaseHead ||
		packet.LeaseGeneration != generation || packet.Issue.URL != record.IssueURL {
		return fmt.Errorf("sealed context packet execution identity mismatch: packet_generation=%d expected_generation=%d", packet.LeaseGeneration, generation)
	}
	if packet.Issue.BodySHA256 != issueDigest {
		return fmt.Errorf("sealed context packet issue body digest mismatch: expected=%s observed=%s", issueDigest, packet.Issue.BodySHA256)
	}
	if observed := fmt.Sprintf("%x", sha256.Sum256([]byte(packet.Issue.Body))); observed != issueDigest {
		return fmt.Errorf("sealed context packet issue body does not hash to its sealed digest: expected=%s observed=%s", issueDigest, observed)
	}
	return nil
}

func ReplacementFinalizePreviewCommand(id string, generation uint64) string {
	return "issueops execution replace --id " + quoteReplacementArg(id) + " --expected-generation " + strconv.FormatUint(generation, 10) + " --finalize-preview"
}
func OwnerReseedNextCommand(id string, generation uint64, mode, claimTokenPath string) string {
	switch issueops.ExecutionMode(mode) {
	case issueops.ExecutionModeOrca:
		return ReplacementResumeCommand(id, generation)
	case issueops.ExecutionModeDirect:
		return ReplacementClaimCommand(id, generation, claimTokenPath)
	default:
		return ""
	}
}

func OwnerResumePacketPrefixMatches(record issueops.IssueOpsRecord, packet issueops.OwnerContextPacket) bool {
	return packet.SchemaVersion == issueops.IssueOpsSchemaVersion && packet.LifecycleID == record.ID && packet.Mode == record.Execution.Mode
}
func ValidateOwnerResumeGeneration(record issueops.IssueOpsRecord) error {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Lease.Generation == 0 {
		return fmt.Errorf("sealed owner context no longer matches an Orca execution generation")
	}
	return nil
}
func ValidateOwnerResumeProfile(packet issueops.OwnerContextPacket, binding *issueops.OrcaBinding) error {
	if packet.OwnerHost != binding.OwnerHost || packet.OwnerModel != binding.OwnerModel || packet.OwnerEffort != binding.OwnerEffort {
		return fmt.Errorf("sealed owner profile does not match the Orca binding")
	}
	return nil
}
