package issueopsbranch

import (
	"context"
	"fmt"
	"strings"
	"time"

	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type Retargeter struct {
	Records       CycleRecords
	Authority     cycleapp.MutationAuthority
	TargetBranch  func(model.IssueOpsRemoteArtifactVerification) (string, error)
	OriginPresent func(string, string) (bool, error)
	Now           func() time.Time
}

func (s Retargeter) authorize(ctx context.Context, record model.IssueOpsRecord, actor *model.IssueOpsActor) error {
	required, err := domain.RetargetRequiresHolder(record)
	if err != nil || !required {
		return err
	}
	return s.Authority.Validate(ctx, record, actor)
}

// Retarget observes the remote before entering the transaction. Inside the
// transaction it rechecks authority and binds those observations to fresh state.
func (s Retargeter) Retarget(ctx context.Context, id string, req model.IssueOpsBranchRetargetRequest, actor model.IssueOpsActor) (model.IssueOpsRecord, error) {
	observed, err := s.Records.Load(id)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	if err := s.authorize(ctx, observed, &actor); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	base, reason := strings.TrimSpace(req.BaseBranch), strings.TrimSpace(req.Reason)
	observation := s.observe(observed, base)
	var result model.IssueOpsRecord
	err = s.Records.WithinLock(ctx, id, func(spanCtx context.Context) error {
		record, err := s.Records.Load(id)
		if err != nil {
			return err
		}
		if err := s.authorize(ctx, record, &actor); err != nil {
			return err
		}
		if err := domain.ValidateRetargetRequest(base, reason); err != nil {
			return err
		}
		if s.TargetBranch == nil || s.OriginPresent == nil {
			return fmt.Errorf("retarget observation is unavailable")
		}
		if err := domain.ValidateRetargetRecord(record, base); err != nil {
			return err
		}
		if err := domain.ValidateRetargetArtifact(observation.artifact, *record.RemoteArtifact); err != nil {
			return fmt.Errorf("remote artifact target observation failed: %w", err)
		}
		if observation.targetErr != nil {
			return fmt.Errorf("remote artifact target observation failed: %w", observation.targetErr)
		}
		if err := domain.ValidateRetargetTarget(base, observation.target); err != nil {
			return err
		}
		if err := domain.ValidateRetargetOrigin(record.Repo, base, observation.repo, observation.branch, observation.presentStated, observation.present, observation.presentErr); err != nil {
			return err
		}
		result, err = s.Records.Save(spanCtx, domain.ApplyBranchRetarget(record, base, reason, s.Now().UTC().Format(time.RFC3339Nano), s.Now().UTC().Format(time.RFC3339Nano)))
		return err
	})
	return result, err
}

type retargetObservation struct {
	artifact      *model.IssueOpsRemoteArtifactVerification
	target        string
	targetErr     error
	repo, branch  string
	present       bool
	presentErr    error
	presentStated bool
}

func (s Retargeter) observe(record model.IssueOpsRecord, base string) retargetObservation {
	var observation retargetObservation
	if s.TargetBranch == nil || record.RemoteArtifact == nil || base == "" {
		return observation
	}
	artifact := *record.RemoteArtifact
	observation.artifact = &artifact
	observation.target, observation.targetErr = s.TargetBranch(artifact)
	if observation.targetErr != nil || domain.ValidateRetargetTarget(base, observation.target) != nil || s.OriginPresent == nil {
		return observation
	}
	observation.repo, observation.branch = record.Repo, base
	observation.present, observation.presentErr = s.OriginPresent(record.Repo, base)
	observation.presentStated = true
	return observation
}
