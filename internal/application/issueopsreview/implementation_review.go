package issueopsreview

import (
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	reviewcontract "issueops/internal/contract/issueopsreview"
	issueopsdomain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
	"issueops/internal/domain/policy"
	reviewport "issueops/internal/port/issueopsreview"
)

// RecordImplementationReview observes the change set before taking the record lock,
// then rechecks its identity against the locked record before one durable write.
func RecordImplementationReview(store reviewport.EvidenceReviewStore, stateRoot, id string, req model.IssueOpsImplementationReviewRequest) (model.IssueOpsRecord, error) {
	verdict := strings.ToLower(strings.TrimSpace(req.Verdict))
	findings := cleanReviewValues(req.Findings)
	evidence := cleanReviewValues(req.Evidence)
	if err := reviewdomain.ValidateImplementationReviewRecord(verdict, len(findings), len(evidence)); err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	observed, err := store.Read(stateRoot, id)
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	fingerprint := store.Fingerprint(observed)
	var result model.IssueOpsRecord
	err = store.WithLock(stateRoot, id, func() error {
		current, err := store.Read(stateRoot, id)
		if err != nil {
			return err
		}
		if err := store.ValidateMutation(current); err != nil {
			return err
		}
		if err := issueopsdomain.ValidateEvidenceRecordingPhase(current.Phase, "implementation review"); err != nil {
			return err
		}
		if err := validateCurrentChangeObservation(observed, current); err != nil {
			return err
		}
		now := store.Now()
		current.ImplementationReview = &model.IssueOpsImplementationReview{
			Verdict: verdict, Findings: findings, Evidence: evidence,
			ReviewedFingerprint: fingerprint,
			ReviewerHost:        strings.ToLower(strings.TrimSpace(req.ReviewerHost)),
			ReviewerModel:       strings.TrimSpace(req.ReviewerModel),
			ReviewerEffort:      strings.TrimSpace(req.ReviewerEffort),
			RecordedAt:          now,
		}
		current.UpdatedAt = now
		result, err = store.Write(stateRoot, current)
		return err
	})
	if err != nil {
		return model.IssueOpsRecord{OK: false}, err
	}
	return result, nil
}

func cleanReviewValues(values []string) []string {
	out := []string{}
	for _, value := range values {
		value = policy.RedactFreeform(strings.TrimSpace(value))
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}

func changeObservationIdentity(record model.IssueOpsRecord) reviewcontract.ChangeObservationIdentity {
	root := strings.TrimSpace(record.WorktreePath)
	if root == "" {
		root = strings.TrimSpace(record.Repo)
	}
	identity := reviewcontract.ChangeObservationIdentity{Root: root}
	if record.BranchPrepare != nil {
		identity.HasPreparedBase = true
		identity.BaseSHA = record.BranchPrepare.BaseSHA
		identity.BaseBranch = record.BranchPrepare.BaseBranch
	}
	return identity
}

func validateCurrentChangeObservation(observed, current model.IssueOpsRecord) error {
	if !reviewdomain.SameChangeObservationIdentity(changeObservationIdentity(observed), changeObservationIdentity(current)) {
		return fmt.Errorf("IssueOps record %s changed its worktree or base while the change set was observed; retry the command", current.ID)
	}
	return nil
}
