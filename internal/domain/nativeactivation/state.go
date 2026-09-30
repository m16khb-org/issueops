package nativeactivation

import (
	"slices"

	activationcontract "issueops/internal/contract/nativeactivation"
)

const RecordSchemaVersion = 1

type BinaryIdentity struct {
	Executable string
	SHA256     string
	Mode       uint32
	Size       int64
	Device     uint64
	Inode      uint64
}

type TransitionFacts struct {
	StateRoot     string
	IssueOpsRoot  string
	TargetBinary  string
	TransitionID  string
	Active        BinaryIdentity
	CatalogSHA256 string
	Evidence      []activationcontract.Evidence
}

type PendingSnapshot struct {
	SchemaVersion int
	StateRoot     string
	IssueOpsRoot  string
	TargetBinary  string
	Candidate     BinaryIdentity
	TransitionID  string
	StartedAt     string
}

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

func PendingMatches(record PendingSnapshot, facts TransitionFacts) bool {
	return record.SchemaVersion == RecordSchemaVersion && record.StateRoot == facts.StateRoot &&
		record.IssueOpsRoot == facts.IssueOpsRoot && record.TargetBinary == facts.TargetBinary &&
		record.TransitionID == facts.TransitionID && sameBinaryContent(record.Candidate, facts.Active) && record.StartedAt != ""
}

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
