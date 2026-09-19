package issueops

import (
	"sort"
	"strings"

	issueopscontract "issueops/internal/contract/issueops"
)

func EvaluateHandoff(snapshot issueopscontract.IssueOpsHandoffSnapshot) issueopscontract.IssueOpsHandoffDecision {
	decision := issueopscontract.IssueOpsHandoffDecision{}
	if fatal := handoffFatalRejectReasons(snapshot); len(fatal) > 0 {
		decision.RejectReasons = fatal
		return decision
	}

	for _, task := range snapshot.Tasks {
		if handoffTaskIsWriter(task) {
			if task.Live || !task.Terminated {
				decision.RejectReasons = append(decision.RejectReasons, "pending_writer:"+task.ID)
			}
			if task.DescendantsLive {
				decision.RejectReasons = append(decision.RejectReasons, "live_descendant:"+task.ID)
			}
			if task.CancellationRequested && (task.Live || task.DescendantsLive || !task.Terminated) {
				decision.RejectReasons = append(decision.RejectReasons, "cancellation_not_observed:"+task.ID)
			}
			if task.Live || task.DescendantsLive || !task.Terminated {
				decision.SelectiveRecheck = append(decision.SelectiveRecheck, "task:"+task.ID)
			}
		}
	}
	if !snapshot.Current.SharedStateRechecked {
		decision.RejectReasons = append(decision.RejectReasons, "shared_state_not_rechecked")
		decision.SelectiveRecheck = appendIfMissing(decision.SelectiveRecheck, "shared_state_recheck")
	}

	if snapshot.Current.FullHead != snapshot.Sealed.FullHead {
		decision.RejectReasons = append(decision.RejectReasons, "head_changed")
		decision.SelectiveRecheck = append(decision.SelectiveRecheck, "head")
	}
	if snapshot.Current.MaterialDigest != snapshot.Sealed.MaterialDigest {
		decision.RejectReasons = append(decision.RejectReasons, "material_changed")
		decision.SelectiveRecheck = append(decision.SelectiveRecheck, "material")
	}
	if snapshot.Current.PlanDigest != snapshot.Sealed.PlanDigest {
		decision.RejectReasons = append(decision.RejectReasons, "plan_changed")
		decision.SelectiveRecheck = append(decision.SelectiveRecheck, "plan")
	}
	freshnessChanged := containsAnyReason(decision.RejectReasons, "head_changed", "material_changed", "plan_changed")

	currentEvidence := handoffEvidenceMap(snapshot.Current.Evidence)
	for _, required := range snapshot.Sealed.RequiredEvidence {
		if freshnessChanged {
			decision.SelectiveRecheck = append(decision.SelectiveRecheck, "evidence:"+required.ID)
			continue
		}
		current, ok := currentEvidence[required.ID]
		if !ok {
			decision.RejectReasons = append(decision.RejectReasons, "missing_evidence:"+required.ID)
			decision.SelectiveRecheck = append(decision.SelectiveRecheck, "evidence:"+required.ID)
			continue
		}
		if current.Digest != required.Digest || current.Head != snapshot.Current.FullHead ||
			current.PlanDigest != snapshot.Current.PlanDigest || current.MaterialDigest != snapshot.Current.MaterialDigest ||
			current.InputRevision != required.InputRevision {
			decision.RejectReasons = append(decision.RejectReasons, "evidence_changed:"+required.ID)
			decision.SelectiveRecheck = append(decision.SelectiveRecheck, "evidence:"+required.ID)
			continue
		}
		decision.ReusableEvidence = append(decision.ReusableEvidence, required.ID)
	}
	sort.Strings(decision.ReusableEvidence)

	for _, late := range snapshot.LateResults {
		if late.ArrivedAfterRelease || late.AttemptsSourceChange {
			decision.Quarantine = append(decision.Quarantine, late.ID)
		}
	}
	sort.Strings(decision.Quarantine)

	decision.SelectiveRecheck = uniqueSorted(decision.SelectiveRecheck)
	decision.AllowRelease = len(decision.RejectReasons) == 0
	return decision
}

