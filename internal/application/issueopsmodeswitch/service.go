package issueopsmodeswitch

import (
	"context"
	"fmt"
	"strings"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopsmodeswitch"
	cycledomain "issueops/internal/domain/issueops"
	leasedomain "issueops/internal/domain/issueopslease"
	modeswitchdomain "issueops/internal/domain/issueopsmodeswitch"
	"issueops/internal/port"
)

type Service struct {
	Records   port.ModeSwitchRecords
	Workspace port.ModeSwitchWorkspace
	Now       func() string
}

func (s Service) Run(ctx context.Context, req model.ExecutionSwitchModeRequest) (model.ExecutionSwitchModeResult, error) {
	requested, err := modeswitchdomain.NormalizeMode(req.Mode)
	if err != nil {
		return model.ExecutionSwitchModeResult{OK: false, ID: req.ID}, err
	}
	record, err := s.Records.Load(req.ID)
	if err != nil {
		return model.ExecutionSwitchModeResult{OK: false, ID: req.ID}, err
	}
	if record.Execution == nil {
		return model.ExecutionSwitchModeResult{OK: false, ID: req.ID}, fmt.Errorf(
			"IssueOps execution is not prepared; run `issueops execution prepare --id %s --mode %s` instead", record.ID, requested)
	}

	result := model.ExecutionSwitchModeResult{
		OK: true, ID: record.ID, Preview: !req.Apply,
		CurrentMode: string(record.Execution.Mode), RequestedMode: requested,
		LeaseGeneration: record.Execution.Lease.Generation,
	}
	inventory, missing := s.observe(record, requested, &result)
	result.Missing = missing
	if len(missing) > 0 {
		result.OK = false
		return result, fmt.Errorf("execution switch-mode is not ready: %s", strings.Join(missing, ", "))
	}
	fingerprint, err := modeswitchdomain.InventoryFingerprint(inventory)
	if err != nil {
		return model.ExecutionSwitchModeResult{OK: false, ID: record.ID}, err
	}
	result.Fingerprint = fingerprint
	if !req.Apply {
		result.NextCommand = fmt.Sprintf(
			"issueops execution switch-mode --id %s --mode %s --apply --confirm --fingerprint %s --json",
			record.ID, requested, fingerprint)
		return result, nil
	}
	if err := modeswitchdomain.ValidateApply(req.Confirm, req.Fingerprint, fingerprint); err != nil {
		result.OK = false
		return result, err
	}

	expectedSHA, err := cycledomain.ModeSwitchRecordFingerprint(record)
	if err != nil {
		return model.ExecutionSwitchModeResult{OK: false, ID: record.ID}, err
	}
	err = Apply(ctx, ApplyRequest{
		ID: record.ID, Repo: record.Repo, WorktreeRoot: inventory.WorktreeRoot,
		WorktreePresent: inventory.WorktreePresent, Branch: inventory.Branch,
		BranchPresent: inventory.BranchOID != "", ExpectedRecordSHA: expectedSHA,
	}, s)
	if err != nil {
		result.OK = false
		return result, err
	}
	result.WorktreePresent = false
	result.BranchPresent = false
	result.SwitchedAt = s.Now()
	result.NextAction = fmt.Sprintf("prepare IssueOps execution in %s mode", requested)
	return result, nil
}

func (s Service) observe(record model.IssueOpsRecord, requested string, result *model.ExecutionSwitchModeResult) (contract.Inventory, []string) {
	inventory := inventoryFor(record, requested)
	result.WorktreeRoot = inventory.WorktreeRoot
	result.Branch = inventory.Branch
	facts := modeswitchdomain.Facts{CurrentMode: inventory.CurrentMode, RequestedMode: requested, WriterPresent: leasedomain.LeaseHoldsWriter(inventory.LeaseStatus), PendingIntent: record.Execution.Pending != nil, WorktreeClean: true, NoUnpushedCommits: true}
	if inventory.WorktreeRoot != "" && s.Workspace.Present(inventory.WorktreeRoot) {
		inventory.WorktreePresent = true
		facts.WorktreePresent = true
		result.WorktreePresent = true
		facts.WorktreeClean = s.Workspace.Clean(inventory.WorktreeRoot)
		if inventory.Branch != "" {
			facts.NoUnpushedCommits = false
			for _, ref := range modeswitchdomain.ComparisonRefs(inventory.Branch, record.Execution.Workspace.BaseHead) {
				if count, ok := s.Workspace.CommitCount(inventory.WorktreeRoot, ref); ok {
					facts.NoUnpushedCommits = count == "0"
					break
				}
			}
		}
	}
	if inventory.Branch != "" {
		if oid, ok := s.Workspace.BranchOID(record.Repo, inventory.Branch, false); ok {
			inventory.BranchOID = oid
			result.BranchPresent = true
		}
	}
	if requested == string(model.ExecutionModeOrca) && inventory.Branch != "" {
		if _, ok := s.Workspace.BranchOID(record.Repo, inventory.Branch, true); ok {
			facts.OrcaRemoteBranchExists = true
			result.BranchFreeError = fmt.Sprintf("branch %q still exists on origin, so Orca would take a suffixed name after the switch: "+"delete the remote branch if it holds no work, run `git fetch --prune` if it is already gone, "+"or prepare Orca first and re-attach the linked branch afterwards", inventory.Branch)
		}
	}
	return inventory, modeswitchdomain.MissingGates(facts)
}
func (s Service) RemoveWorktree(ctx context.Context, repo, root string) error {
	return s.Workspace.RemoveWorktree(ctx, repo, root)
}
func (s Service) RemoveBranch(ctx context.Context, repo, branch string) error {
	return s.Workspace.RemoveBranch(ctx, repo, branch)
}
func (s Service) ResetExecution(ctx context.Context, id, expectedSHA string) error {
	return s.Records.WithinLock(ctx, id, func(spanCtx context.Context) error {
		current, err := s.Records.Load(id)
		if err != nil {
			return err
		}
		currentSHA, err := cycledomain.ModeSwitchRecordFingerprint(current)
		if err != nil {
			return err
		}
		if currentSHA != expectedSHA {
			return fmt.Errorf("execution switch-mode authority changed before record mutation")
		}
		_, err = s.Records.Save(spanCtx, cycledomain.ResetExecutionForModeSwitch(current))
		return err
	})
}

func inventoryFor(record model.IssueOpsRecord, requested string) contract.Inventory {
	execution := record.Execution
	inventory := contract.Inventory{ID: record.ID, Repo: record.Repo, Branch: strings.TrimSpace(execution.Workspace.Branch), CurrentMode: string(execution.Mode), RequestedMode: requested, WorktreeRoot: strings.TrimSpace(execution.Workspace.Root), LeaseStatus: string(execution.Lease.Status), LeaseGeneration: execution.Lease.Generation}
	if execution.Pending != nil {
		inventory.PendingID = strings.TrimSpace(execution.Pending.OperationID)
	}
	return inventory
}
