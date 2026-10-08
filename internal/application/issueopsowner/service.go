package issueopsowner

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	executionissue "issueops/internal/contract/executionissue"
	"issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
	"strings"
)

type Service struct {
	Files      port.OwnerContextFiles
	ReadIssue  executionissue.ExecutionIssueSnapshotReadFunc
	Template   string
	ReadRecord func(string) (issueops.IssueOpsRecord, error)
	// ResolveModel resolves the reviewer, research, and reader-check roles.
	ResolveModel ResolveModelFunc
}

func (s Service) ReadSnapshot(ctx context.Context, record issueops.IssueOpsRecord) (issueops.OwnerSnapshot, error) {
	if s.ReadIssue == nil {
		return issueops.OwnerSnapshot{}, fmt.Errorf("remote issue snapshot reader is unavailable")
	}
	provider := domain.OwnerIssueProvider(record)
	if provider == "" || strings.TrimSpace(record.IssueURL) == "" {
		return issueops.OwnerSnapshot{}, fmt.Errorf("linked GitHub or GitLab issue is required before owner dispatch")
	}
	snapshot, err := s.ReadIssue(ctx, provider, executionissue.ExecutionIssueSnapshotRequest{Repo: record.Repo, URL: record.IssueURL})
	if err != nil {
		return issueops.OwnerSnapshot{}, fmt.Errorf("read remote issue snapshot: %w", err)
	}
	acceptance, verification, err := domain.ValidateOwnerSnapshot(record.IssueURL, snapshot.URL, snapshot.Body, leasecontract.OwnerArtifactMaxBytes/2)
	if err != nil {
		return issueops.OwnerSnapshot{}, err
	}
	return issueops.OwnerSnapshot{
		Issue:                issueops.OwnerIssue{URL: strings.TrimSpace(snapshot.URL), Body: snapshot.Body, BodySHA256: digest([]byte(snapshot.Body))},
		RequiredDocs:         s.Files.RegularFiles(record.Repo, domain.OwnerRequiredDocCandidates()),
		RequiredSkills:       []string{"issueops", "verified-execution", "atomic-commit-push"},
		AcceptanceIDs:        acceptance,
		VerificationCommands: verification,
	}, nil
}

func (s Service) Build(record issueops.IssueOpsRecord, req issueops.ExecutionPrepareRequest, snapshot issueops.OwnerSnapshot, artifactManifest map[string]string) (issueops.OwnerArtifacts, error) {
	if record.Execution == nil || record.Execution.Lease.Generation == 0 {
		return issueops.OwnerArtifacts{}, fmt.Errorf("execution identity is unavailable for owner packet")
	}
	paths := s.Files.Paths(record)
	packetPath, promptPath := paths.Packet, paths.Prompt
	policy, err := PolicyContext(record, req, s.ResolveModel)
	if err != nil {
		return issueops.OwnerArtifacts{}, err
	}
	packet := domain.OwnerPacket(record, req, snapshot, artifactManifest, paths, policy)
	packetBytes, err := json.MarshalIndent(packet, "", "  ")
	if err != nil {
		return issueops.OwnerArtifacts{}, err
	}
	packetBytes = append(packetBytes, '\n')
	if len(packetBytes) > leasecontract.OwnerArtifactMaxBytes {
		return issueops.OwnerArtifacts{}, fmt.Errorf("owner context packet exceeds %d bytes", leasecontract.OwnerArtifactMaxBytes)
	}
	packetDigest := digest(packetBytes)
	prompt, err := domain.RenderOwnerPrompt(packet, packetPath, packetDigest, s.Template, leasecontract.OwnerArtifactMaxBytes)
	if err != nil {
		return issueops.OwnerArtifacts{}, err
	}
	if err := ValidateOwnerCatalog(packet.Commands); err != nil {
		return issueops.OwnerArtifacts{}, err
	}
	if err := s.Files.Write(record.Execution.Workspace.Root, packetPath, packetBytes); err != nil {
		return issueops.OwnerArtifacts{}, err
	}
	if err := s.Files.Write(record.Execution.Workspace.Root, promptPath, []byte(prompt)); err != nil {
		return issueops.OwnerArtifacts{}, err
	}
	return issueops.OwnerArtifacts{
		PacketPath: packetPath, PacketSHA256: packetDigest, PromptPath: promptPath,
		PromptSHA256: digest([]byte(prompt)), Prompt: prompt,
	}, nil
}

func digest(value []byte) string { return fmt.Sprintf("%x", sha256.Sum256(value)) }
