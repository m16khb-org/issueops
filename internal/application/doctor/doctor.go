package doctor

import (
	"fmt"
	doctorcontract "issueops/internal/contract/doctor"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
	statecontract "issueops/internal/contract/state"
	"sort"
	"strings"
	"time"

	doctordomain "issueops/internal/domain/doctor"
	"issueops/internal/domain/operationalhealth"
)

func (service Service) Run(req doctorcontract.HarnessDoctorRequest) (doctorcontract.HarnessDoctorResult, error) {
	root, err := service.Effects.NormalizeRoot(req.RepoRoot)
	if err != nil {
		return doctorcontract.HarnessDoctorResult{OK: false, Kind: "harness_doctor", StateDir: service.Effects.StateDir()}, err
	}
	result := doctorcontract.HarnessDoctorResult{
		OK:           true,
		Healthy:      true,
		Kind:         "harness_doctor",
		Version:      req.Version,
		IssueOpsRoot: req.IssueOpsRoot,
		RepoRoot:     root,
		StateDir:     service.Effects.StateDir(),
		Checks:       []doctorcontract.HarnessDoctorCheck{},
		Issues:       []doctorcontract.HarnessDoctorIssue{},
		GeneratedAt:  service.Effects.Now().UTC().Format(time.RFC3339Nano),
	}
	AddCheck(&result, "binary", true, "issueops command is running")

	stateDoctor, stateDoctorErr := service.Effects.StateDoctor()
	service.addStateStoreFindings(&result, req, stateDoctor, stateDoctorErr)
	if req.OperationalSnapshot != nil {
		addOperationalFindings(&result, req, stateDoctor, stateDoctorErr)
	}

	lifecycle, err := service.Effects.ValidateLifecycle(root)
	if err == nil {
		result.LifecycleState = lifecycle
	}
	lifecycleFindings := doctordomain.EvaluateLifecycle(root, lifecycle, err != nil)
	result.Checks = append(result.Checks, lifecycleFindings.Checks...)
	result.Issues = append(result.Issues, lifecycleFindings.Issues...)
	findings := doctordomain.Evaluate(service.observe(req, root))
	result.Checks = append(result.Checks, findings.Checks...)
	result.Issues = append(result.Issues, findings.Issues...)
	result.PipeCapacityBytes = findings.PipeCapacityBytes

	sortIssues(result.Issues)
	result.Healthy = doctordomain.Healthy(result.Checks, result.Issues)
	return result, nil
}

func (service Service) observe(req doctorcontract.HarnessDoctorRequest, root string) doctordomain.Observations {
	observations := doctordomain.Observations{Root: root}
	observations.ProjectDocs = service.Effects.ProjectDocs(root)
	observations.RuntimeState = service.Effects.RuntimeState(root)
	observations.Loop = service.Effects.LoopContracts(root)
	if !req.StaticOnly {
		capacity, err := service.Effects.PipeCapacity()
		observations.Pipe = &doctordomain.PipeObservation{Capacity: capacity, Error: err}
		gateways := service.Effects.MCPGateways(req.Home)
		observations.Gateways = &gateways
	}
	observations.Native = service.Effects.NativeIntegrations(req.Home)
	observations.Binary = service.Effects.BinaryDrift(req.IssueOpsRoot)
	return observations
}

func sortIssues(issues []doctorcontract.HarnessDoctorIssue) {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Severity != issues[j].Severity {
			return severityRank(issues[i].Severity) < severityRank(issues[j].Severity)
		}
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		return issues[i].Path < issues[j].Path
	})
}

func (service Service) addStateStoreFindings(result *doctorcontract.HarnessDoctorResult, req doctorcontract.HarnessDoctorRequest, stateDoctor statecontract.StateDoctorResult, stateDoctorErr error) {
	if stateDoctorErr != nil {
		AddIssue(result, "state_doctor_error", "error", "state doctor could not inspect the user-state store", service.Effects.StateDir(), &doctorcontract.HarnessDoctorFix{Description: "Check user-state directory permissions or set ISSUEOPS_STATE_DIR to a writable location."})
		return
	}
	AddCheck(result, "state_store", stateDoctor.Healthy, stateDoctor.StateDir)
	for _, issue := range stateDoctor.Issues {
		if req.OperationalSnapshot != nil && doctordomain.IsUnexpectedStateArtifact(issue.Code) {
			continue
		}
		AddIssue(result, "state_"+issue.Code, issue.Severity, issue.Message, issue.Path, &doctorcontract.HarnessDoctorFix{Command: "issueops state doctor --json", Description: "Inspect state-store integrity details with the narrow state doctor."})
	}
}

func addOperationalFindings(result *doctorcontract.HarnessDoctorResult, req doctorcontract.HarnessDoctorRequest, stateDoctor statecontract.StateDoctorResult, stateDoctorErr error) {
	snapshot := cloneOperationalSnapshot(*req.OperationalSnapshot)
	if stateDoctorErr == nil {
		for _, issue := range stateDoctor.Issues {
			if doctordomain.IsUnexpectedStateArtifact(issue.Code) {
				snapshot.StateArtifacts = append(snapshot.StateArtifacts, operationalhealthcontract.StateArtifact{Path: issue.Path, Code: issue.Code})
			}
		}
	}
	op := operationalhealth.Classify(snapshot, req.OperationalOptions)
	AddCheck(result, "operational_state", op.Healthy, fmt.Sprintf("findings=%d", len(op.Findings)))
	for _, finding := range op.Findings {
		severity := operationalhealth.SeverityForFinding(finding.Code)
		AddIssue(result, finding.Code, severity, operationalFindingSummary(finding), finding.Path, &doctorcontract.HarnessDoctorFix{Description: "Inspect exact operational identities and reconcile this finding before continuing."})
	}
}

func operationalFindingSummary(finding operationalhealthcontract.Finding) string {
	resourceID := strings.TrimSpace(finding.ResourceID)
	if resourceID == "" {
		return finding.Summary
	}
	resourceKind := strings.TrimSpace(finding.ResourceKind)
	if resourceKind == "" {
		resourceKind = "resource"
	}
	return fmt.Sprintf("%s %s: %s", resourceKind, resourceID, finding.Summary)
}

func AddCheck(r *doctorcontract.HarnessDoctorResult, name string, healthy bool, summary string) {
	r.Checks = append(r.Checks, doctorcontract.HarnessDoctorCheck{Name: name, Healthy: healthy, Summary: summary})
}

func AddIssue(r *doctorcontract.HarnessDoctorResult, code, severity, summary, path string, fix *doctorcontract.HarnessDoctorFix) {
	r.Issues = append(r.Issues, doctorcontract.HarnessDoctorIssue{Code: code, Severity: severity, Summary: summary, Path: path, Fix: fix})
}

func cloneOperationalSnapshot(snapshot operationalhealth.Snapshot) operationalhealth.Snapshot {
	snapshot.Cycles = append([]operationalhealthcontract.Cycle(nil), snapshot.Cycles...)
	snapshot.GitWorktrees = append([]operationalhealthcontract.GitWorktree(nil), snapshot.GitWorktrees...)
	snapshot.LocalRefs = append([]operationalhealthcontract.GitRef(nil), snapshot.LocalRefs...)
	snapshot.RemoteRefs = append([]operationalhealthcontract.GitRef(nil), snapshot.RemoteRefs...)
	snapshot.StateArtifacts = append([]operationalhealthcontract.StateArtifact(nil), snapshot.StateArtifacts...)
	snapshot.InventoryProblems = append([]operationalhealthcontract.InventoryProblem(nil), snapshot.InventoryProblems...)
	return snapshot
}
