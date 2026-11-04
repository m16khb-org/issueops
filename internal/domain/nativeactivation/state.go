// Package nativeactivation holds the pure domain rules of the native activation transition: validating requests, digests and timestamps, matching stored pending and receipt records against observed facts, and ordering the first-party MCP and hook readbacks. It performs no I/O.
package nativeactivation

import (
	"slices"

	activationcontract "issueops/internal/contract/nativeactivation"
)

// RecordSchemaVersion is the schema version a pending or receipt record must carry to match.
const RecordSchemaVersion = 1

// BinaryIdentity identifies an executable by path, SHA-256, file mode, size, device and inode.
type BinaryIdentity struct {
	Executable string
	SHA256     string
	Mode       uint32
	Size       int64
	Device     uint64
	Inode      uint64
}

// TransitionFacts are the currently observed values a stored record is compared against: roots, target binary, transition ID, the active binary, the catalog digest and the readback evidence.
type TransitionFacts struct {
	StateRoot     string
	IssueOpsRoot  string
	TargetBinary  string
	TransitionID  string
	Active        BinaryIdentity
	CatalogSHA256 string
	Evidence      []activationcontract.Evidence
}

// PendingSnapshot is the record written when a transition starts, naming the candidate binary and the start time.
type PendingSnapshot struct {
	SchemaVersion int
	StateRoot     string
	IssueOpsRoot  string
	TargetBinary  string
	Candidate     BinaryIdentity
	TransitionID  string
	StartedAt     string
}

// ReceiptSnapshot is the record sealed when a transition completes, with the activated binary, catalog digest, readback evidence and seal time.
type ReceiptSnapshot struct {
	SchemaVersion int
	StateRoot     string
	IssueOpsRoot  string
	TargetBinary  string
	Binary        BinaryIdentity
	CatalogSHA256 string
	Evidence      []activationcontract.Evidence
	TransitionID  string
	SealedAt      string
}

// PendingMatches reports whether a pending record belongs to the observed facts: same schema version, roots, target binary and transition ID, a candidate with the same content identity as the active binary, and a start time. The executable path is not compared.
func PendingMatches(record PendingSnapshot, facts TransitionFacts) bool {
	return record.SchemaVersion == RecordSchemaVersion && record.StateRoot == facts.StateRoot &&
		record.IssueOpsRoot == facts.IssueOpsRoot && record.TargetBinary == facts.TargetBinary &&
		record.TransitionID == facts.TransitionID && sameBinaryContent(record.Candidate, facts.Active) && record.StartedAt != ""
}

// ReceiptMatches reports whether a sealed receipt equals the observed facts: same schema version, roots, target binary, transition ID, catalog digest and evidence, the exact active binary identity, and a seal time.
func ReceiptMatches(record ReceiptSnapshot, facts TransitionFacts) bool {
	return record.SchemaVersion == RecordSchemaVersion && record.StateRoot == facts.StateRoot &&
		record.IssueOpsRoot == facts.IssueOpsRoot && record.TargetBinary == facts.TargetBinary &&
		record.Binary == facts.Active && record.CatalogSHA256 == facts.CatalogSHA256 &&
		record.TransitionID == facts.TransitionID && slices.Equal(record.Evidence, facts.Evidence) && record.SealedAt != ""
}

func sameBinaryContent(left, right BinaryIdentity) bool {
	return left.SHA256 == right.SHA256 && left.Mode == right.Mode && left.Size == right.Size &&
		left.Device == right.Device && left.Inode == right.Inode
}
