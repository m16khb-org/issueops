package issueops

import (
	"fmt"
	"issueops/internal/contract/issueops"
	"regexp"
	"strconv"
	"strings"
)

var (
	executionPromptPlaceholder = regexp.MustCompile(`\{[A-Z][A-Z0-9_]*\}`)
	ownerAcceptanceID          = regexp.MustCompile(`\bAC-[0-9]{2,}\b`)
)

func OwnerAcceptanceIDs(body string) []string {
	return uniqueExecutionOwnerValues(ownerAcceptanceID.FindAllString(body, -1))
}
func OwnerIssueProvider(record issueops.IssueOpsRecord) string {
	if record.BranchPrepare != nil {
		provider := strings.ToLower(strings.TrimSpace(record.BranchPrepare.Provider))
		if provider == "github" || provider == "gitlab" {
			return provider
		}
	}
	if strings.Contains(record.IssueURL, "github") {
		return "github"
	}
	if strings.Contains(record.IssueURL, "gitlab") {
		return "gitlab"
	}
	return ""
}

func RenderOwnerPrompt(packet issueops.OwnerContextPacket, packetPath, packetDigest, template string, maxBytes int) (string, error) {
	if err := validateExecutionOwnerPromptInputs(packet, packetPath, packetDigest); err != nil {
		return "", err
	}
	values := map[string]string{
		"LIFECYCLE_ID": packet.LifecycleID, "MODE": string(packet.Mode), "SCHEMA_VERSION": strconv.Itoa(packet.SchemaVersion),
		"SOURCE_ROOT": packet.SourceRoot, "WORKTREE_ROOT": packet.WorktreeRoot, "WORKTREE_BASE": packet.WorktreeBase,
		"BRANCH": packet.Branch, "BASE_HEAD": packet.BaseHead, "LEASE_GENERATION": strconv.FormatUint(packet.LeaseGeneration, 10),
		"LEASE_STATUS_COMMAND": packet.Commands.LeaseStatus, "CLAIM_COMMAND": strings.ReplaceAll(packet.Commands.Claim, "<PACKET_SHA256>", packetDigest),
		"ISSUE_URL": packet.Issue.URL, "ISSUE_BODY_SHA256": packet.Issue.BodySHA256,
		"PACKET_PATH": packetPath, "PACKET_SHA256": packetDigest,
		"OWNER_HOST": packet.OwnerHost, "OWNER_MODEL": packet.OwnerModel, "OWNER_EFFORT": packet.OwnerEffort,
		"REVIEWER_MODEL": packet.ReviewerModel, "REVIEWER_EFFORT": packet.ReviewerEffort,
		"RESEARCH_MODEL": packet.ResearchModel, "RESEARCH_EFFORT": packet.ResearchEffort,
		"AWAIT_BRANCH_LINK_COMMAND":       packet.Commands.AwaitBranchLink,
		"RELEASE_COMMAND":                 packet.Commands.Release,
		"VERIFY_BRANCH_LINK_READ_COMMAND": packet.Commands.VerifyBranchLinkRead,
		"VERIFY_BRANCH_LINK_COMMAND":      packet.Commands.VerifyBranchLink,
		"LINK_PLAN_COMMAND":               packet.Commands.LinkPlan,
		"COMPATIBILITY_REVIEW_COMMAND":    packet.Commands.CompatibilityReview,
		"ENTER_IMPLEMENT_COMMAND":         packet.Commands.EnterImplement,
		"AI_SLOP_CLEAN_RECORD_COMMAND":    packet.Commands.AISlopCleanRecord,
		"ENTER_AI_SLOP_CLEAN_COMMAND":     packet.Commands.EnterAISlopClean,
		"IMPLEMENTATION_REVIEW_COMMAND":   packet.Commands.ImplementationReview,
		"PROJECT_DOCS_REVIEW_COMMAND":     packet.Commands.ProjectDocsReview,
		"SCHEMA_EVIDENCE_COMMAND":         packet.Commands.SchemaEvidence,
		"ENTER_PR_COMMAND":                packet.Commands.EnterPR,
		"REQUIRED_DOCS":                   renderExecutionOwnerLines(packet.RequiredDocs), "REQUIRED_SKILLS": renderExecutionOwnerLines(packet.RequiredSkills),
		"ACCEPTANCE_IDS": strings.Join(packet.AcceptanceIDs, ", "), "VERIFICATION_COMMANDS": renderExecutionOwnerLines(packet.Verification),
		"TURING_REPORT_PATH": packet.VerificationReportPath, "REMOTE_CREATE_COMMAND": packet.Commands.RemoteCreate, "COMPLETE_COMMAND": packet.Commands.Complete,
	}
	missing := ""
	prompt := executionPromptPlaceholder.ReplaceAllStringFunc(template, func(token string) string {
		key := strings.TrimSuffix(strings.TrimPrefix(token, "{"), "}")
		value, ok := values[key]
		if !ok && missing == "" {
			missing = token
		}
		return value
	})
	if missing != "" {
		return "", fmt.Errorf("owner prompt placeholder %s has no renderer", missing)
	}
	if unresolved := executionPromptPlaceholder.FindString(prompt); unresolved != "" {
		return "", fmt.Errorf("owner prompt value introduced unresolved placeholder %s", unresolved)
	}
	if len(prompt) > maxBytes {
		return "", fmt.Errorf("owner prompt exceeds %d bytes", maxBytes)
	}
	return prompt, nil
}