func handoffFatalRejectReasons(snapshot issueopscontract.IssueOpsHandoffSnapshot) []string {
	reasons := []string{}
	if snapshot.SchemaVersion != issueopscontract.IssueOpsHandoffSchemaVersion {
		reasons = append(reasons, "unsupported_schema")
	}
	for _, required := range []struct {
		name  string
		value string
	}{
		{"base_head", snapshot.Sealed.BaseHead},
		{"sealed_head", snapshot.Sealed.FullHead},
		{"sealed_plan_digest", snapshot.Sealed.PlanDigest},
		{"sealed_material_digest", snapshot.Sealed.MaterialDigest},
		{"current_head", snapshot.Current.FullHead},
		{"current_plan_digest", snapshot.Current.PlanDigest},
		{"current_material_digest", snapshot.Current.MaterialDigest},
		{"material_purpose", snapshot.Sealed.Material.Purpose},
		{"material_approved_endpoint", snapshot.Sealed.Material.ApprovedEndpoint},
		{"material_source_root", snapshot.Sealed.Material.SourceRoot},
		{"material_canonical_worktree", snapshot.Sealed.Material.CanonicalWorktree},
		{"material_plan_path", snapshot.Sealed.Material.PlanPath},
		{"material_diff_digest", snapshot.Sealed.Material.DiffDigest},
		{"material_lifecycle_state", snapshot.Sealed.Material.LifecycleState},
		{"sender_host", snapshot.Sender.Host},
		{"sender_session", snapshot.Sender.SessionID},
		{"sender_process", snapshot.Sender.ProcessReceipt},
		{"receiver_host", snapshot.Receiver.Host},
		{"receiver_session", snapshot.Receiver.SessionID},
		{"receiver_process", snapshot.Receiver.ProcessReceipt},
	} {
		if strings.TrimSpace(required.value) == "" {
			reasons = append(reasons, "missing_"+required.name)
		}
	}
	if len(snapshot.Sealed.Material.NonGoals) == 0 {
		reasons = append(reasons, "missing_material_non_goals")
	}
	if len(snapshot.Sealed.Material.ResumeCommands) == 0 {
		reasons = append(reasons, "missing_resume_commands")
	} else {
		for _, command := range snapshot.Sealed.Material.ResumeCommands {
			if strings.TrimSpace(command) == "" {
				reasons = append(reasons, "missing_resume_command")
				break
			}
		}
	}
	if len(snapshot.Sealed.Material.ReadOnlyCommands) == 0 {
		reasons = append(reasons, "missing_read_only_commands")
	} else {
		for _, command := range snapshot.Sealed.Material.ReadOnlyCommands {
			if strings.TrimSpace(command) == "" {
				reasons = append(reasons, "missing_read_only_command")
				break
			}
		}
	}
	if len(snapshot.Sealed.Material.Verification) == 0 {
		reasons = append(reasons, "missing_verification")
	}
	verificationIDs := map[string]bool{}
	for _, verification := range snapshot.Sealed.Material.Verification {
		verificationID := strings.TrimSpace(verification.ID)
		if verificationID == "" {
			reasons = append(reasons, "missing_verification_id")
		} else if verificationIDs[verificationID] {
			reasons = append(reasons, "duplicate_verification:"+verification.ID)
		}
		verificationIDs[verificationID] = true
		for _, required := range []struct {
			name  string
			value string
		}{
			{"input", verification.Input},
			{"command", verification.Command},
			{"timestamp", verification.Timestamp},
			{"environment", verification.Environment},
			{"result_location", verification.ResultLocation},
		} {
			if strings.TrimSpace(required.value) == "" {
				reasons = append(reasons, "missing_verification_"+required.name+":"+verification.ID)
			}
		}
	}
	reasons = append(reasons, handoffEvidenceRejectReasons("required", snapshot.Sealed.RequiredEvidence, snapshot.Sealed.FullHead, snapshot.Sealed.PlanDigest, snapshot.Sealed.MaterialDigest)...)
	if snapshot.Current.FullHead == snapshot.Sealed.FullHead &&
		snapshot.Current.PlanDigest == snapshot.Sealed.PlanDigest &&
		snapshot.Current.MaterialDigest == snapshot.Sealed.MaterialDigest {
		reasons = append(reasons, handoffEvidenceRejectReasons("current", snapshot.Current.Evidence, snapshot.Current.FullHead, snapshot.Current.PlanDigest, snapshot.Current.MaterialDigest)...)
	} else {
		reasons = append(reasons, handoffEvidenceIdentityRejectReasons("current", snapshot.Current.Evidence)...)
	}
	taskIDs := map[string]bool{}
	for _, task := range snapshot.Tasks {
		taskID := strings.TrimSpace(task.ID)
		if taskID == "" {
			reasons = append(reasons, "missing_task_id")
		} else if taskIDs[taskID] {
			reasons = append(reasons, "duplicate_task:"+task.ID)
		}
		taskIDs[taskID] = true
		for _, required := range []struct {
			name  string
			value string
		}{
			{"owner", task.Owner},
			{"execution_handle", task.ExecutionHandle},
			{"input_revision", task.InputRevision},
			{"result_location", task.ResultLocation},
		} {
			if strings.TrimSpace(required.value) == "" {
				reasons = append(reasons, "missing_task_"+required.name+":"+task.ID)
			}
		}
		if handoffTaskIsWriter(task) && strings.TrimSpace(task.WriteScope) == "" {
			reasons = append(reasons, "missing_task_write_scope:"+task.ID)
		}
	}
	lateResultIDs := map[string]bool{}
	for _, late := range snapshot.LateResults {
		lateID := strings.TrimSpace(late.ID)
		if lateID == "" {
			reasons = append(reasons, "missing_late_result_id")
		} else if lateResultIDs[lateID] {
			reasons = append(reasons, "duplicate_late_result:"+late.ID)
		}
		lateResultIDs[lateID] = true
	}
	if snapshot.UserDirective.Cancelled && snapshot.UserDirective.LatestVersion > snapshot.UserDirective.MaterialVersion {
		reasons = append(reasons, "user_cancelled_after_material")
	}
	if snapshot.UserDirective.ScopeChanged && snapshot.UserDirective.LatestVersion > snapshot.UserDirective.MaterialVersion {
		reasons = append(reasons, "user_scope_changed_after_material")
	}
	if snapshot.Sender.Host != snapshot.Receiver.Host && snapshot.Sender.SessionID == snapshot.Receiver.SessionID {
		reasons = append(reasons, "cross_host_session_id_not_portable")
	}
	if !snapshot.Execution.StatusValidated {
		reasons = append(reasons, "status_not_validated")
	}
	switch snapshot.Execution.Mode {
	case issueopscontract.ExecutionModeDirect:
	case issueopscontract.ExecutionModeOrca:
		if !snapshot.Execution.OrcaPacketPresent {
			reasons = append(reasons, "orca_packet_required")
		}
	default:
		reasons = append(reasons, "unsupported_execution_mode")
	}
	return reasons
}

