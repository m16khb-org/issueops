package doctor

import (
	"fmt"
	"strings"
	"time"

	contract "issueops/internal/contract/doctor"
)

const PipeCapacityWarningThreshold = 8192
const MCPGatewayFDWarningThreshold = 512

func Evaluate(observation Observations) Findings {
	result := Findings{}
	evaluateProjectDocs(&result, observation.Root, observation.ProjectDocs)
	evaluateRuntimeState(&result, observation.RuntimeState)
	evaluateLoop(&result, observation.Loop)
	if observation.Pipe != nil {
		evaluatePipe(&result, *observation.Pipe)
	}
	if observation.Gateways != nil {
		evaluateGateways(&result, *observation.Gateways)
	}
	evaluateNative(&result, observation.Native)
	evaluateBinary(&result, observation.Binary)
	return result
}

func Healthy(checks []contract.HarnessDoctorCheck, issues []contract.HarnessDoctorIssue) bool {
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

func evaluateProjectDocs(result *Findings, root string, observation ProjectDocsObservation) {
	if len(observation.Missing) == 0 {
		result.check("project_docs", true, "all standard .issueops docs exist")
		return
	}
	result.check("project_docs", false, strings.Join(observation.Missing, ", "))
	result.issue("project_docs_missing", "warning", "standard .issueops docs are missing", observation.Directory, &contract.HarnessDoctorFix{Command: "issueops project bootstrap --repo " + shellQuote(root), Description: "Create or refresh the standard project guidance docs and profile metadata."})
}

func evaluateRuntimeState(result *Findings, observation RuntimeStateObservation) {
	for _, path := range observation.Paths {
		result.issue("repo_local_state_present", "warning", "repo-local lifecycle runtime or schema state should not be committed in team repositories", path, &contract.HarnessDoctorFix{Description: "Move runtime state to the user-state project namespace and ensure repo-local state paths are ignored or removed."})
	}
	lower := strings.ToLower(observation.Document)
	if strings.Contains(lower, "schema") || strings.Contains(lower, "runtime state") || strings.Contains(lower, "jsonl") {
		result.issue("repo_local_state_present", "warning", "STATE.md appears to describe runtime/schema state rather than shared project knowledge", observation.DocumentPath, &contract.HarnessDoctorFix{Description: "Keep lifecycle schemas in issueops core and runtime state in user-state, not target repo docs."})
	}
}

func evaluateLoop(result *Findings, observation LoopObservation) {
	incomplete := observation.Active + observation.Exhausted
	result.check("loop_contracts", incomplete == 0 && len(observation.Warnings) == 0, fmt.Sprintf("active=%d exhausted=%d", observation.Active, observation.Exhausted))
	if len(observation.Warnings) > 0 {
		result.issue("loop_contracts_unreadable", "warning", strings.Join(observation.Warnings, "; "), observation.StateRoot, &contract.HarnessDoctorFix{Command: "issueops loop status --id <loop-id> --json", Description: "Inspect loop state records before PR readiness."})
		return
	}
	if incomplete > 0 {
		result.issue("loop_contracts_incomplete", "warning", fmt.Sprintf("repo has incomplete loop contracts: active=%d exhausted=%d", observation.Active, observation.Exhausted), observation.StateRoot, &contract.HarnessDoctorFix{Command: "issueops loop status --id <loop-id> --json", Description: "Stop or complete same-repo loop runs before PR readiness."})
	}
}

func evaluatePipe(result *Findings, observation PipeObservation) {
	if observation.Error != nil {
		result.check("pipe_capacity", true, "pipe capacity unavailable: "+observation.Error.Error())
		result.issue("pipe_capacity_unavailable", "warning", "system pipe buffer capacity could not be measured", "", &contract.HarnessDoctorFix{Description: "Retry doctor; if this persists, inspect process file descriptors and OS pipe limits."})
		return
	}
	result.PipeCapacityBytes = observation.Capacity
	summary := fmt.Sprintf("capacity=%d bytes", observation.Capacity)
	if observation.Capacity < PipeCapacityWarningThreshold {
		result.check("pipe_capacity", false, summary)
		result.issue("pipe_capacity_degraded", "warning", "system pipe buffer degraded; long-lived host process may be leaking pipes; see CAUTIONS 2026-07-09", "", &contract.HarnessDoctorFix{Description: "Restart the leaking long-lived host process, then rerun lsof pipe counts and issueops doctor."})
		return
	}
	result.check("pipe_capacity", true, summary)
}

func evaluateGateways(result *Findings, observation GatewayObservation) {
	if observation.Home == "" {
		result.check("mcp_gateway", true, "home directory unavailable; skipped loopback MCP gateway checks")
		return
	}
	if observation.ConfigError != nil {
		result.check("mcp_gateway", true, "claude MCP config unreadable: "+observation.ConfigError.Error())
		return
	}
	if len(observation.Endpoints) == 0 {
		result.check("mcp_gateway", true, "no loopback HTTP MCP servers configured; skipped")
		return
	}
	unreachable := 0
	for _, endpoint := range observation.Endpoints {
		if endpoint.Error != nil {
			unreachable++
			result.issue("mcp_gateway_unreachable", "warning", fmt.Sprintf("loopback MCP server %q did not answer an initialize probe: %v", endpoint.Name, endpoint.Error), endpoint.URL, &contract.HarnessDoctorFix{Description: "Restart the local MCP gateway process serving this URL, then rerun doctor; see CAUTIONS 2026-07-10."})
		}
	}
	pressure := false
	summaries := []string{}
	for _, fd := range observation.FDs {
		if !fd.Available {
			summaries = append(summaries, fmt.Sprintf("fd[:%d]=unavailable", fd.Port))
			continue
		}
		summaries = append(summaries, fmt.Sprintf("fd[:%d]=%d", fd.Port, fd.Count))
		if fd.Count >= MCPGatewayFDWarningThreshold {
			pressure = true
			result.issue("mcp_gateway_fd_pressure", "warning", fmt.Sprintf("loopback MCP gateway on port %d holds %d open file descriptors; sessions or sockets may be accumulating toward fd exhaustion", fd.Port, fd.Count), fmt.Sprintf("127.0.0.1:%d", fd.Port), &contract.HarnessDoctorFix{Description: "Restart the gateway before it hits its fd limit and check that its HTTP transport runs stateless; see CAUTIONS 2026-07-10."})
		}
	}
	result.check("mcp_gateway", unreachable == 0 && !pressure, fmt.Sprintf("endpoints=%d unreachable=%d %s", len(observation.Endpoints), unreachable, strings.Join(summaries, " ")))
}

func evaluateNative(result *Findings, observation NativeObservation) {
	if observation.Home == "" {
		result.check("native_integrations", true, "home directory unavailable; skipped user-level integration checks")
		return
	}
	if observation.HooksMissing {
		result.issue("codex_hooks_missing", "warning", "Codex hooks.json is not present", observation.HooksPath, &contract.HarnessDoctorFix{Command: "issueops install", Description: "Install user-level hooks, skills, and MCP configuration."})
	}
	result.check("native_integrations", true, "checked user-level integration paths")
}

func evaluateBinary(result *Findings, observation BinaryObservation) {
	if observation.Root == "" {
		return
	}
	if !observation.Found {
		result.check("binary_drift", true, "no prebuilt bin/issueops found; skipping drift check")
		return
	}
	if observation.LatestSource.After(observation.BuiltAt) {
		delta := observation.LatestSource.Sub(observation.BuiltAt).Round(time.Second)
		result.check("binary_drift", false, fmt.Sprintf("bin/issueops is %s older than latest source change", delta))
		result.issue("binary_drift", "warning", fmt.Sprintf("bin/issueops may be stale (%s older than source)", delta), observation.Path, &contract.HarnessDoctorFix{Command: "go build -o bin/issueops ./cmd/issueops", Description: "Rebuild the issueops binary from the current source."})
	} else {
		result.check("binary_drift", true, "bin/issueops is current")
	}
}

func EvaluateLifecycle(root string, plan contract.ProjectLifecycleStatePlan, failed bool) Findings {
	result := Findings{}
	if failed {
		result.issue("lifecycle_state_error", "error", "project lifecycle state could not be resolved", root, &contract.HarnessDoctorFix{Command: "issueops project bootstrap --repo " + shellQuote(root), Description: "Initialize project lifecycle state and repo metadata through project bootstrap."})
		return result
	}
	result.check("project_lifecycle_state", plan.Exists && plan.NamespaceValid, plan.ProjectStateDir)
	if !plan.Exists {
		result.issue("lifecycle_state_missing", "warning", "project lifecycle namespace has not been initialized", plan.ProjectStateDir, &contract.HarnessDoctorFix{Command: "issueops project bootstrap --repo " + shellQuote(root), Description: "Create the repo-scoped lifecycle namespace and profile metadata in user-state."})
	} else if !plan.NamespaceValid {
		result.issue("lifecycle_namespace_mismatch", "error", "project lifecycle state fingerprint does not match this repo", plan.ProjectJSONPath, &contract.HarnessDoctorFix{Command: "issueops doctor --repo " + shellQuote(root) + " --json", Description: "Review the namespace mismatch before migrating or deleting stale state."})
	}
	return result
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func IsUnexpectedStateArtifact(code string) bool {
	return code == "unexpected_file" || code == "unexpected_directory"
}
