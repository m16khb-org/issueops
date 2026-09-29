package issueopsowner

import (
	cycleapp "issueops/internal/application/issueopscycle"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"strings"
)

func (s Service) RequirePlan(record issueops.IssueOpsRecord) (issueops.OwnerPlanIdentity, error) {
	staged, err := s.Files.ReadStaged(record.ID)
	if err != nil {
		return issueops.OwnerPlanIdentity{}, err
	}
	plan, ok := staged["plan"]
	if !ok || strings.TrimSpace(plan) == "" {
		return issueops.OwnerPlanIdentity{}, s.planRequiredError(record, true)
	}
	identity := issueops.OwnerPlanIdentity{Digest: digest([]byte(plan))}
	if !cycleapp.DevilsAdvocateStagedPlanBound(record, identity.Digest) {
		return issueops.OwnerPlanIdentity{}, &domain.OwnerPlanReviewStaleError{NextCommand: "issueops devils-advocate review --id " + quoteArg(record.ID) +
			" --reviewer-context subagent --verdict <VERDICT> --finding <TEXT> --json"}
	}
	if strings.TrimSpace(record.PlanPath) == "" {
		return identity, nil
	}
	linked, err := s.Files.ReadLinkedPlan(record)
	if err != nil || linked.Digest != identity.Digest {
		return issueops.OwnerPlanIdentity{}, s.planRequiredError(record, false)
	}
	return linked, nil
}

func (s Service) planRequiredError(record issueops.IssueOpsRecord, allowStageCommand bool) error {
	typed := &domain.OwnerPlanRequiredError{}
	if !allowStageCommand {
		return typed
	}
	if identity, err := s.Files.ReadLinkedPlan(record); err == nil {
		typed.NextCommand = "issueops artifact stage --id " + quoteArg(record.ID) +
			" --name plan --file " + quoteArg(identity.Path) + " --json"
	}
	return typed
}

func (s Service) MaterializePlan(record issueops.IssueOpsRecord) (issueops.OwnerPlanIdentity, map[string]string, error) {
	preflight, err := s.RequirePlan(record)
	if err != nil {
		return issueops.OwnerPlanIdentity{}, nil, err
	}
	manifest, err := s.Files.Materialize(record)
	if err != nil {
		return issueops.OwnerPlanIdentity{}, nil, err
	}
	planDigest, ok := manifest["plan"]
	if !ok || !strings.EqualFold(planDigest, preflight.Digest) {
		return issueops.OwnerPlanIdentity{}, nil, s.planRequiredError(record, false)
	}
	if preflight.Path != "" {
		return preflight, manifest, nil
	}
	prepared := record
	prepared.WorktreePath = record.Execution.Workspace.Root
	prepared.PlanPath = s.Files.Paths(record).Plan
	identity, err := s.Files.ReadLinkedPlan(prepared)
	if err != nil || !strings.EqualFold(identity.Digest, planDigest) {
		return issueops.OwnerPlanIdentity{}, nil, s.planRequiredError(prepared, false)
	}
	return identity, manifest, nil
}

func quoteArg(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'" }
