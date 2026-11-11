package issueopsnext

import (
	"context"
	"fmt"
	"strings"
	"time"

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
	now time.Time,
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
		return invalidSelectionResult(result, record, id, sourceRoot, fromEnv), nil
	}
	result.Selected = recordEntry(record)
	input := service.buildInput(ctx, stateRoot, sourceRoot, record, nil, actorHost, actorSession)
	if record.Execution == nil && strings.TrimSpace(record.Branch) != "" {
		if ports.ScanRecords == nil {
			return issueopsnextcontract.Result{OK: false}, fmt.Errorf("issueops next conflict lookup dependency is required")
		}
		if err := scanRootConflict(ctx, ports, stateRoot, sourceRoot, record, &input, normalize); err != nil {
			return issueopsnextcontract.Result{OK: false}, err
		}
	}
	decided := applyDecision(result, issueopsnextdomain.Classify(input))
	decided = applyIdleWarning(decided, record, now)
	return service.applyReviewTier(decided, record, actorHost), nil
}

// invalidSelectionResult marks an explicitly selected cycle that is missing or
// belongs to another repository, and points at the command that still works.
func invalidSelectionResult(result issueopsnextcontract.Result, record issueopscontract.Record, id, sourceRoot string, fromEnv bool) issueopsnextcontract.Result {
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
	} else {
		// Probe: a valid cycle of another repository read as stage
		// "invalid" with no reason, as if the record were broken.
		warning := id + " belongs to " + record.Repo + ", not this repository; run next from there (or its worktree)"
		if fromEnv {
			warning += "; the id came from ISSUEOPS_ID: unset it for this repository"
		}
		result.Warnings = append(result.Warnings, warning)
	}
	result.NextCommandKind = commandKind(result.NextCommand)
	return result
}

// scanRootConflict sets input.RootConflictID from the first other cycle of
// this repository whose execution workspace conflicts with record's root.
func scanRootConflict(
	ctx context.Context,
	ports Ports,
	stateRoot, sourceRoot string,
	record issueopscontract.Record,
	input *issueopsnextdomain.Input,
	normalize func(string) string,
) error {
	return ports.ScanRecords(ctx, stateRoot, func(candidate issueopscontract.Record) error {
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
}
