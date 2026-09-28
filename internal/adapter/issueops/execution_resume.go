package issueops

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"issueops/internal/contract/issueops"
)

type executionResumeArtifacts struct {
	claimTokenPath  string
	issueBodySHA256 string
	packetPath      string
	packetSHA256    string
	promptPath      string
	promptSHA256    string
}

func readExecutionResumeArtifacts(record issueops.IssueOpsRecord) (executionResumeArtifacts, error) {
	tokenPath := claimTokenPath(record)
	token, err := readExecutionResumeClaimToken(record, tokenPath)
	if err != nil {
		return executionResumeArtifacts{}, fmt.Errorf("read current generation claim token: %w", err)
	}
	if tokenSHA256(token) != record.Execution.Lease.ClaimTokenSHA256 {
		return executionResumeArtifacts{}, fmt.Errorf("current generation claim token identity changed")
	}
	binding := record.Execution.Orca
	if binding == nil || !completeOrcaArtifactIdentity(binding) {
		return executionResumeArtifacts{}, fmt.Errorf("sealed Orca artifact identity is not durable; run %s", executionReplacementPreviewCommand(record.ID, record.Execution.Lease.Generation))
	}
	packetPath, promptPath := executionOwnerArtifactPaths(record)
	packetData, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, packetPath)
	if err != nil {
		return executionResumeArtifacts{}, fmt.Errorf("read sealed context packet: %w", err)
	}
	packetSHA256 := digestExecutionOwnerBytes(packetData)
	if packetSHA256 != binding.ContextPacketSHA256 {
		return executionResumeArtifacts{}, fmt.Errorf("sealed context packet identity changed")
	}
	var packet executionOwnerContextPacket
	if err := json.Unmarshal(packetData, &packet); err != nil {
		return executionResumeArtifacts{}, fmt.Errorf("parse sealed context packet: %w", err)
	}
	issueBodySHA256 := strings.TrimSpace(packet.Issue.BodySHA256)
	if issueBodySHA256 != binding.IssueBodySHA256 {
		return executionResumeArtifacts{}, fmt.Errorf("sealed issue body identity changed")
	}
	if err := validateExecutionResumePacket(record, binding.IssueBodySHA256, binding.ContextPacketSHA256); err != nil {
		return executionResumeArtifacts{}, err
	}
	if packet.OwnerHost != binding.OwnerHost || packet.OwnerModel != binding.OwnerModel || packet.OwnerEffort != binding.OwnerEffort {
		return executionResumeArtifacts{}, fmt.Errorf("sealed owner profile does not match the Orca binding")
	}
	promptData, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, promptPath)
	if err != nil {
		return executionResumeArtifacts{}, fmt.Errorf("read sealed owner prompt: %w", err)
	}
	promptSHA256 := digestExecutionOwnerBytes(promptData)
	if promptSHA256 != binding.OwnerPromptSHA256 {
		return executionResumeArtifacts{}, fmt.Errorf("sealed owner prompt identity changed")
	}
	return executionResumeArtifacts{
		claimTokenPath: tokenPath, issueBodySHA256: issueBodySHA256,
		packetPath: packetPath, packetSHA256: packetSHA256,
		promptPath: promptPath, promptSHA256: promptSHA256,
	}, nil
}

func completeOrcaArtifactIdentity(binding *issueops.OrcaBinding) bool {
	return binding != nil && binding.ArtifactIdentityVersion == issueops.OrcaArtifactIdentityVersion &&
		validExecutionOwnerDigest(binding.IssueBodySHA256) &&
		validExecutionOwnerDigest(binding.ContextPacketSHA256) && validExecutionOwnerDigest(binding.OwnerPromptSHA256)
}

func validExecutionOwnerDigest(value string) bool {
	return executionSHA256.MatchString(strings.TrimSpace(value))
}

// readExecutionResumeClaimToken은 owner를 다시 띄우기 전에 현재 generation의
// 비공개 token이 그대로 남아 있는지만 확인한다. Claim mutation은 application
// vertical이 소유하며, 이 함수는 파일을 소비하거나 상태를 바꾸지 않는다.
func readExecutionResumeClaimToken(record issueops.IssueOpsRecord, path string) (string, error) {
	expected := claimTokenPath(record)
	if !samePath(path, expected) {
		return "", fmt.Errorf("claim_token_file must be the deterministic current-generation path")
	}
	data, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, expected)
	if err != nil {
		return "", err
	}
	if len(data) > 256 {
		return "", fmt.Errorf("claim token file is oversized")
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", fmt.Errorf("claim token file is empty")
	}
	return token, nil
}

