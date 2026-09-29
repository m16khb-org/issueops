package issueopsowner

import (
	"encoding/json"
	"fmt"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
	"strings"
)

type ResumeReader struct{ Files port.OwnerResumeFiles }

func (s ResumeReader) Read(record issueops.IssueOpsRecord) (issueops.OwnerResumeArtifacts, error) {
	tokenPath := s.Files.TokenPath(record)
	token, err := s.readToken(record, tokenPath)
	if err != nil {
		return issueops.OwnerResumeArtifacts{}, fmt.Errorf("read current generation claim token: %w", err)
	}
	if digest([]byte(token)) != record.Execution.Lease.ClaimTokenSHA256 {
		return issueops.OwnerResumeArtifacts{}, fmt.Errorf("current generation claim token identity changed")
	}
	binding := record.Execution.Orca
	if binding == nil || !domain.CompleteOwnerArtifactIdentity(binding) {
		return issueops.OwnerResumeArtifacts{}, fmt.Errorf("sealed Orca artifact identity is not durable; run %s", domain.ReplacementPreviewCommand(record.ID, record.Execution.Lease.Generation))
	}
	paths := s.Files.Paths(record)
	packetPath, promptPath := paths.Packet, paths.Prompt
	packetData, err := s.Files.ReadArtifact(record.Execution.Workspace.Root, packetPath)
	if err != nil {
		return issueops.OwnerResumeArtifacts{}, fmt.Errorf("read sealed context packet: %w", err)
	}
	packetSHA256 := digest(packetData)
	if packetSHA256 != binding.ContextPacketSHA256 {
		return issueops.OwnerResumeArtifacts{}, fmt.Errorf("sealed context packet identity changed")
	}
	var packet issueops.OwnerContextPacket
	if err := json.Unmarshal(packetData, &packet); err != nil {
		return issueops.OwnerResumeArtifacts{}, fmt.Errorf("parse sealed context packet: %w", err)
	}
	issueBodySHA256 := strings.TrimSpace(packet.Issue.BodySHA256)
	if issueBodySHA256 != binding.IssueBodySHA256 {
		return issueops.OwnerResumeArtifacts{}, fmt.Errorf("sealed issue body identity changed")
	}
	if err := s.validatePacket(record, binding.IssueBodySHA256, binding.ContextPacketSHA256); err != nil {
		return issueops.OwnerResumeArtifacts{}, err
	}
	if err := domain.ValidateOwnerResumeProfile(packet, binding); err != nil {
		return issueops.OwnerResumeArtifacts{}, err
	}
	promptData, err := s.Files.ReadArtifact(record.Execution.Workspace.Root, promptPath)
	if err != nil {
		return issueops.OwnerResumeArtifacts{}, fmt.Errorf("read sealed owner prompt: %w", err)
	}
	promptSHA256 := digest(promptData)
	if promptSHA256 != binding.OwnerPromptSHA256 {
		return issueops.OwnerResumeArtifacts{}, fmt.Errorf("sealed owner prompt identity changed")
	}
	return issueops.OwnerResumeArtifacts{
		ClaimTokenPath: tokenPath, IssueBodySHA256: issueBodySHA256,
		ContextPacketPath: packetPath, ContextPacketSHA256: packetSHA256,
		OwnerPromptPath: promptPath, OwnerPromptSHA256: promptSHA256,
	}, nil
}

func (s ResumeReader) readToken(record issueops.IssueOpsRecord, path string) (string, error) {
	expected := s.Files.TokenPath(record)
	if !s.Files.SamePath(path, expected) {
		return "", fmt.Errorf("claim_token_file must be the deterministic current-generation path")
	}
	data, err := s.Files.ReadArtifact(record.Execution.Workspace.Root, expected)
	if err != nil {
		return "", err
	}
	return domain.ParseOwnerResumeToken(data)
}

func (s ResumeReader) validatePacket(record issueops.IssueOpsRecord, issueDigest, packetDigest string) error {
	if err := domain.ValidateOwnerResumeGeneration(record); err != nil {
		return err
	}
	packetPath := s.Files.Paths(record).Packet
	data, err := s.Files.ReadArtifact(record.Execution.Workspace.Root, packetPath)
	if err != nil {
		return fmt.Errorf("read sealed context packet: %w", err)
	}
	if observed := digest(data); observed != packetDigest {
		return fmt.Errorf("sealed context packet digest mismatch: expected=%s observed=%s path=%s", packetDigest, observed, packetPath)
	}
	var packet issueops.OwnerContextPacket
	if err := json.Unmarshal(data, &packet); err != nil {
		return fmt.Errorf("parse sealed context packet: %w", err)
	}
	sourceMatches, rootMatches := false, false
	if domain.OwnerResumePacketPrefixMatches(record, packet) {
		sourceMatches = s.Files.SamePath(packet.SourceRoot, record.Execution.Workspace.SourceRoot)
		if sourceMatches {
			rootMatches = s.Files.SamePath(packet.WorktreeRoot, record.Execution.Workspace.Root)
		}
	}
	if err := domain.ValidateOwnerResumePacket(record, packet, issueDigest, sourceMatches, rootMatches); err != nil {
		return err
	}
	planDigest, ok := packet.ArtifactManifest["plan"]
	if !ok || !domain.ValidOwnerDigest(planDigest) {
		return domain.OwnerResumePlanRequired(record)
	}
	sealedPlanPath := s.Files.ArtifactPath(record, "plan")
	sealedPlan, err := s.Files.ReadArtifact(record.Execution.Workspace.Root, sealedPlanPath)
	if err != nil || digest(sealedPlan) != planDigest {
		return domain.OwnerResumePlanRequired(record)
	}
	durablePlan, err := s.Files.ReadLinkedPlan(record)
	if err != nil || durablePlan.Digest != planDigest {
		return domain.OwnerResumePlanRequired(record)
	}
	for name, expectedDigest := range packet.ArtifactManifest {
		if name == "plan" {
			continue
		}
		path := s.Files.ArtifactPath(record, name)
		artifact, err := s.Files.ReadArtifact(record.Execution.Workspace.Root, path)
		if err != nil {
			return fmt.Errorf("read sealed artifact %s: %w", name, err)
		}
		if digest(artifact) != expectedDigest {
			return fmt.Errorf("sealed artifact %s digest mismatch", name)
		}
	}
	return nil
}