func validateExecutionOwnerPromptInputs(packet issueops.OwnerContextPacket, packetPath, packetDigest string) error {
	scalars := []struct{ name, value string }{
		{"lifecycle_id", packet.LifecycleID}, {"mode", string(packet.Mode)}, {"source_root", packet.SourceRoot},
		{"worktree_root", packet.WorktreeRoot}, {"worktree_base", packet.WorktreeBase}, {"branch", packet.Branch},
		{"base_head", packet.BaseHead}, {"issue_url", packet.Issue.URL}, {"issue_body_sha256", packet.Issue.BodySHA256},
		{"packet_path", packetPath}, {"packet_sha256", packetDigest}, {"owner_host", packet.OwnerHost},
		{"owner_model", packet.OwnerModel}, {"owner_effort", packet.OwnerEffort}, {"verification_report_path", packet.VerificationReportPath},
		{"lease_status_command", packet.Commands.LeaseStatus}, {"claim_command", packet.Commands.Claim},
		{"verify_branch_link_read_command", packet.Commands.VerifyBranchLinkRead},
		{"verify_branch_link_command", packet.Commands.VerifyBranchLink},
		{"link_plan_command", packet.Commands.LinkPlan}, {"compatibility_review_command", packet.Commands.CompatibilityReview},
		{"enter_implement_command", packet.Commands.EnterImplement},
		{"ai_slop_clean_record_command", packet.Commands.AISlopCleanRecord}, {"enter_ai_slop_clean_command", packet.Commands.EnterAISlopClean},
		{"remote_create_command", packet.Commands.RemoteCreate}, {"complete_command", packet.Commands.Complete},
		{"reviewer_model", packet.ReviewerModel}, {"reviewer_effort", packet.ReviewerEffort},
		{"research_model", packet.ResearchModel}, {"research_effort", packet.ResearchEffort},
		{"implementation_review_command", packet.Commands.ImplementationReview},
		{"project_docs_review_command", packet.Commands.ProjectDocsReview},
		{"schema_evidence_command", packet.Commands.SchemaEvidence},
		{"enter_pr_command", packet.Commands.EnterPR},
	}
	for _, scalar := range scalars {
		if strings.ContainsAny(scalar.value, "\r\n") || executionPromptPlaceholder.MatchString(scalar.value) {
			return fmt.Errorf("owner prompt %s contains a line break or placeholder token", scalar.name)
		}
	}
	lists := []struct {
		name   string
		values []string
	}{
		{"required_docs", packet.RequiredDocs}, {"required_skills", packet.RequiredSkills},
		{"acceptance_ids", packet.AcceptanceIDs}, {"verification_commands", packet.Verification},
	}
	for _, list := range lists {
		for _, value := range list.values {
			if strings.TrimSpace(value) == "" || strings.ContainsAny(value, "\r\n") || executionPromptPlaceholder.MatchString(value) {
				return fmt.Errorf("owner prompt %s contains an empty, multiline, or placeholder value", list.name)
			}
		}
	}
	return nil
}

