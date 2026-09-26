package model

import contract "issueops/internal/contract/selfaugment"

const (
	SelfVerificationSummaryKind     = "self_verification_summary"
	SelfVerificationKoreanName      = contract.SelfVerificationKoreanName
	SelfAugmentationKoreanName      = "자가 증강 루프"
	DefaultLoopTargetScoreExclusive = 95.0
)

type SelfAugmentStateCheckpoint = contract.SelfAugmentStateCheckpoint

type SelfAugmentPromoteResult struct {
	OK                  bool               `json:"ok"`
	StateDir            string             `json:"state_dir"`
	FromKey             string             `json:"from_key"`
	BaselineKey         string             `json:"baseline_key"`
	Confirm             bool               `json:"confirm"`
	DryRun              bool               `json:"dry_run"`
	Promoted            bool               `json:"promoted"`
	SourcePassed        bool               `json:"source_passed"`
	Path                string             `json:"path,omitempty"`
	Bytes               int                `json:"bytes,omitempty"`
	SnapshotGeneratedAt string             `json:"snapshot_generated_at"`
	Summary             SelfAugmentSummary `json:"summary"`
}

type SelfAugmentIteration = contract.SelfAugmentIteration