func handoffEvidenceIdentityRejectReasons(scope string, evidence []issueopscontract.IssueOpsHandoffEvidence) []string {
	reasons := []string{}
	seen := map[string]bool{}
	for _, ref := range evidence {
		if strings.TrimSpace(ref.ID) == "" {
			reasons = append(reasons, "missing_"+scope+"_evidence_id")
			continue
		}
		if seen[ref.ID] {
			reasons = append(reasons, "duplicate_"+scope+"_evidence:"+ref.ID)
			continue
		}
		seen[ref.ID] = true
		if strings.TrimSpace(ref.Digest) == "" {
			reasons = append(reasons, "missing_"+scope+"_evidence_digest:"+ref.ID)
		}
		if strings.TrimSpace(ref.InputRevision) == "" {
			reasons = append(reasons, "missing_"+scope+"_evidence_input_revision:"+ref.ID)
		}
	}
	return reasons
}

func handoffEvidenceRejectReasons(scope string, evidence []issueopscontract.IssueOpsHandoffEvidence, head, planDigest, materialDigest string) []string {
	reasons := []string{}
	seen := map[string]bool{}
	for _, ref := range evidence {
		if strings.TrimSpace(ref.ID) == "" {
			reasons = append(reasons, "missing_"+scope+"_evidence_id")
			continue
		}
		if seen[ref.ID] {
			reasons = append(reasons, "duplicate_"+scope+"_evidence:"+ref.ID)
			continue
		}
		seen[ref.ID] = true
		if strings.TrimSpace(ref.Digest) == "" {
			reasons = append(reasons, "missing_"+scope+"_evidence_digest:"+ref.ID)
		}
		if strings.TrimSpace(ref.Head) == "" {
			reasons = append(reasons, "missing_"+scope+"_evidence_head:"+ref.ID)
		} else if ref.Head != head {
			reasons = append(reasons, "stale_"+scope+"_evidence_head:"+ref.ID)
		}
		if strings.TrimSpace(ref.PlanDigest) == "" {
			reasons = append(reasons, "missing_"+scope+"_evidence_plan:"+ref.ID)
		} else if ref.PlanDigest != planDigest {
			reasons = append(reasons, "stale_"+scope+"_evidence_plan:"+ref.ID)
		}
		if strings.TrimSpace(ref.MaterialDigest) == "" {
			reasons = append(reasons, "missing_"+scope+"_evidence_material:"+ref.ID)
		} else if ref.MaterialDigest != materialDigest {
			reasons = append(reasons, "stale_"+scope+"_evidence_material:"+ref.ID)
		}
		if strings.TrimSpace(ref.InputRevision) == "" {
			reasons = append(reasons, "missing_"+scope+"_evidence_input_revision:"+ref.ID)
		}
	}
	return reasons
}