func OwnerCommandsFor(record issueops.IssueOpsRecord, req issueops.ExecutionPrepareRequest, issueBodySHA256 string, paths issueops.OwnerPaths, policy issueops.OwnerPolicyContext) issueops.OwnerCommands {
	generation := record.Execution.Lease.Generation
	actorFlags := strings.Join([]string{
		"--host", strings.ToLower(strings.TrimSpace(req.OwnerHost)), "--session-id", "<SESSION_ID>",
		"--session-pid", "<SESSION_PID>", "--session-started-at", "<SESSION_STARTED_AT>", "--session-executable", "<SESSION_EXECUTABLE>",
		"--cwd", quoteReplacementArg(record.Execution.Workspace.Root),
	}, " ")
	status := "issueops execution status --id " + quoteReplacementArg(record.ID) + " --json"
	claim := "none"
	if record.Execution.Mode == issueops.ExecutionModeOrca {
		claim = "issueops execution claim --id " + quoteReplacementArg(record.ID) +
			" --generation " + strconv.FormatUint(generation, 10) + " --claim-current-token" +
			" --issue-body-sha256 " + strings.TrimSpace(issueBodySHA256) + " --context-packet-sha256 <PACKET_SHA256> " + actorFlags + " --json"
	}
	shortActor := strings.Join([]string{
		"--host", strings.ToLower(strings.TrimSpace(req.OwnerHost)), "--session-id", "<SESSION_ID>",
		"--cwd", quoteReplacementArg(record.Execution.Workspace.Root),
	}, " ")
	// 반납은 막힌 owner의 안전한 출구다. 들고 종료하면 프로세스가 살아 있는 한
	// 아무도 그 lifecycle을 회수할 수 없다(#319).
	release := "issueops execution release --id " + quoteReplacementArg(record.ID) +
		" --generation " + strconv.FormatUint(record.Execution.Lease.Generation, 10) + " " + actorFlags + " --json"
	verifyBranchLinkRead := "none"
	verifyBranchLink := "none"
	awaitBranchLink := "none"
	if prepared := record.BranchPrepare; prepared != nil && !prepared.LinkVerified {
		if strings.EqualFold(strings.TrimSpace(prepared.Provider), "github") {
			// GitHub에서만 pre-link 창이 존재한다. GitLab은 prepare 시점에
			// 이미 link_verified를 요구하므로 기다릴 것이 없다(#319).
			awaitBranchLink = "issueops branch await-link --id " + quoteReplacementArg(record.ID) + " --json"
			projectKey := policy.ProjectKey
			issueNumber := policy.IssueNumber
			repoSlug := strings.TrimPrefix(projectKey, "github.com/")
			if projectKey != "" && repoSlug != projectKey && issueNumber != "" {
				verifyBranchLinkRead = "gh issue develop --list " + issueNumber +
					" --repo " + quoteReplacementArg(repoSlug)
			}
		}
		verifyBranchLink = "issueops branch prepare --id " + quoteReplacementArg(record.ID) +
			" --provider " + quoteReplacementArg(strings.ToLower(strings.TrimSpace(prepared.Provider))) +
			" --issue-url " + quoteReplacementArg(strings.TrimSpace(prepared.IssueURL)) +
			" --branch " + quoteReplacementArg(strings.TrimSpace(prepared.Branch)) +
			" --base-branch " + quoteReplacementArg(strings.TrimSpace(prepared.BaseBranch))
		for _, optional := range []struct{ flag, value string }{
			{"--base-sha", prepared.BaseSHA},
			{"--parent-worktree", prepared.ParentWorktree},
			{"--remote-branch-url", prepared.RemoteBranchURL},
		} {
			if value := strings.TrimSpace(optional.value); value != "" {
				verifyBranchLink += " " + optional.flag + " " + quoteReplacementArg(value)
			}
		}
		verifyBranchLink += " --link-verified " + shortActor + " --json"
	}
	planPath := paths.Plan
	linkPlan := "none"
	if strings.TrimSpace(record.PlanPath) == "" {
		linkPlan = "issueops link-plan --id " + quoteReplacementArg(record.ID) +
			" --plan-path " + quoteReplacementArg(planPath) + " " + shortActor + " --json"
	}
	compatibilityReview := "issueops compatibility review --id " + quoteReplacementArg(record.ID) +
		" --backward-compatibility " + quoteReplacementArg("<BACKWARD_COMPATIBILITY>") +
		" --side-effect " + quoteReplacementArg("<SIDE_EFFECT>") +
		" --rollback-plan " + quoteReplacementArg("<ROLLBACK_PLAN>") +
		" --verification " + quoteReplacementArg("<COMPATIBILITY_VERIFICATION>") + " --approved " +
		shortActor + " --json"
	enterImplement := "issueops phase --id " + quoteReplacementArg(record.ID) +
		" --to implement " + shortActor + " --json"
	if record.Phase == issueops.IssueOpsPhaseAISlopClean || record.Phase == issueops.IssueOpsPhaseFeedback {
		enterImplement = "none"
	}
	aiSlopCleanRecord := "issueops ai-slop-clean record --id " + quoteReplacementArg(record.ID) +
		" --category <CLEANUP_CATEGORY> --verification <VERIFICATION_EVIDENCE> " + shortActor + " --json"
	enterAISlopClean := "issueops phase --id " + quoteReplacementArg(record.ID) +
		" --to ai-slop-clean " + shortActor + " --json"
	base := ""
	if record.BranchPrepare != nil {
		base = strings.TrimSpace(record.BranchPrepare.BaseBranch)
	}
	remote := "issueops remote create-pr --id " + quoteReplacementArg(record.ID) +
		" --expected-generation " + strconv.FormatUint(generation, 10) + " --title <PR_TITLE> --body-file <PR_BODY_FILE>" +
		" --head " + quoteReplacementArg(record.Execution.Workspace.Branch) + " --base " + quoteReplacementArg(base) +
		" --label <LABEL> --assignee <ASSIGNEE> " + actorFlags + " --confirm --json"
	complete := "issueops execution complete --id " + quoteReplacementArg(record.ID) +
		" --generation " + strconv.FormatUint(generation, 10) + " --final-head <FINAL_HEAD> --verification-report " + quoteReplacementArg(paths.Report) +
		" --remote-artifact-url <DRAFT_PR_OR_MR_URL> --verification <VERIFICATION_EVIDENCE> " + actorFlags + " --confirm --json"
	implementationReview := "issueops implementation-review record --id " + quoteReplacementArg(record.ID) +
		" --verdict <VERDICT> --finding <FINDING> --evidence <EVIDENCE> --reviewer-host " + strings.ToLower(strings.TrimSpace(req.OwnerHost)) +
		" --reviewer-model <REVIEWER_MODEL> --reviewer-effort <REVIEWER_EFFORT> " + shortActor + " --json"
	projectDocsReview := "issueops project-docs-review record --id " + quoteReplacementArg(record.ID) +
		" --verdict <PROJECT_DOCS_VERDICT> --doc <UPDATED_DOC_PATH> --reviewed-doc <REVIEWED_DOC_PATH> --evidence <PROJECT_DOCS_EVIDENCE> " + shortActor + " --json"
	schemaEvidence := "issueops schema-evidence record --id " + quoteReplacementArg(record.ID) +
		" --measurement <OBSERVED_VALUE> --source <OBSERVATION_SOURCE> " + shortActor + " --json"
	enterPR := "issueops phase --id " + quoteReplacementArg(record.ID) +
		" --to pr " + shortActor + " --json"
	return issueops.OwnerCommands{
		LeaseStatus: status, Claim: claim, VerifyBranchLinkRead: verifyBranchLinkRead,
		AwaitBranchLink:  awaitBranchLink,
		Release:          release,
		VerifyBranchLink: verifyBranchLink, LinkPlan: linkPlan,
		CompatibilityReview: compatibilityReview, EnterImplement: enterImplement,
		AISlopCleanRecord: aiSlopCleanRecord, EnterAISlopClean: enterAISlopClean,
		ImplementationReview: implementationReview,
		ProjectDocsReview:    projectDocsReview, SchemaEvidence: schemaEvidence,
		EnterPR:      enterPR,
		RemoteCreate: remote, Complete: complete,
	}
}

