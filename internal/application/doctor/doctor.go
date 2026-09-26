package doctor

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"issueops/internal/domain/operationalhealth"
)

func (service Service) Run(req HarnessDoctorRequest) (HarnessDoctorResult, error) {
	root, err := service.Effects.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return HarnessDoctorResult{OK: false, Kind: "harness_doctor", StateDir: service.Effects.StateDir()}, err
	}
	result := HarnessDoctorResult{
		OK:                true,
		Healthy:           true,
		Kind:              "harness_doctor",
		Version:           req.Version,
		IssueOpsRoot:      req.IssueOpsRoot,
		RepoRoot:          root,
		StateDir:          service.Effects.StateDir(),
		ActiveConnections: req.DaemonAdmission.ActiveConnections,
		MaxConnections:    req.DaemonAdmission.MaxConnections,
		Accepting:         req.DaemonAdmission.Accepting,
		Draining:          req.DaemonAdmission.Draining,
		Checks:            []HarnessDoctorCheck{},
		Issues:            []HarnessDoctorIssue{},
		GeneratedAt:       service.Effects.Now().UTC().Format(time.RFC3339Nano),
	}
	AddCheck(&result, "binary", true, "issueops command is running")
	CheckDaemonAdmission(&result, req.DaemonAdmission)

	stateDoctor, stateDoctorErr := service.Effects.StateDoctor()
	if stateDoctorErr != nil {
		AddIssue(&result, "state_doctor_error", "error", "state doctor could not inspect the user-state store", service.Effects.StateDir(), &HarnessDoctorFix{Description: "Check user-state directory permissions or set ISSUEOPS_STATE_DIR to a writable location."})
	} else {
		AddCheck(&result, "state_store", stateDoctor.Healthy, stateDoctor.StateDir)
		for _, issue := range stateDoctor.Issues {
			if req.OperationalSnapshot != nil && isUnexpectedStateArtifact(issue.Code) {
				continue
			}
			AddIssue(&result, "state_"+issue.Code, issue.Severity, issue.Message, issue.Path, &HarnessDoctorFix{Command: "issueops state doctor --json", Description: "Inspect state-store integrity details with the narrow state doctor."})
		}
	}
	if req.OperationalSnapshot != nil {
		snapshot := cloneOperationalSnapshot(*req.OperationalSnapshot)
		if stateDoctorErr == nil {
			for _, issue := range stateDoctor.Issues {
				if isUnexpectedStateArtifact(issue.Code) {
					snapshot.StateArtifacts = append(snapshot.StateArtifacts, operationalhealth.StateArtifact{Path: issue.Path, Code: issue.Code})
				}
			}
		}
		op := operationalhealth.Classify(snapshot, req.OperationalOptions)
		AddCheck(&result, "operational_state", op.Healthy, fmt.Sprintf("findings=%d", len(op.Findings)))
		for _, finding := range op.Findings {
			severity := operationalhealth.SeverityForFinding(finding.Code)
			summary := finding.Summary
			if resourceID := strings.TrimSpace(finding.ResourceID); resourceID != "" {
				resourceKind := strings.TrimSpace(finding.ResourceKind)
				if resourceKind == "" {
					resourceKind = "resource"
				}
				summary = fmt.Sprintf("%s %s: %s", resourceKind, resourceID, summary)
			}
			AddIssue(&result, finding.Code, severity, summary, finding.Path, &HarnessDoctorFix{Description: "Inspect exact operational identities and reconcile this finding before continuing."})
		}
	}

	lifecycle, err := service.Effects.ValidateLifecycle(root)
	if err != nil {
		AddIssue(&result, "lifecycle_state_error", "error", "project lifecycle state could not be resolved", root, &HarnessDoctorFix{Command: "issueops project bootstrap --repo " + shellQuote(root), Description: "Initialize project lifecycle state and repo metadata through project bootstrap."})
	} else {
		result.LifecycleState = lifecycle
		AddCheck(&result, "project_lifecycle_state", lifecycle.Exists && lifecycle.NamespaceValid, lifecycle.ProjectStateDir)
		if !lifecycle.Exists {
			AddIssue(&result, "lifecycle_state_missing", "warning", "project lifecycle namespace has not been initialized", lifecycle.ProjectStateDir, &HarnessDoctorFix{Command: "issueops project bootstrap --repo " + shellQuote(root), Description: "Create the repo-scoped lifecycle namespace and profile metadata in user-state."})
		} else if !lifecycle.NamespaceValid {
			AddIssue(&result, "lifecycle_namespace_mismatch", "error", "project lifecycle state fingerprint does not match this repo", lifecycle.ProjectJSONPath, &HarnessDoctorFix{Command: "issueops doctor --repo " + shellQuote(root) + " --json", Description: "Review the namespace mismatch before migrating or deleting stale state."})
		}
	}

	service.Effects.CheckProjectDocs(&result, root)
	service.Effects.CheckRepoLocalRuntimeState(&result, root)
	service.Effects.CheckLoopContracts(&result, root)
	if !req.StaticOnly {
		service.Effects.CheckPipeCapacity(&result)
		service.Effects.CheckMCPGateways(&result, req.Home)
	}
	service.Effects.CheckNativeIntegrations(&result, req.Home)
	service.Effects.CheckBinaryDrift(&result, req.IssueOpsRoot)

	sort.Slice(result.Issues, func(i, j int) bool {
		if result.Issues[i].Severity != result.Issues[j].Severity {
			return severityRank(result.Issues[i].Severity) < severityRank(result.Issues[j].Severity)
		}
		if result.Issues[i].Code != result.Issues[j].Code {
			return result.Issues[i].Code < result.Issues[j].Code
		}
		return result.Issues[i].Path < result.Issues[j].Path
	})
	result.Healthy = DoctorHealthy(result.Checks, result.Issues)
	return result, nil
}

