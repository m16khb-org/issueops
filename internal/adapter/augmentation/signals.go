package augmentation

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"

	contract "issueops/internal/contract/selfaugment"
)

type repoSignalRule struct {
	apply func(root string, signals *contract.SelfAugmentRepoSignals)
}

func (repo Repository) CollectSignals(root string, docsIndexed int, skills []string, geniusText string) contract.SelfAugmentRepoSignals {
	signals := contract.SelfAugmentRepoSignals{
		DocsIndexed:         docsIndexed,
		Skills:              append([]string{}, skills...),
		HasGeniusThink:      strings.TrimSpace(geniusText) != "",
		HasSelfAugmentSkill: slices.Contains(skills, "self-augment"),
	}
	for _, rule := range repo.signalRules() {
		rule.apply(root, &signals)
	}
	return signals
}

func (repo Repository) signalRules() []repoSignalRule {
	return []repoSignalRule{
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasSelfVerificationDocs = repo.DocsContainTerm(root, "Self-verification") && repo.DocsContainTerm(root, "Self-augmentation")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasSelfVerifyCLI = FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "root_command_facade.go"), `"self-verify":`) &&
				FileContainsTerm(root, filepath.Join("internal", "application", "selfverify", "loop.go"), "SelfVerificationKoreanName")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasSelfAugmentPlanner = FileContainsTerm(root, filepath.Join("internal", "application", "selfaugment", "planner.go"), "func (planner Planner) Plan(") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "self_workflow_planning_wiring.go"), "planner.Plan(")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasSelfAugmentStateCapture = FileContainsTerm(root, filepath.Join("internal", "application", "selfaugment", "save_plan.go"), "func SavePlan(") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "self_workflow_state_wiring.go"), "augmentapp.SavePlan(") &&
				repo.DocsContainTerm(root, "--save-state")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasSelfAugmentLessonCapture = FileContainsTerm(root, filepath.Join("internal", "application", "selfaugment", "save_lesson.go"), "func SaveLesson(") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "self_workflow_planning_wiring.go"), "augmentapp.SaveLesson(")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasAdapterContractMatrix = FileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallAdapterContractMatrix") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "testdata", "native_install_contract_matrix.golden.json"), "project-local-opt-in")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasRiskQATier = DirContainsTerm(root, filepath.Join("internal", "adapter", "verification", "riskqa"), "Validate") &&
				FileContainsTerm(root, filepath.Join("internal", "domain", "selfverify", "contract.go"), "risk_qa")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasGoalScoreSummary = FileContainsTerm(root, filepath.Join("internal", "domain", "selfaugment", "summary.go"), "GoalScores") &&
				FileContainsTerm(root, filepath.Join("internal", "domain", "selfaugment", "summary.go"), "MinimumGoalScore")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasRepoLocalSandbox = DirContainsTerm(root, filepath.Join("internal", "adapter", "policy"), "path_outside_workspace") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "policy", "policy_test.go"), "TestCommandPolicyDeniesPathArgsOutsideWorkspace") &&
				(DirContainsTerm(root, filepath.Join("internal", "adapter", "verification", "probe"), "policy deny outside path arg") ||
					DirContainsTerm(root, filepath.Join("internal", "adapter", "verification", "probe", "commandpolicy"), "policy deny outside path arg"))
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasPerformanceBaseline = FileContainsTerm(root, filepath.Join("internal", "domain", "selfaugment", "history_compare.go"), "SlowStepRegressions") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "selfworkflow", "historycompare", "self_augment_compare_test.go"), "TestCompareSelfAugmentSummariesDetectsSlowStepRegression") &&
				repo.DocsContainTerm(root, "slow_step:*")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasSelfAugmentSignalTable = FileContainsTerm(root, filepath.Join("internal", "adapter", "augmentation", "signals.go"), "func (repo Repository) signalRules() []repoSignalRule") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "augmentation", "signals.go"), "for _, rule := range repo.signalRules()")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasQualityInspectCLI = FileContainsTerm(root, filepath.Join("cmd", "issueops", "qualitycli", "quality_inspect.go"), "quality inspect") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "root_command_facade.go"), `"quality":`)
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasQualityInspectSignals = qualityInspectContainsTerm(root, "branch_candidate_functions") &&
				qualityInspectContainsTerm(root, "audit_p1_p2_items") &&
				qualityInspectContainsTerm(root, "low_coverage_packages")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasMCPResourceCoverage = FileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "catalog", "mcp", "resource_catalog_test.go"), "TestResourcesExposeStableDescriptors") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "catalog", "mcp", "catalog_assembly_test.go"), "TestResourceMapsPreserveDescriptorShape") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "mcpcli", "resources", "resources_test.go"), "TestHandleResourceReadReportsInvalidUnknownAndReadErrors") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "mcpcli", "resources", "resources_test.go"), "TestHandleResourceReadUsesCatalogSkillNameWhenConfigSkillNameIsEmpty") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "mcpcli", "resources", "context_determinism_test.go"), "TestResourcesContextIsByteDeterministic")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasHostJudgementCoverage = FileContainsTerm(root, filepath.Join("internal", "domain", "judgement", "structured_test.go"), "TestDecodeStructuredJSONObjectRejectsMalformedOutputs") &&
				FileContainsTerm(root, filepath.Join("internal", "domain", "judgement", "structured_test.go"), "TestDecodeStructuredJSONObjectBoundsLargeErrorOutput")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasIssueOpsLinkingBoundaryCoverage = FileContainsTerm(root, filepath.Join("internal", "application", "issueopsbranch", "link_test.go"), "TestLinkIssueRejectsInvalidURL") &&
				FileContainsTerm(root, filepath.Join("internal", "application", "issueopsbranch", "workspace_link_test.go"), "TestLinkPlanRejectsBoundaryViolations") &&
				FileContainsTerm(root, filepath.Join("internal", "application", "issueopsbranch", "workspace_link_test.go"), "plan_path does not exist") &&
				FileContainsTerm(root, filepath.Join("internal", "application", "issueopsbranch", "workspace_link_test.go"), "plan_path must be inside linked worktree") &&
				FileContainsTerm(root, filepath.Join("internal", "application", "issueopsbranch", "workspace_link_test.go"), "TestValidateIssueURL")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasStateWriteLocking = FileContainsTerm(root, filepath.Join("internal", "application", "state", "service.go"), "func (service *Service) Write(ctx context.Context, key, content string)") &&
				FileContainsTerm(root, filepath.Join("internal", "application", "state", "service.go"), "store.WithSpan(ctx, func(spanCtx context.Context) error {") &&
				FileContainsTerm(root, filepath.Join("internal", "application", "state", "service.go"), "service.writeRecord(spanCtx, store, dir, key, record)") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "outbound", "state", "state_io.go"), "func NewService() *stateapplication.Service {") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "outbound", "state", "state_test.go"), "TestStateWriteWaitsForKeyLock")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasWorkerStuckRunningDetection = FileContainsTerm(root, filepath.Join("internal", "application", "worker", "service.go"), "func (service Service) DetectStuck(ctx context.Context)") &&
				FileContainsTerm(root, filepath.Join("internal", "domain", "worker", "lifecycle.go"), "WorkerStatusFailed") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "worker_test.go"), "TestWorkerDetectStuckJobsMarksDeadPIDAsFailed") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "worker", "worker_test.go"), "TestWorkerDetectStuckJobsSkipsAlivePID") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "workercli", "worker.go"), `"cleanup-stuck"`) &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "workercli", "worker_queue_cli.go"), "func (command Command) RunCleanupStuck(") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "workercli", "worker_test.go"), "TestRunWorkerCleanupStuckMarksDeadPIDJobsFailed")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			hasDaemonConnectionCap := FileContainsTerm(root, filepath.Join("internal", "domain", "daemon", "settings.go"), "func MaxConnections(value string)") &&
				FileContainsTerm(root, filepath.Join("internal", "domain", "daemon", "settings.go"), "DefaultMaxConnections") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "daemon_wiring.go"), `domain.MaxConnections(os.Getenv("ISSUEOPS_DAEMON_MAX_CONNECTIONS"))`) &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "daemon_wiring.go"), "MaxConnections: reader.MaxConnections")
			signals.HasDaemonConnectionLimit = hasDaemonConnectionCap &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_server.go"), "newDaemonAdmission(deps.MaxConnections)") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_admission.go"), "case a.slots <- struct{}{}") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_admission.go"), "writeDaemonAdmissionError") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_server_loop_test.go"), "TestRunDaemonAcceptLoopRejectsWhenConnectionLimitReached") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "daemoncli", "daemon_server_loop_test.go"), "TestRunDaemonAcceptLoopExpires64IdleSessionsAndAdmitsInitialize")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasIssueOpsInboundAdapterCoverage = FileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsdecision", "decision_test.go"), "TestHandlersAddDelegatesToServiceAndAppliesDecision") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsinventory", "list_test.go"), "TestListHandlerDelegatesScanAndProjectsResult") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsretention", "prune_test.go"), "TestPruneHandlerReportsDryRunByDefault") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsrouting", "routing_test.go"), "TestRoutingHandlersDelegateRecordAndScore") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopsstatus", "status_test.go"), "TestStatusHandlerProjectsStoredRecord") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "issueopslease", "handlers_test.go"), "TestPublicClaimErrorMapsDenialsToStableMessages")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasToolConformanceTransportCoverage = FileContainsTerm(root, filepath.Join("internal", "contract", "toolconformance", "types_test.go"), "TestClassificationsCoverAllContractCases") &&
				FileContainsTerm(root, filepath.Join("internal", "contract", "toolconformance", "types_test.go"), "TestBenchmarkReportJSONRoundTripPreservesTypedEnums") &&
				FileContainsTerm(root, filepath.Join("internal", "domain", "issueops", "execution_sync_base_validation_test.go"), "TestValidateWriteLeaseStatusMatrix") &&
				FileContainsTerm(root, filepath.Join("internal", "contract", "issueops", "execution_sync_base_test.go"), "TestBaseSyncRequiredErrorCarriesReseedFreeNextCommand")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasGeniusMermaidLint = DirContainsTerm(root, filepath.Join("internal", "adapter", "verification", "probe"), "lintMermaidBlocks") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "verification", "probe", "validation_mcp_mermaid_native_wrappers_test.go"), "TestLintMermaidBlocksEnforcesGeniusThinkRules") &&
				!FileContainsTerm(root, filepath.Join(".issueops", "ARCHITECTURE.md"), `\n`)
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasInstallDryRunMode = DirContainsTerm(root, filepath.Join("cmd", "issueops", "installcli"), "dry-run") &&
				FileContainsTerm(root, filepath.Join("internal", "adapter", "install_contract_matrix_test.go"), "TestNativeInstallDryRunDoesNotWrite") &&
				repo.DocsContainTerm(root, "install --dry-run")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasCLIAdapterSplit = FileContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "catalog", "cli", "usage.go"), "func Usage") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "app.go"), "cliadapter.Usage")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasMCPAdapterCatalog = hasMCPAdapterCatalog(root)
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasCompatibilityContract = (FileContainsTerm(root, filepath.Join("cmd", "issueops", "contract.go"), "CompatibilityContract") ||
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "misc_facade.go"), "CompatibilityContract")) &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "issueopsapp", "root_command_facade.go"), `"contract":`)
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasCandidateRefill = FileContainsTerm(root, filepath.Join("internal", "domain", "selfaugment", "candidates.go"), "candidate-refill-curriculum") &&
				FileContainsTerm(root, filepath.Join("internal", "domain", "selfaugment", "candidates.go"), "release-repro-pack")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasCommandAuditLog = FileContainsTerm(root, filepath.Join("internal", "application", "audit", "service.go"), "func (service Service) Audit(") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "policycli", "policy_cli.go"), "command.Audit.Audit(")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasWorkerMVP = FileContainsTerm(root, filepath.Join("internal", "application", "worker", "service.go"), "func (service Service) Enqueue(") &&
				FileContainsTerm(root, filepath.Join("cmd", "issueops", "workercli", "worker_queue_cli.go"), "func (command Command) RunEnqueue(")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasReleaseReproPack = FileContainsTerm(root, filepath.Join("scripts", "release-repro-smoke.sh"), "install --dry-run --project-local --json") &&
				FileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Release Checklist") &&
				repo.DocsContainTerm(root, "release install reproducibility smoke")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			hasReleaseGuideHeading := readmeContainsTerm(root, "Release User Guide: Install, Update, Rollback") ||
				readmeContainsTerm(root, "## Release and rollback") ||
				readmeContainsTerm(root, "## 릴리스와 롤백")
			hasInstallCommand := readmeContainsTerm(root, "./install.sh")
			hasUpdateCommand := readmeContainsTerm(root, "issueops update") ||
				readmeContainsTerm(root, "io update")
			hasRollbackReference := readmeContainsTerm(root, ".issueops/operations/release-reproducibility.md")
			signals.HasReleaseUserReadme = hasReleaseGuideHeading &&
				hasInstallCommand &&
				hasUpdateCommand &&
				hasRollbackReference &&
				!readmeContainsTerm(root, "git reset --hard") &&
				FileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Release User Guide: Install, Update, Rollback")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasCrossPlatformBuildMatrix = FileContainsTerm(root, filepath.Join("scripts", "release-build-matrix.sh"), "darwin/arm64 darwin/amd64 linux/amd64 linux/arm64") &&
				FileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Cross-Platform Build Matrix") &&
				repo.DocsContainTerm(root, "cross-platform release build matrix smoke")
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasDistributionDecision = repo.DocsContainTerm(root, "2026-06-13 — Distribution decision gate") &&
				FileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Current decision: prefer tarball/manual archive") &&
				FileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Rollback criteria") &&
				(readmeContainsTerm(root, "Current distribution decision") || readmeContainsTerm(root, "현재 배포 결정"))
		}},
		{func(root string, signals *contract.SelfAugmentRepoSignals) {
			signals.HasReleaseDogfoodNotes = FileContainsTerm(root, filepath.Join(".issueops", "operations", "release-dogfood-notes.md"), "Codex MCP transcript") &&
				FileContainsTerm(root, filepath.Join(".issueops", "operations", "release-dogfood-notes.md"), "Claude MCP transcript") &&
				FileContainsTerm(root, filepath.Join(".issueops", "operations", "release-dogfood-notes.md"), "inspect/docs/state workflow") &&
				FileContainsTerm(root, filepath.Join(".issueops", "operations", "release-reproducibility.md"), "Release Dogfood Notes")
		}},
	}
}

