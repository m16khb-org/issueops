package issueopsreplacement

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

func (s Service) inventory(ctx context.Context, record issueops.IssueOpsRecord, requester issueops.NativeActor) (string, port.ExecutionOrcaOwnerInventory, error) {
	snapshot, err := s.Workspace.WorkspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", port.ExecutionOrcaOwnerInventory{}, err
	}
	processSnapshot := s.ObserveProcesses()
	processStatus, orcaInventory, err := s.ownerInventory(ctx, record, processSnapshot)
	if err != nil {
		return "", port.ExecutionOrcaOwnerInventory{}, err
	}
	if err := domain.ValidateReplacementRuntimeRollover(record, processStatus, runtimeFacts(orcaInventory)); err != nil {
		return "", port.ExecutionOrcaOwnerInventory{}, err
	}
	payload := struct {
		ID         string                           `json:"id"`
		Generation uint64                           `json:"generation"`
		Status     issueops.LeaseStatus             `json:"status"`
		Holder     *issueops.NativeActor            `json:"holder,omitempty"`
		Requester  issueops.NativeActor             `json:"requester"`
		Process    string                           `json:"process_status"`
		Orca       port.ExecutionOrcaOwnerInventory `json:"orca"`
		Snapshot   string                           `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, record.Execution.Lease.Status, record.Execution.Lease.Holder, requester, processStatus, orcaInventory, snapshot}
	fingerprint, err := hashJSON(payload)
	return fingerprint, orcaInventory, err
}

func (s Service) quiescence(ctx context.Context, record issueops.IssueOpsRecord, requester issueops.NativeActor) (string, error) {
	holder := record.Execution.Lease.Holder
	if err := domain.ValidateReplacementHolderProcess(holder); err != nil {
		return "", err
	}
	processSnapshot := s.ObserveProcesses()
	processStatus, _, err := processSnapshot.Inspect(*holder.SessionProcess)
	if err != nil {
		return "", err
	}
	if err := domain.ValidateReplacementQuiescence(*holder.SessionProcess, processStatus); err != nil {
		return "", err
	}
	orcaInventory, err := s.orcaOwnerInventory(ctx, record, processStatus)
	if err != nil {
		return "", err
	}
	if err := domain.ValidateReplacementRuntimeRollover(record, processStatus, runtimeFacts(orcaInventory)); err != nil {
		return "", err
	}
	if err := domain.ValidateReplacementOwnerQuiescence(record, processStatus, runtimeFacts(orcaInventory)); err != nil {
		return "", err
	}
	inventoryOwners := map[int]bool{s.PID: true}
	requesterOwners := map[int]bool{}
	if requester.SessionProcess != nil {
		inventoryOwners[requester.SessionProcess.PID] = true
		requesterOwners[requester.SessionProcess.PID] = true
	}
	excluded := map[int]bool{}
	for pid := range inventoryOwners {
		for ancestor := range processSnapshot.AncestryPIDs(pid) {
			excluded[ancestor] = true
		}
	}
	workspaceProcesses, err := s.Workspace.WorkspaceProcesses(record.Execution.Workspace.Root, excluded)
	if err != nil {
		return "", err
	}
	if len(requesterOwners) > 0 {
		kept := make([]issueops.ReplacementWorkspaceProcess, 0, len(workspaceProcesses))
		for _, process := range workspaceProcesses {
			if !processSnapshot.HasAncestor(process.PID, requesterOwners) {
				kept = append(kept, process)
			}
		}
		workspaceProcesses = kept
	}
	if len(workspaceProcesses) > 0 {
		process := workspaceProcesses[0]
		return "", fmt.Errorf("workspace process is not quiescent: pid=%d command=%s fd=%s access=%s path=%s", process.PID, process.Command, process.FD, process.Access, process.Path)
	}
	snapshot, err := s.Workspace.WorkspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", err
	}
	payload := struct {
		ID         string                           `json:"id"`
		Generation uint64                           `json:"generation"`
		Holder     issueops.NativeActor             `json:"holder"`
		Requester  issueops.NativeActor             `json:"requester"`
		Process    issueops.NativeProcessReceipt    `json:"process"`
		Orca       port.ExecutionOrcaOwnerInventory `json:"orca"`
		Snapshot   string                           `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, *holder, requester, *holder.SessionProcess, orcaInventory, snapshot}
	return hashJSON(payload)
}

func (s Service) ownerInventory(
	ctx context.Context,
	record issueops.IssueOpsRecord,
	processSnapshot port.ReplacementProcessSnapshot,
) (string, port.ExecutionOrcaOwnerInventory, error) {
	status := "none"
	if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
		var err error
		status, _, err = processSnapshot.Inspect(*holder.SessionProcess)
		if err != nil {
			return "", port.ExecutionOrcaOwnerInventory{}, err
		}
	}
	inventory, err := s.orcaOwnerInventory(ctx, record, status)
	return status, inventory, err
}

func (s Service) orcaOwnerInventory(
	ctx context.Context,
	record issueops.IssueOpsRecord,
	status string,
) (port.ExecutionOrcaOwnerInventory, error) {
	if record.Execution.Mode != issueops.ExecutionModeOrca {
		return port.ExecutionOrcaOwnerInventory{}, nil
	}
	if record.Execution.Orca == nil || s.OrcaOwner == nil {
		return port.ExecutionOrcaOwnerInventory{}, fmt.Errorf("Orca execution requires exact owner terminal and task inventory")
	}
	binding := record.Execution.Orca
	inventory, err := s.OrcaOwner.InspectOwner(ctx, port.ExecutionOrcaOwnerInventoryRequest{
		RuntimeID: binding.RuntimeID, WorktreeID: binding.WorktreeID, RunID: binding.RunID, TaskID: binding.TaskID,
		DispatchID: binding.DispatchID, TerminalPTYID: binding.TerminalPTYID,
		AllowRuntimeRollover: domain.AllowReplacementRuntimeRollover(record, status),
	})
	return inventory, err
}

func runtimeFacts(inventory port.ExecutionOrcaOwnerInventory) domain.ReplacementRuntimeFacts {
	return domain.ReplacementRuntimeFacts{RuntimeID: inventory.RuntimeID, TerminalID: inventory.TerminalID, TerminalLive: inventory.TerminalLive, TaskLive: inventory.TaskLive, TaskStatus: inventory.TaskStatus, DispatchStatus: inventory.DispatchStatus}
}
func hashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
func (s Service) refuseSelfRevoke(id string, lease issueops.WriteLease, actor issueops.NativeActor) error {
	if !domain.ReplacementSelfHolder(lease, actor) {
		return nil
	}
	status, _, err := s.InspectProcess(*lease.Holder.SessionProcess)
	if err != nil {
		return nil
	}
	return domain.ValidateReplacementSelfRevoke(id, lease.Generation, status)
}
func (s Service) validateCWD(record issueops.IssueOpsRecord, cwd string) error {
	sourceMatches := s.Workspace.SamePath(cwd, record.Execution.Workspace.SourceRoot)
	workspaceMatches := false
	if !sourceMatches {
		workspaceMatches = s.Workspace.SamePath(cwd, record.Execution.Workspace.Root)
	}
	return domain.ValidateReplacementCWD(sourceMatches, workspaceMatches)
}