// validateExecutionResumePacket은 이미 claim vertical이 봉인한 packet을 owner
// 재기동 전에 다시 읽어 generation과 artifact digest가 변하지 않았는지 검증한다.
// mutation 규칙을 복제하지 않고 resume의 read-only 사전조건만 확인한다.
func validateExecutionResumePacket(record issueops.IssueOpsRecord, issueDigest, packetDigest string) error {
	if record.Execution == nil || record.Execution.Mode != issueops.ExecutionModeOrca || record.Execution.Lease.Generation == 0 {
		return fmt.Errorf("sealed owner context no longer matches an Orca execution generation")
	}
	generation := record.Execution.Lease.Generation
	packetPath, _ := executionOwnerArtifactPaths(record)
	data, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, packetPath)
	if err != nil {
		return fmt.Errorf("read sealed context packet: %w", err)
	}
	if observed := digestExecutionOwnerBytes(data); observed != packetDigest {
		return fmt.Errorf("sealed context packet digest mismatch: expected=%s observed=%s path=%s", packetDigest, observed, packetPath)
	}
	var packet executionOwnerContextPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return fmt.Errorf("parse sealed context packet: %w", err)
	}
	if packet.SchemaVersion != issueops.IssueOpsSchemaVersion || packet.LifecycleID != record.ID || packet.Mode != record.Execution.Mode ||
		!samePath(packet.SourceRoot, record.Execution.Workspace.SourceRoot) || !samePath(packet.WorktreeRoot, record.Execution.Workspace.Root) ||
		packet.Branch != record.Execution.Workspace.Branch || packet.BaseHead != record.Execution.Workspace.BaseHead ||
		packet.LeaseGeneration != generation || packet.Issue.URL != record.IssueURL {
		return fmt.Errorf("sealed context packet execution identity mismatch: packet_generation=%d expected_generation=%d", packet.LeaseGeneration, generation)
	}
	if packet.Issue.BodySHA256 != issueDigest {
		return fmt.Errorf("sealed context packet issue body digest mismatch: expected=%s observed=%s", issueDigest, packet.Issue.BodySHA256)
	}
	if observed := digestExecutionOwnerBytes([]byte(packet.Issue.Body)); observed != issueDigest {
		return fmt.Errorf("sealed context packet issue body does not hash to its sealed digest: expected=%s observed=%s", issueDigest, observed)
	}
	planDigest, ok := packet.ArtifactManifest["plan"]
	if !ok || !validExecutionOwnerDigest(planDigest) {
		return newPlanResumeArtifactRequiredError(record)
	}
	sealedPlanPath := sealedArtifactPath(record, record.Execution.Workspace.Root, "plan")
	sealedPlan, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, sealedPlanPath)
	if err != nil || digestExecutionOwnerBytes(sealedPlan) != planDigest {
		return newPlanResumeArtifactRequiredError(record)
	}
	durablePlan, err := readLinkedPlanIdentity(record)
	if err != nil || durablePlan.Digest != planDigest {
		return newPlanResumeArtifactRequiredError(record)
	}
	for name, digest := range packet.ArtifactManifest {
		if name == "plan" {
			continue
		}
		path := sealedArtifactPath(record, record.Execution.Workspace.Root, name)
		artifact, err := readExecutionOwnerArtifact(record.Execution.Workspace.Root, path)
		if err != nil {
			return fmt.Errorf("read sealed artifact %s: %w", name, err)
		}
		if digestExecutionOwnerBytes(artifact) != digest {
			return fmt.Errorf("sealed artifact %s digest mismatch", name)
		}
	}
	return nil
}

func ExecutionResumeNextCommand(id string, generation uint64, _ string, issueBodySHA256, contextPacketSHA256 string) string {
	return "issueops execution claim --id " + quoteExecutionOwnerArg(id) +
		" --generation " + strconv.FormatUint(generation, 10) +
		" --claim-current-token" +
		" --issue-body-sha256 " + issueBodySHA256 +
		" --context-packet-sha256 " + contextPacketSHA256
}

func ExecutionResumeRecoveryCommand(id string, generation uint64) string {
	return "issueops execution resume --id " + quoteExecutionOwnerArg(id) +
		" --expected-generation " + strconv.FormatUint(generation, 10) + " --confirm"
}
