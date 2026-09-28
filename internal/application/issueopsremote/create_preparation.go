package issueopsremote

import (
	"context"
	"strings"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
	reviewcontract "issueops/internal/contract/issueopsreview"
	remote "issueops/internal/domain/issueopsremote"
	reviewdomain "issueops/internal/domain/issueopsreview"
	"issueops/internal/domain/policy"
)

type PreparationObserver interface {
	NormalizeActor(context.Context, contract.Actor) (contract.Actor, error)
	Read(context.Context, string) (model.IssueOpsRecord, error)
	Authorize(context.Context, model.IssueOpsRecord, contract.CreateCommand) error
	Fingerprint(context.Context, model.IssueOpsRecord) string
	Head(context.Context, model.IssueOpsRecord) string
}

type CreatePreparation struct{ observer PreparationObserver }

func NewCreatePreparation(observer PreparationObserver) *CreatePreparation {
	return &CreatePreparation{observer: observer}
}

func (s *CreatePreparation) Prepare(ctx context.Context, command contract.CreateCommand) (contract.PreparedCreate, error) {
	if command.Confirm {
		actor, err := s.observer.NormalizeActor(ctx, command.Actor)
		if err != nil {
			return contract.PreparedCreate{}, err
		}
		command.Actor = actor
	}
	record, err := s.observer.Read(ctx, command.ID)
	if err != nil {
		return contract.PreparedCreate{}, err
	}
	facts := remote.CreateAuthority{
		Provider: command.Provider, PhasePR: record.Phase == model.IssueOpsPhasePR,
		Artifact: record.RemoteArtifact != nil, Confirm: command.Confirm,
		Execution: record.Execution != nil, ExpectedGeneration: command.ExpectedGeneration,
	}
	if record.Execution != nil {
		facts.Generation = record.Execution.Lease.Generation
	}
	provider, kind, err := remote.ValidateCreateAuthority(facts)
	if err != nil {
		return contract.PreparedCreate{}, err
	}
	if command.Confirm {
		if err := s.observer.Authorize(ctx, record, command); err != nil {
			return contract.PreparedCreate{}, err
		}
		if err := remote.ValidateCreatePending(record.Execution.Pending != nil); err != nil {
			return contract.PreparedCreate{}, err
		}
		fingerprint := s.observer.Fingerprint(ctx, record)
		review := reviewcontract.ReviewGateEvidence{}
		if record.ImplementationReview != nil {
			review = reviewcontract.ReviewGateEvidence{Present: true, Verdict: record.ImplementationReview.Verdict, ReviewedFingerprint: record.ImplementationReview.ReviewedFingerprint}
		}
		missing := reviewdomain.ImplementationReviewMissing(true, review, fingerprint)
		if err := remote.ValidateCreateReview(record.ID, missing, fingerprint, review.ReviewedFingerprint != ""); err != nil {
			return contract.PreparedCreate{}, err
		}
	}
	authority := remote.CreateBranchAuthority{Prepared: record.BranchPrepare != nil, IssueURL: record.IssueURL, WorkspaceBranch: strings.TrimSpace(record.Branch)}
	if record.BranchPrepare != nil {
		authority.Provider = record.BranchPrepare.Provider
		authority.BaseBranch = record.BranchPrepare.BaseBranch
		authority.CodeProjectKey = record.BranchPrepare.CodeProjectKey
	}
	if record.Execution != nil {
		authority.WorkspaceBranch = record.Execution.Workspace.Branch
	}
	title, body := strings.TrimSpace(command.Title), strings.TrimSpace(command.Body)
	prepared, err := remote.PrepareCreateRequest(authority, remote.CreateRequest{
		Provider: provider, Head: command.Head, Base: command.Base, Title: title, Body: body, Labels: command.Labels, Assignees: command.Assignees,
	}, policy.RedactFreeform(title) != title || policy.RedactFreeform(body) != body)
	if err != nil {
		return contract.PreparedCreate{}, err
	}
	root, head := record.Repo, ""
	if command.Confirm {
		head = s.observer.Head(ctx, record)
		if err := remote.ValidateCreateHead(head); err != nil {
			return contract.PreparedCreate{}, err
		}
		root = record.Execution.Workspace.Root
	}
	return contract.PreparedCreate{
		Command: command.Clone(),
		Request: contract.ProviderCreateRequest{
			Repo: root, ProjectKey: prepared.ProjectKey, Title: prepared.Title, Body: prepared.Body,
			HeadBranch: prepared.Head, BaseBranch: prepared.Base, Labels: prepared.Labels, Assignees: prepared.Assignees,
			Draft: true, ExpectedHeadSHA: head, Confirm: command.Confirm,
			Host: command.Actor.Host, SessionID: command.Actor.SessionID, AgentID: command.Actor.AgentID, CWD: command.CWD,
		},
		Eligibility: contract.CreateEligibility{
			Provider: provider, Kind: kind, Confirm: command.Confirm, PhasePR: facts.PhasePR, NoArtifact: !facts.Artifact,
			ExecutionActive: record.Execution != nil && record.Execution.Lease.Status == model.LeaseStatusActive,
			NoPending:       record.Execution == nil || record.Execution.Pending == nil,
			BranchAuthority: true, CanonicalLabelsAssignees: true,
		},
	}, nil
}
