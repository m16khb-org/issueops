package issueopsbasesync

type RecordGateFacts struct {
	ExecutionPresent      bool
	LeaseActive           bool
	CompletionPresent     bool
	CompletionGeneration  uint64
	RemoteArtifactPresent bool
	PendingIntentAbsent   bool
	BaseBranchPresent     bool
}

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

type MergeStateFacts struct {
	Mode            string
	MergeInProgress bool
	TrackedDirty    bool
}

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
