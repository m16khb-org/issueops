package issueopsbasesync

// RecordGateFacts are the lifecycle record facts that gate a sync-base run:
// execution and completion presence, lease state, remote artifact, pending
// intent, and base branch.
type RecordGateFacts struct {
	ExecutionPresent      bool
	LeaseActive           bool
	CompletionPresent     bool
	CompletionGeneration  uint64
	RemoteArtifactPresent bool
	PendingIntentAbsent   bool
	BaseBranchPresent     bool
}

// MissingRecordGates returns the names of the record gates facts fails. A
// missing execution short-circuits to execution_prepared; a lease that is not
// active additionally needs a current completion and a remote artifact.
func MissingRecordGates(facts RecordGateFacts) []string {
	if !facts.ExecutionPresent {
		return []string{"execution_prepared"}
	}
	missing := []string{}
	if !facts.LeaseActive {
		if !facts.CompletionPresent {
			missing = append(missing, "completion_present")
		} else if facts.CompletionGeneration == 0 {
			missing = append(missing, "current_completion_generation_present")
		}
		if !facts.RemoteArtifactPresent {
			missing = append(missing, "remote_artifact_present")
		}
	}
	if !facts.PendingIntentAbsent {
		missing = append(missing, "pending_intent_absent")
	}
	if !facts.BaseBranchPresent {
		missing = append(missing, "base_branch_present")
	}
	return missing
}

// MergeStateFacts are the worktree facts of a sync-base run: its Mode, whether
// a merge is in progress, and whether tracked files are dirty.
type MergeStateFacts struct {
	Mode            string
	MergeInProgress bool
	TrackedDirty    bool
}

// MissingMergeStateGates returns the merge-state gates facts fails: apply needs
// a clean worktree with no merge in progress; finalize and abort need a merge
// in progress.
func MissingMergeStateGates(facts MergeStateFacts) []string {
	missing := []string{}
	switch facts.Mode {
	case "apply":
		if facts.MergeInProgress {
			missing = append(missing, "merge_state_clean")
		}
		if facts.TrackedDirty {
			missing = append(missing, "worktree_clean")
		}
	case "finalize", "abort":
		if !facts.MergeInProgress {
			missing = append(missing, "merge_in_progress")
		}
	}
	return missing
}
