package issueopslease

import issueopscontract "issueops/internal/contract/issueops"

// 아래 변환은 lease vertical의 persisted shape를 production execution DTO로
// 옮기는 유일한 경로다. complete/release/prepare adapter가 각자 복사하던 변환이
// 갈라져 prepare 응답에서 ArtifactDir이 빠졌으므로, field 추가는 이 파일 한 곳과
// TestExecutionToIssueOpsCopiesEveryField에서만 일어난다. nil과 빈 slice는 JSON
// 출력이 다르므로(`verification`은 omitempty가 아니다) 구분을 그대로 보존한다.

// ToIssueOps는 execution을 production DTO로 옮긴다. slice와 pointer는 복사해
// 호출자가 결과를 바꿔도 원본이 바뀌지 않는다.
func (execution Execution) ToIssueOps() issueopscontract.Execution {
	result := issueopscontract.Execution{
		Workspace: execution.Workspace.ToIssueOps(),
		Lease:     execution.Lease.toIssueOps(),
	}
	if execution.Pending != nil {
		pending := execution.Pending
		result.Pending = &issueopscontract.ExternalIntent{OperationID: pending.OperationID, Kind: pending.Kind, Marker: pending.Marker, StartedAt: pending.StartedAt}
	}
	if execution.Completion != nil {
		completion := execution.Completion.toIssueOps()
		result.Completion = &completion
	}
	if execution.CompletionHistory != nil {
		result.CompletionHistory = make([]issueopscontract.ExecutionCompletionHistory, len(execution.CompletionHistory))
		for index, entry := range execution.CompletionHistory {
			result.CompletionHistory[index] = issueopscontract.ExecutionCompletionHistory{
				Generation: entry.Generation, Completion: entry.Completion.toIssueOps(), Reason: entry.Reason, ReopenedAt: entry.ReopenedAt,
			}
		}
	}
	if execution.Failure != nil {
		failure := execution.Failure
		result.Failure = &issueopscontract.ExecutionFailure{OperationID: failure.OperationID, Code: failure.Code, Message: failure.Message, At: failure.At}
	}
	if execution.SyncBaseResolution != nil {
		resolution := execution.SyncBaseResolution
		result.SyncBaseResolution = &issueopscontract.ExecutionSyncBaseResolution{
			Generation: resolution.Generation, CompletionGeneration: resolution.CompletionGeneration, BaseOID: resolution.BaseOID,
			Actor: resolution.Actor.toIssueOps(), ConflictFiles: cloneStrings(resolution.ConflictFiles), StartedAt: resolution.StartedAt,
		}
	}
	if execution.SyncBaseEvents != nil {
		result.SyncBaseEvents = make([]issueopscontract.ExecutionSyncBaseEvent, len(execution.SyncBaseEvents))
		for index, event := range execution.SyncBaseEvents {
			result.SyncBaseEvents[index] = issueopscontract.ExecutionSyncBaseEvent{
				Mode: event.Mode, BaseBranch: event.BaseBranch, BaseOID: event.BaseOID, MergeCommit: event.MergeCommit,
				ConflictFiles: event.ConflictFiles, Actor: event.Actor, At: event.At,
			}
		}
	}
	return result
}

// ToIssueOps는 workspace를 production DTO로 옮긴다. ArtifactDir을 포함한 모든
// field를 옮긴다.
func (workspace Workspace) ToIssueOps() issueopscontract.Workspace {
	return issueopscontract.Workspace{
		SourceRoot: workspace.SourceRoot, Root: workspace.Root, Branch: workspace.Branch,
		BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree, LinkedAt: workspace.LinkedAt,
		ArtifactDir: workspace.ArtifactDir,
	}
}

func (lease Lease) toIssueOps() issueopscontract.WriteLease {
	result := issueopscontract.WriteLease{
		Generation: lease.Generation, Status: issueopscontract.LeaseStatus(lease.Status),
		ClaimTokenSHA256: lease.ClaimTokenSHA256, ClaimedAt: lease.ClaimedAt,
		ReleasedAt: lease.ReleasedAt, ReplacedAt: lease.ReplacedAt, ReplacementReason: lease.ReplacementReason,
	}
	if lease.Holder != nil {
		holder := lease.Holder.toIssueOps()
		result.Holder = &holder
	}
	return result
}

func (actor Actor) toIssueOps() issueopscontract.NativeActor {
	result := issueopscontract.NativeActor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.SessionProcess != nil {
		result.SessionProcess = &issueopscontract.NativeProcessReceipt{
			PID: actor.SessionProcess.PID, StartedAt: actor.SessionProcess.StartedAt, Executable: actor.SessionProcess.Executable,
		}
	}
	return result
}

func (completion Completion) toIssueOps() issueopscontract.ExecutionCompletion {
	return issueopscontract.ExecutionCompletion{
		Generation: completion.Generation, FinalHead: completion.FinalHead, VerificationReportPath: completion.VerificationReportPath,
		Verification: cloneStrings(completion.Verification), RemoteArtifactURL: completion.RemoteArtifactURL, CompletedAt: completion.CompletedAt,
	}
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string{}, values...)
}