func readmeContainsTerm(root, term string) bool {
	return FileContainsTerm(root, "README.md", term) || FileContainsTerm(root, "README.en.md", term)
}

func hasMCPAdapterCatalog(root string) bool {
	return DirContainsTerm(root, filepath.Join("internal", "contract", "mcp"), "AdapterOwnedTools") &&
		DirContainsTerm(root, filepath.Join("internal", "adapter", "inbound", "catalog", "mcp"), "contract.AdapterOwnedTools") &&
		rootWiresMCPCatalog(root)
}

func qualityInspectContainsTerm(root, term string) bool {
	return DirContainsTerm(root, filepath.Join("cmd", "issueops", "qualitycli"), term) ||
		DirContainsTerm(root, filepath.Join("internal", "contract", "quality"), term) ||
		DirContainsTerm(root, filepath.Join("internal", "core", "qualityinspect"), term)
}

// Observe the field assignment, not gofmt's alignment or a comment mentioning it.
func rootWiresMCPCatalog(root string) bool {
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(root, "cmd", "issueops", "issueopsapp", "mcp_facade.go"), nil, 0)
	if err != nil {
		return false
	}
	found := false
	ast.Inspect(file, func(node ast.Node) bool {
		literal, ok := node.(*ast.CompositeLit)
		if !ok {
			return true
		}
		typ, ok := literal.Type.(*ast.SelectorExpr)
		if !ok || typ.Sel.Name != "MCPDependencies" {
			return true
		}
		for _, element := range literal.Elts {
			field, ok := element.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			key, ok := field.Key.(*ast.Ident)
			if !ok || key.Name != "Catalog" {
				continue
			}
			call, ok := field.Value.(*ast.CallExpr)
			if !ok {
				continue
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Build" {
				continue
			}
			pkg, ok := selector.X.(*ast.Ident)
			if ok && pkg.Name == "mcpcatalog" {
				found = true
			}
		}
		return true
	})
	return found
}