func OwnerVerificationCommands(body string) []string {
	lines := strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n")
	inSection, inFence := false, false
	commands := []string{}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "## ") {
			heading := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "## ")))
			inSection = heading == "검증" || heading == "검증 명령" ||
				heading == "verification" || heading == "verification commands"
			inFence = false
			continue
		}
		if !inSection {
			continue
		}
		if strings.HasPrefix(line, "```") {
			if inFence {
				break
			}
			inFence = true
			continue
		}
		if inFence && line != "" && !strings.HasPrefix(line, "#") {
			commands = append(commands, line)
		}
	}
	return uniqueExecutionOwnerValues(commands)
}

func uniqueExecutionOwnerValues(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

func renderExecutionOwnerLines(values []string) string {
	if len(values) == 0 {
		return "- none"
	}
	rows := append([]string(nil), values...)
	for index := range rows {
		rows[index] = "- " + rows[index]
	}
	return strings.Join(rows, "\n")
}

func ValidOwnerDigest(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 64 {
		return false
	}
	for _, c := range value {
		if !(c >= '0' && c <= '9') && !(c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

func OwnerWorkspaceMatches(source, root, branch, head bool, driver, runtimeID, repoID, worktreeID string) bool {
	return source && root && branch && head && driver == "orca" && strings.TrimSpace(runtimeID) != "" && strings.TrimSpace(repoID) != "" && strings.TrimSpace(worktreeID) != ""
}

func ValidateOwnerSnapshot(linkedURL, observedURL, body string, maxBytes int) ([]string, []string, error) {
	if strings.TrimSpace(observedURL) != strings.TrimSpace(linkedURL) || strings.TrimSpace(body) == "" || len(body) > maxBytes {
		return nil, nil, fmt.Errorf("remote issue snapshot identity or bounded body is invalid")
	}
	acceptance, verification := OwnerAcceptanceIDs(body), OwnerVerificationCommands(body)
	if len(acceptance) == 0 || len(verification) == 0 {
		return nil, nil, fmt.Errorf("remote issue must contain acceptance IDs and an exact verification command block")
	}
	return acceptance, verification, nil
}

func OwnerRequiredDocCandidates() []string {
	return []string{
		"AGENTS.md", ".issueops/CONSTITUTION.md", ".issueops/ARCHITECTURE.md", ".issueops/CONVENTIONS.md",
		".issueops/TESTING.md", ".issueops/CAUTIONS.md", ".issueops/TECH_STACK.md", ".issueops/ADR.md",
		".issueops/OPERATIONS.md", ".issueops/AGENT_WORKFLOW.md",
	}
}
