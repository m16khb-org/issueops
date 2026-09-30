package issueopslease

import (
	"context"
	"fmt"

	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type ClaimContextReader interface {
	Load(string) (leasecontract.Record, error)
	ReadPacket(leasecontract.Record) ([]byte, string, error)
	DecodePacket([]byte) (leasecontract.ClaimContextPacket, error)
	ReadArtifact(leasecontract.Record, string) ([]byte, error)
}
type IssueSnapshot struct{ URL, Body string }
type IssueSnapshotReader func(context.Context, string, string) (IssueSnapshot, error)

type SealedClaimContext struct {
	reader    ClaimContextReader
	readIssue IssueSnapshotReader
	paths     CanonicalPathMatcher
}

func NewSealedClaimContext(reader ClaimContextReader, readIssue IssueSnapshotReader, paths CanonicalPathMatcher) *SealedClaimContext {
	return &SealedClaimContext{reader: reader, readIssue: readIssue, paths: paths}
}
func (p *SealedClaimContext) Preflight(ctx context.Context, request ClaimPreflightRequest) (RecordValidator, error) {
	record, err := p.reader.Load(request.ID)
	if err != nil {
		return nil, err
	}
	if !leasedomain.RequiresSealedClaimContext(record, request.Generation) {
		return func(Record) error { return nil }, nil
	}
	issueDigest, packetDigest, err := leasedomain.ClaimContextDigests(request.IssueBodySHA256, request.ContextPacketSHA256)
	if err != nil {
		return nil, err
	}
	if err := p.validatePacket(record, issueDigest, packetDigest); err != nil {
		return nil, err
	}
	if p.readIssue == nil {
		return nil, fmt.Errorf("remote issue snapshot reader is unavailable for the Orca claim")
	}
	snapshot, err := p.readIssue(ctx, record.Repo, record.IssueURL)
	if err != nil {
		return nil, fmt.Errorf("read remote issue before claim: %w", err)
	}
	if err := leasedomain.ValidateClaimIssueSnapshot(record.IssueURL, snapshot.URL, snapshot.Body, issueDigest); err != nil {
		return nil, err
	}
	// The claim transaction invokes this after locking, so disk drift since preflight is refused.
	return func(current Record) error { return p.validatePacket(current.Stable, issueDigest, packetDigest) }, nil
}
func (p *SealedClaimContext) validatePacket(record leasecontract.Record, issueDigest, packetDigest string) error {
	if err := leasedomain.ValidateSealedClaimGeneration(record); err != nil {
		return err
	}
	data, path, err := p.reader.ReadPacket(record)
	if err != nil {
		return fmt.Errorf("read sealed context packet: %w", err)
	}
	if err := leasedomain.ValidateClaimPacketDigest(data, packetDigest, path); err != nil {
		return err
	}
	packet, err := p.reader.DecodePacket(data)
	if err != nil {
		return fmt.Errorf("parse sealed context packet: %w", err)
	}
	sourceMatches := p.paths.Matches(packet.SourceRoot, record.Execution.Workspace.SourceRoot)
	worktreeMatches := p.paths.Matches(packet.WorktreeRoot, record.Execution.Workspace.Root)
	if err := leasedomain.ValidateClaimPacketIdentity(record, packet, sourceMatches, worktreeMatches, issueDigest); err != nil {
		return err
	}
	for name, digest := range packet.ArtifactManifest {
		artifact, err := p.reader.ReadArtifact(record, name)
		if err != nil {
			return fmt.Errorf("read sealed artifact %s: %w", name, err)
		}
		if err := leasedomain.ValidateClaimArtifact(name, artifact, digest); err != nil {
			return err
		}
	}
	return nil
}
