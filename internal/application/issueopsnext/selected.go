package issueopsnext

import (
	"context"
	"fmt"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
	issueopsinventorycontract "issueops/internal/contract/issueopsinventory"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
	issueopsnextdomain "issueops/internal/domain/issueopsnext"
)

func (service *Service) nextSelected(
	ctx context.Context,
	stateRoot, sourceRoot, id string,
	result issueopsnextcontract.Result,
	actorHost, actorSession string,
	fromEnv bool,
) (issueopsnextcontract.Result, error) {
	ports := service.ports
	if ports.ReadSelected == nil || ports.SourceRoot == nil {
		return issueopsnextcontract.Result{OK: false}, fmt.Errorf("issueops next selected lookup dependencies are required")
	}
	record, err := ports.ReadSelected(ctx, stateRoot, id)
	if err != nil {
		return issueopsnextcontract.Result{OK: false}, err
	}
	normalizedRepos := map[string]string{sourceRoot: sourceRoot}
	normalize := func(repo string) string {
		if normalized, ok := normalizedRepos[repo]; ok {
			return normalized
		}
		normalized := ports.SourceRoot(repo)
		normalizedRepos[repo] = normalized
		return normalized
	}
	if record.Invalid || !sameDir(ports, normalize(record.Repo), sourceRoot) {
		result.Selected = entryOf(issueopsinventorycontract.ListEntry{ID: id, Invalid: true})
		result.Stage = issueopsnextcontract.Stage{Key: issueopsnextcontract.StageInvalid}
		result.NextCommand = "issueops status --id " + id + " --json"
		if record.Invalid {
			// status cannot read a missing record either; list shows what exists.
			result.NextCommand = "issueops list --repo " + placeholder(sourceRoot, "<source_root>") + " --json"
			warning := "no readable record for " + id + " (cleaned up, abandoned, or in another state dir)"
			if fromEnv {
				warning += "; the id came from ISSUEOPS_ID: unset it so next picks the cycle itself"
			}
			result.Warnings = append(result.Warnings, warning)
		}
		result.NextCommandKind = commandKind(result.NextCommand)
		return result, nil
	}
	result.Selected = recordEntry(record)
	input := service.buildInput(ctx, stateRoot, sourceRoot, record, nil, actorHost, actorSession)
	if record.Execution == nil && strings.TrimSpace(record.Branch) != "" {
		if ports.ScanRecords == nil {
			return issueopsnextcontract.Result{OK: false}, fmt.Errorf("issueops next conflict lookup dependency is required")
		}
		err := ports.ScanRecords(ctx, stateRoot, func(candidate issueopscontract.IssueOpsRecord) error {
			if input.RootConflictID != "" || candidate.ID == record.ID || candidate.Execution == nil {
				return nil
			}
			if !sameDir(ports, normalize(candidate.Repo), sourceRoot) {
				return nil
			}
			input.RootConflictID = rootConflict(ports, record, sourceRoot, []issueopsinventorycontract.ListEntry{{
				ID: candidate.ID, WorkspaceRoot: candidate.Execution.Workspace.Root,
			}})
			return nil
		})
		if err != nil {
			return issueopsnextcontract.Result{OK: false}, err
		}
	}
	decided := applyDecision(result, issueopsnextdomain.Classify(input))
	return service.applyReviewTier(decided, record, actorHost), nil
}
