// Package issueopspreparation defines the transport-neutral contract for preparing an issueops cycle: the command,
// result, snapshot, and workspace types shared by domain, application, and adapters.
package issueopspreparation

import (
	"encoding/json"

	leasecontract "issueops/internal/contract/issueopslease"
)

// Record aliases the lease contract's cycle record under this package's name.
// domain/issueopspreparation은 이 contract 패키지만 import할 수 있어서
// (preparation domain의 dependency 규칙) lease 어휘를 여기서 이 이름으로 둔다.
type Record = leasecontract.Record

// Command is the request to prepare a cycle: the cycle ID, the acting caller, the working directory,
// and whether to apply (Confirm) or only preview.
type Command struct {
	ID      string              `json:"id"`
	Actor   leasecontract.Actor `json:"actor"`
	CWD     string              `json:"cwd"`
	Confirm bool                `json:"confirm,omitempty"`
}

// Clone returns a copy that shares no mutable state with command.
func (command Command) Clone() Command {
	cloned := command
	cloned.Actor = cloneActor(command.Actor)
	return cloned
}

// Result reports a preparation: the cycle ID, whether it was only a preview, the workspace,
// the resulting execution, the claim token path, and the next command to run.
type Result struct {
	OK             bool                     `json:"ok"`
	ID             string                   `json:"id"`
	Preview        bool                     `json:"preview,omitempty"`
	Workspace      leasecontract.Workspace  `json:"workspace"`
	Execution      *leasecontract.Execution `json:"execution,omitempty"`
	ClaimTokenPath string                   `json:"claim_token_path,omitempty"`
	NextCommand    string                   `json:"next_command,omitempty"`
}

// Clone returns a copy that shares no mutable state with result.
func (result Result) Clone() Result {
	cloned := result
	cloned.Execution = cloneExecutionPointer(result.Execution)
	return cloned
}

// Snapshot is the persisted state preparation reads: the cycle record with its raw bytes,
// the canonical root, and the claim token path.
type Snapshot struct {
	Record         leasecontract.Record
	RecordRaw      []byte
	CanonicalRoot  string
	ClaimTokenPath string
	// ArtifactDir is the sealed-artifact directory derived from the linked
	// issue URL; empty when the URL carries no issue number.
	ArtifactDir string
}

