package selfworkflow

import (
	"issueops/cmd/issueops/selfworkflow/augmentcatalog"
	contract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
)

const SelfVerificationCandidateExportKind = contract.SelfVerificationCandidateExportKind

const (
	selfAugmentCandidateStatusOpen      = augmentcatalog.SelfAugmentCandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = augmentcatalog.SelfAugmentCandidateStatusSatisfied
)

type SelfVerificationCandidate = verifycontract.SelfVerificationCandidate
type SelfVerificationCandidateExportResult = contract.SelfVerificationCandidateExportResult
type SelfVerificationCandidateExportStateSnapshot = contract.SelfVerificationCandidateExportStateSnapshot