func handoffTaskIsWriter(task issueopscontract.IssueOpsHandoffTask) bool {
	if task.Classification == issueopscontract.IssueOpsHandoffTaskWriter {
		return true
	}
	switch task.Kind {
	case issueopscontract.IssueOpsHandoffTaskKindBuild, issueopscontract.IssueOpsHandoffTaskKindGolden, issueopscontract.IssueOpsHandoffTaskKindGenerator, issueopscontract.IssueOpsHandoffTaskKindFormatter, issueopscontract.IssueOpsHandoffTaskKindFixture, issueopscontract.IssueOpsHandoffTaskKindUnknown:
		return true
	default:
		return task.Classification != issueopscontract.IssueOpsHandoffTaskReader
	}
}

func handoffEvidenceMap(refs []issueopscontract.IssueOpsHandoffEvidence) map[string]issueopscontract.IssueOpsHandoffEvidence {
	out := make(map[string]issueopscontract.IssueOpsHandoffEvidence, len(refs))
	for _, ref := range refs {
		if strings.TrimSpace(ref.ID) != "" {
			out[ref.ID] = ref
		}
	}
	return out
}

func appendIfMissing(values []string, value string) []string {
	for _, current := range values {
		if current == value {
			return values
		}
	}
	return append(values, value)
}

func containsAnyReason(reasons []string, candidates ...string) bool {
	for _, reason := range reasons {
		for _, candidate := range candidates {
			if reason == candidate {
				return true
			}
		}
	}
	return false
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	for _, value := range values {
		if len(out) == 0 || out[len(out)-1] != value {
			out = append(out, value)
		}
	}
	return out
}
