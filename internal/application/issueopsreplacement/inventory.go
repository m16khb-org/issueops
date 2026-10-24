package issueopsreplacement

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	"issueops/internal/port"
)

func (s Service) inventory(record issueops.IssueOpsRecord, requester issueops.NativeActor) (string, error) {
	snapshot, err := s.Workspace.WorkspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", err
	}
	processStatus := "none"
	if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
		processStatus, _, err = s.ObserveProcesses().Inspect(*holder.SessionProcess)
		if err != nil {
			return "", err
		}
	}
	payload := struct {
		ID         string                `json:"id"`
		Generation uint64                `json:"generation"`
		Status     issueops.LeaseStatus  `json:"status"`
		Holder     *issueops.NativeActor `json:"holder,omitempty"`
		Requester  issueops.NativeActor  `json:"requester"`
		Process    string                `json:"process_status"`
		Snapshot   string                `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, record.Execution.Lease.Status, record.Execution.Lease.Holder, requester, processStatus, snapshot}
	return hashJSON(payload)
}

func (s Service) quiescence(record issueops.IssueOpsRecord, requester issueops.NativeActor) (string, error) {
	holder := record.Execution.Lease.Holder
	if err := domain.ValidateReplacementHolderProcess(holder); err != nil {
		return "", err
	}
	processSnapshot := s.ObserveProcesses()
	processStatus, observed, err := processSnapshot.Inspect(*holder.SessionProcess)
	if err != nil {
		return "", err
	}
	if err := domain.ValidateReplacementQuiescence(*holder.SessionProcess, processStatus, observed); err != nil {
		return "", err
	}
	if err := s.requireQuiescentWorkspaceProcesses(record, processSnapshot, requester); err != nil {
		return "", err
	}
	snapshot, err := s.Workspace.WorkspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", err
	}
	payload := struct {
		ID         string                        `json:"id"`
		Generation uint64                        `json:"generation"`
		Holder     issueops.NativeActor          `json:"holder"`
		Requester  issueops.NativeActor          `json:"requester"`
		Process    issueops.NativeProcessReceipt `json:"process"`
		Snapshot   string                        `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, *holder, requester, *holder.SessionProcess, snapshot}
	return hashJSON(payload)
}

// requireQuiescentWorkspaceProcesses는 요청자 조상 체인을 제외한 워크스페이스 프로세스가 남아 있으면 오류를 돌려준다.
func (s Service) requireQuiescentWorkspaceProcesses(record issueops.IssueOpsRecord, processSnapshot port.ReplacementProcessSnapshot, requester issueops.NativeActor) error {
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
		return err
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
		return fmt.Errorf("workspace process is not quiescent: pid=%d command=%s fd=%s access=%s path=%s", process.PID, process.Command, process.FD, process.Access, process.Path)
	}
	return nil
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
