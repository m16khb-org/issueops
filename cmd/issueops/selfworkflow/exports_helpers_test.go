package selfworkflow

import (
	contract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
)

const SelfVerificationCandidateExportKind = contract.SelfVerificationCandidateExportKind

const (
	selfAugmentCandidateStatusOpen      = contract.CandidateStatusOpen
	selfAugmentCandidateStatusSatisfied = contract.CandidateStatusSatisfied
)

type SelfVerificationCandidate = verifycontract.SelfVerificationCandidate
type SelfVerificationCandidateExportResult = contract.SelfVerificationCandidateExportResult
type SelfVerificationCandidateExportStateSnapshot = contract.SelfVerificationCandidateExportStateSnapshot