// WorkspaceRequest is the canonical worktree the preparation will create.
type WorkspaceRequest struct {
	LifecycleID    string `json:"lifecycle_id"`
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseBranch     string `json:"base_branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Confirm        bool   `json:"confirm,omitempty"`
	CWD            string `json:"-"`
}

// WorkspaceReceipt reports the worktree that exists after preparation: source root, root, branch,
// base head, optional parent worktree, and whether it already existed.
type WorkspaceReceipt struct {
	SourceRoot     string `json:"source_root"`
	Root           string `json:"root"`
	Branch         string `json:"branch"`
	BaseHead       string `json:"base_head"`
	ParentWorktree string `json:"parent_worktree,omitempty"`
	Exists         bool   `json:"exists,omitempty"`
}

// AccessResult is the decision of the access check on preparation: whether the caller is allowed and, if
// not, the denial code and the command to relaunch with.
type AccessResult struct {
	Allowed         bool   `json:"allowed"`
	Code            string `json:"code,omitempty"`
	RelaunchCommand string `json:"relaunch_command,omitempty"`
}

// Clone returns a deep copy of the snapshot, including the record and its raw bytes.
func (snapshot Snapshot) Clone() Snapshot {
	cloned := snapshot
	cloned.Record = cloneRecord(snapshot.Record)
	cloned.RecordRaw = cloneBytes(snapshot.RecordRaw)
	return cloned
}

func cloneRecord(record leasecontract.Record) leasecontract.Record {
	cloned := record
	cloned.Intent = cloneRaw(record.Intent)
	cloned.ArchitectureReview = cloneRaw(record.ArchitectureReview)
	cloned.DomainReview = cloneRaw(record.DomainReview)
	cloned.IssueLinks = cloneRaw(record.IssueLinks)
	cloned.BranchPrepare = cloneRaw(record.BranchPrepare)
	cloned.RemoteArtifact = cloneRaw(record.RemoteArtifact)
	cloned.Decisions = cloneRaw(record.Decisions)
	cloned.PlanPrep = cloneRaw(record.PlanPrep)
	cloned.CompatibilityReview = cloneRaw(record.CompatibilityReview)
	cloned.DevilsAdvocateReview = cloneRaw(record.DevilsAdvocateReview)
	cloned.Feedback = cloneRaw(record.Feedback)
	cloned.RegressEvents = cloneRaw(record.RegressEvents)
	cloned.Delegation = cloneRaw(record.Delegation)
	cloned.ChildCycles = cloneRaw(record.ChildCycles)
	cloned.RemoteCompletion = cloneRaw(record.RemoteCompletion)
	if record.CleanupAttempt != nil {
		attempt := *record.CleanupAttempt
		cloned.CleanupAttempt = &attempt
	}
	cloned.CleanupFinishFailure = cloneRaw(record.CleanupFinishFailure)
	cloned.ImplementationReview = cloneRaw(record.ImplementationReview)
	cloned.RoutingTrace = cloneRaw(record.RoutingTrace)
	cloned.AISlopCleanCategories = cloneRaw(record.AISlopCleanCategories)
	cloned.AISlopCleanVerification = cloneRaw(record.AISlopCleanVerification)
	cloned.PhaseLedger = cloneRaw(record.PhaseLedger)
	cloned.Execution = cloneExecutionPointer(record.Execution)
	return cloned
}

func cloneExecutionPointer(execution *leasecontract.Execution) *leasecontract.Execution {
	if execution == nil {
		return nil
	}
	cloned := *execution
	cloned.Lease.Holder = cloneActorPointer(execution.Lease.Holder)
	if execution.Pending != nil {
		pending := *execution.Pending
		cloned.Pending = &pending
	}
	if execution.Completion != nil {
		completion := *execution.Completion
		completion.Verification = cloneStrings(execution.Completion.Verification)
		cloned.Completion = &completion
	}
	if execution.CompletionHistory != nil {
		cloned.CompletionHistory = append([]leasecontract.CompletionHistoryEntry(nil), execution.CompletionHistory...)
		for index := range cloned.CompletionHistory {
			cloned.CompletionHistory[index].Completion.Verification = cloneStrings(execution.CompletionHistory[index].Completion.Verification)
		}
	}
	if execution.Failure != nil {
		failure := *execution.Failure
		cloned.Failure = &failure
	}
	if execution.SyncBaseResolution != nil {
		resolution := *execution.SyncBaseResolution
		resolution.Actor = cloneActor(execution.SyncBaseResolution.Actor)
		resolution.ConflictFiles = cloneStrings(execution.SyncBaseResolution.ConflictFiles)
		cloned.SyncBaseResolution = &resolution
	}
	if execution.SyncBaseEvents != nil {
		cloned.SyncBaseEvents = append([]leasecontract.SyncBaseEvent{}, execution.SyncBaseEvents...)
	}
	return &cloned
}

func cloneActor(actor leasecontract.Actor) leasecontract.Actor {
	cloned := actor
	if actor.SessionProcess != nil {
		process := *actor.SessionProcess
		cloned.SessionProcess = &process
	}
	return cloned
}

func cloneActorPointer(actor *leasecontract.Actor) *leasecontract.Actor {
	if actor == nil {
		return nil
	}
	cloned := cloneActor(*actor)
	return &cloned
}

func cloneRaw(value json.RawMessage) json.RawMessage {
	if value == nil {
		return nil
	}
	return append(json.RawMessage{}, value...)
}

func cloneBytes(value []byte) []byte {
	if value == nil {
		return nil
	}
	return append([]byte{}, value...)
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
