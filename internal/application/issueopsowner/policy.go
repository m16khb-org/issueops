package issueopsowner

import (
	"fmt"
	"issueops/internal/contract/issueops"
	"issueops/internal/domain/agentmodel"
	"issueops/internal/domain/commandparse"
	remote "issueops/internal/domain/issueopsremote"
	"regexp"
	"strings"
)

var executionCommandValue = regexp.MustCompile(`<[A-Z][A-Z0-9_-]*>`)

func PolicyContext(record issueops.IssueOpsRecord, req issueops.ExecutionPrepareRequest) issueops.OwnerPolicyContext {
	host := strings.ToLower(strings.TrimSpace(req.OwnerHost))
	reviewer, reviewerEffort, _ := agentmodel.PlannerDefaults(host)
	research, researchEffort, _ := agentmodel.ResearchDefaults(host)
	policy := issueops.OwnerPolicyContext{ReviewerModel: reviewer, ReviewerEffort: reviewerEffort, ResearchModel: research, ResearchEffort: researchEffort}
	if record.BranchPrepare != nil {
		policy.ProjectKey = remote.ProjectKey(record.BranchPrepare.IssueURL, "github", "issue")
		policy.IssueNumber = remote.IssueNumber(record.BranchPrepare.IssueURL)
	}
	return policy
}

func ValidateOwnerCatalog(commands issueops.OwnerCommands) error {
	for _, path := range []string{
		"execution status", "execution claim", "branch prepare", "link-plan", "compatibility review", "phase", "ai-slop-clean record",
		"implementation-review record", "project-docs-review record", "schema-evidence record",
		"remote create-pr", "execution complete",
	} {
		if _, _, _, ok := commandparse.IssueOpsCommandSpec(path); !ok {
			return fmt.Errorf("IssueOps v1 command catalog is not ready: missing %s", path)
		}
	}
	for _, path := range []string{"worktree prepare", "handoff start", "handoff claim", "handoff acknowledge"} {
		if _, _, _, ok := commandparse.IssueOpsCommandSpec(path); ok {
			return fmt.Errorf("IssueOps v1 command catalog still exposes retired %s", path)
		}
	}
	checks := []struct{ command, path string }{
		{commands.LeaseStatus, "execution status"}, {commands.Claim, "execution claim"},
		{commands.VerifyBranchLink, "branch prepare"},
		{commands.LinkPlan, "link-plan"}, {commands.CompatibilityReview, "compatibility review"},
		{commands.EnterImplement, "phase"},
		{commands.AISlopCleanRecord, "ai-slop-clean record"}, {commands.EnterAISlopClean, "phase"},
		{commands.ImplementationReview, "implementation-review record"},
		{commands.ProjectDocsReview, "project-docs-review record"}, {commands.SchemaEvidence, "schema-evidence record"},
		{commands.EnterPR, "phase"},
		{commands.RemoteCreate, "remote create-pr"}, {commands.Complete, "execution complete"},
	}
	for _, check := range checks {
		if check.command == "none" {
			continue
		}
		command := executionCommandValue.ReplaceAllString(check.command, "VALUE")
		parsed, ok := commandparse.ParseExactIssueOpsCommand(command)
		if !ok || parsed.Path != check.path {
			return fmt.Errorf("IssueOps v1 owner command does not match catalog path %s", check.path)
		}
		values, booleans, repeatable, _ := commandparse.IssueOpsCommandSpec(check.path)
		if _, ok := commandparse.ExactFlags(parsed, values, booleans, repeatable); !ok {
			return fmt.Errorf("IssueOps v1 owner command flags do not match catalog path %s", check.path)
		}
	}
	return nil
}

func OwnerArtifactDir(record issueops.IssueOpsRecord) string {
	if dir := remote.IssueArtifactDir(record.IssueURL); dir != "" {
		return dir
	}
	if record.BranchPrepare != nil {
		return remote.IssueArtifactDir(record.BranchPrepare.IssueURL)
	}
	return ""
}
