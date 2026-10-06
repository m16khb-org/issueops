package issueopslease

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	leasecontract "issueops/internal/contract/issueopslease"
	ownerdomain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

// ClaimContextReader observes persisted records and private sealed files.
type ClaimContextReader struct{ store port.TransactionalRecordStore }

func NewClaimContextReader(store port.TransactionalRecordStore) *ClaimContextReader {
	return &ClaimContextReader{store: store}
}
func (p *ClaimContextReader) Load(id string) (leasecontract.Record, error) {
	if p == nil || p.store == nil {
		return leasecontract.Record{}, fmt.Errorf("claim context store is required")
	}
	data, ok, err := p.store.Get(recordBucket, id)
	if err != nil {
		return leasecontract.Record{}, leasecontract.Fail(leasecontract.FailurePersistence, err)
	}
	if !ok {
		return leasecontract.Record{}, leasecontract.Fail(leasecontract.FailurePersistence, fmt.Errorf("issueops record %s not found", id))
	}
	record, err := decodeMutableLeaseRecord(id, data)
	if err != nil {
		return leasecontract.Record{}, err
	}
	return record, nil
}
func (*ClaimContextReader) ReadPacket(record leasecontract.Record) ([]byte, string, error) {
	path := claimContextPacketPath(record)
	data, err := readClaimOwnerArtifact(record.Execution.Workspace.Root, path)
	return data, path, err
}
func (*ClaimContextReader) DecodePacket(data []byte) (leasecontract.ClaimContextPacket, error) {
	var packet leasecontract.ClaimContextPacket
	err := json.Unmarshal(data, &packet)
	return packet, err
}
func (*ClaimContextReader) ReadArtifact(record leasecontract.Record, name string) ([]byte, error) {
	dir := strings.TrimSpace(record.Execution.Workspace.ArtifactDir)
	if dir == "" {
		return nil, ownerdomain.ErrSealedArtifactDirMissing
	}
	root := record.Execution.Workspace.Root
	return readClaimOwnerArtifact(root, filepath.Join(root, filepath.FromSlash(dir), name+".md"))
}

func claimContextPacketPath(record leasecontract.Record) string {
	key := claimTokenSHA256(record.ID)[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".issueops", "state", "issueops-v1", key, fmt.Sprintf("generation-%d", record.Execution.Lease.Generation), "context.json")
}

func readClaimOwnerArtifact(root, path string) ([]byte, error) {
	root, path = filepath.Clean(root), filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("owner artifact must be inside the canonical worktree")
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	current := root
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("owner artifact path contains a missing entry or symlink")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("owner artifact ancestor is not a directory")
		}
		if index == len(parts)-1 && (!info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > leasecontract.OwnerArtifactMaxBytes) {
			return nil, fmt.Errorf("owner artifact must be a private bounded regular file")
		}
	}
	return os.ReadFile(path)
}
