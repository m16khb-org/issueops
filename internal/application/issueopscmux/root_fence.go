package issueopscmux

import (
	"fmt"

	issueopscontract "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
	port "issueops/internal/port/cmux"
)

type rootFence struct {
	stateRoot, lifecycleID, requestedCWD, canonicalRoot, workspaceRoot, worktreePath string
	generation                                                                       uint64
	rootHandle                                                                       port.DirectoryPin
	env                                                                              port.RootEnvironment
}

func openRootFence(env port.RootEnvironment, stateRoot string, request issueopscontract.ExecutionCmuxHandoffRequest) (*rootFence, issueopscontract.IssueOpsRecord, error) {
	record, err := env.Read(stateRoot, request.ID)
	if err != nil {
		return nil, issueopscontract.IssueOpsRecord{}, err
	}
	identity, err := validateReleasedRoot(env, record, request.Generation, request.CWD)
	if err != nil {
		return nil, issueopscontract.IssueOpsRecord{}, err
	}
	handle, err := identity.Pin()
	if err != nil {
		return nil, issueopscontract.IssueOpsRecord{}, fmt.Errorf("open cmux handoff canonical worktree: %w", err)
	}
	fence := &rootFence{
		stateRoot: stateRoot, lifecycleID: request.ID, generation: request.Generation,
		requestedCWD: request.CWD, canonicalRoot: identity.Path(), rootHandle: handle,
		workspaceRoot: record.Execution.Workspace.Root, worktreePath: record.WorktreePath,
		env: env,
	}
	checked, err := fence.Check()
	if err != nil {
		handle.Close()
		return nil, issueopscontract.IssueOpsRecord{}, err
	}
	return fence, checked, nil
}

func (fence *rootFence) Check() (issueopscontract.IssueOpsRecord, error) {
	record, err := fence.env.Read(fence.stateRoot, fence.lifecycleID)
	if err != nil {
		return issueopscontract.IssueOpsRecord{}, err
	}
	if record.Execution == nil || record.Execution.Workspace.Root != fence.workspaceRoot || record.WorktreePath != fence.worktreePath {
		return issueopscontract.IssueOpsRecord{}, fmt.Errorf("cmux handoff durable worktree path changed")
	}
	currentIdentity, err := validateReleasedRoot(fence.env, record, fence.generation, fence.requestedCWD)
	if err != nil {
		return issueopscontract.IssueOpsRecord{}, err
	}
	if !fence.rootHandle.Matches(currentIdentity) {
		return issueopscontract.IssueOpsRecord{}, fmt.Errorf("cmux handoff canonical worktree filesystem identity changed")
	}
	return record, nil
}

func (fence *rootFence) Close() {
	if fence != nil && fence.rootHandle != nil {
		fence.rootHandle.Close()
	}
}

func validateReleasedRoot(env port.RootEnvironment, record issueopscontract.IssueOpsRecord, generation uint64, requestedCWD string) (port.Directory, error) {
	if err := domain.ValidateCmuxReleasedRecord(record, generation); err != nil {
		return nil, err
	}
	identity, err := env.Directory(record.Execution.Workspace.Root)
	if err != nil {
		return nil, fmt.Errorf("cmux handoff canonical worktree identity mismatch")
	}
	if !identity.SamePath(record.WorktreePath) {
		return nil, fmt.Errorf("cmux handoff durable worktree is not the canonical worktree")
	}
	if !identity.SamePath(requestedCWD) {
		return nil, fmt.Errorf("cmux handoff request cwd is not the canonical worktree")
	}
	processCWD, err := env.Getwd()
	if err != nil || !identity.SamePath(processCWD) {
		return nil, fmt.Errorf("cmux handoff process cwd is not the canonical worktree")
	}
	top, err := env.GitTop(identity.Path())
	if err != nil || !identity.SamePath(top) {
		return nil, fmt.Errorf("cmux handoff cwd is not the canonical Git worktree")
	}
	return identity, nil
}