func DoctorHealthy(checks []HarnessDoctorCheck, issues []HarnessDoctorIssue) bool {
	for _, check := range checks {
		if !check.Healthy {
			return false
		}
	}
	for _, issue := range issues {
		if issue.Severity == "error" || issue.Severity == "warning" {
			return false
		}
	}
	return true
}

func CheckDaemonAdmission(r *HarnessDoctorResult, admission HarnessDoctorDaemonAdmission) {
	decision := operationalhealth.DecideAdmission(operationalhealth.Admission{
		Observed: admission.Observed, Active: admission.ActiveConnections, Maximum: admission.MaxConnections,
		Accepting: admission.Accepting, Draining: admission.Draining,
	})
	if !decision.Evaluated {
		AddCheck(r, "daemon_admission", true, "not evaluated")
		return
	}
	summary := fmt.Sprintf(
		"active_connections=%d max_connections=%d accepting=%t draining=%t",
		admission.ActiveConnections,
		admission.MaxConnections,
		admission.Accepting,
		admission.Draining,
	)
	AddCheck(r, "daemon_admission", decision.Healthy, summary)
	switch decision.IssueCode {
	case "daemon_admission_inconsistent":
		AddIssue(r, decision.IssueCode, "warning", "daemon admission telemetry is internally inconsistent", "", nil)
	case "daemon_connection_limit_reached":
		AddIssue(r, decision.IssueCode, "warning", "daemon is not accepting new MCP connections because its connection limit is exhausted", "", nil)
	}
}

func AddCheck(r *HarnessDoctorResult, name string, healthy bool, summary string) {
	r.Checks = append(r.Checks, HarnessDoctorCheck{Name: name, Healthy: healthy, Summary: summary})
}

func AddIssue(r *HarnessDoctorResult, code, severity, summary, path string, fix *HarnessDoctorFix) {
	r.Issues = append(r.Issues, HarnessDoctorIssue{Code: code, Severity: severity, Summary: summary, Path: path, Fix: fix})
}

func isUnexpectedStateArtifact(code string) bool {
	return code == "unexpected_file" || code == "unexpected_directory"
}

func cloneOperationalSnapshot(snapshot operationalhealth.Snapshot) operationalhealth.Snapshot {
	snapshot.Cycles = append([]operationalhealth.Cycle(nil), snapshot.Cycles...)
	snapshot.GitWorktrees = append([]operationalhealth.GitWorktree(nil), snapshot.GitWorktrees...)
	snapshot.LocalRefs = append([]operationalhealth.GitRef(nil), snapshot.LocalRefs...)
	snapshot.RemoteRefs = append([]operationalhealth.GitRef(nil), snapshot.RemoteRefs...)
	snapshot.OrcaWorktrees = append([]operationalhealth.OrcaWorktree(nil), snapshot.OrcaWorktrees...)
	snapshot.Terminals = append([]operationalhealth.OrcaTerminal(nil), snapshot.Terminals...)
	snapshot.Tasks = append([]operationalhealth.OrcaTask(nil), snapshot.Tasks...)
	snapshot.Dispatches = append([]operationalhealth.OrcaDispatch(nil), snapshot.Dispatches...)
	snapshot.Gates = append([]operationalhealth.OrcaGate(nil), snapshot.Gates...)
	snapshot.StateArtifacts = append([]operationalhealth.StateArtifact(nil), snapshot.StateArtifacts...)
	snapshot.InventoryProblems = append([]operationalhealth.InventoryProblem(nil), snapshot.InventoryProblems...)
	return snapshot
}
