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
	if record.Invalid || normalize(record.Repo) != sourceRoot {
		result.Selected = entryOf(issueopsinventorycontract.ListEntry{ID: id, Invalid: true})
		result.Stage = issueopsnextcontract.Stage{Key: issueopsnextcontract.StageInvalid}
		result.NextCommand = "issueops status --id " + id + " --json"
		result.NextCommandKind = commandKind(result.NextCommand)
		return result, nil
	}
	result.Selected = recordEntry(record)
	input := service.buildInput(stateRoot, sourceRoot, record, nil, actorHost, actorSession)
	if record.Execution == nil && strings.TrimSpace(record.Branch) != "" {
		if ports.ScanRecords == nil {
			return issueopsnextcontract.Result{OK: false}, fmt.Errorf("issueops next conflict lookup dependency is required")
		}
		err := ports.ScanRecords(ctx, stateRoot, func(candidate issueopscontract.IssueOpsRecord) error {
			if input.RootConflictID != "" || candidate.ID == record.ID || candidate.Execution == nil {
				return nil
			}
			if normalize(candidate.Repo) != sourceRoot {
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
