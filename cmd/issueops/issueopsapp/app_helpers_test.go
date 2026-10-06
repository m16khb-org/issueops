package issueopsapp

import (
	reviewprompt "issueops/cmd/issueops/apidoc/reviewprompt"
	commandstep "issueops/cmd/issueops/commandstep"
	mcpcli "issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/pathutil"
	llmeval "issueops/cmd/issueops/selfworkflow/llmeval"
	augmentation "issueops/internal/adapter/augmentation"
	reviewfiles "issueops/internal/adapter/outbound/apidoc/reviewfiles"
	verification "issueops/internal/adapter/verification"
	riskqa "issueops/internal/adapter/verification/riskqa"
	app "issueops/internal/application/apidoc"
	augmentapp "issueops/internal/application/selfaugment"
	verifyapp "issueops/internal/application/selfverify"
	riskqaxx "issueops/internal/contract/riskqa"
	augmentcontract "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"
	apidoccontract "issueops/internal/domain/apidoc"
	riskqadomain "issueops/internal/domain/riskqa"
	domain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"

	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	updatecli "issueops/cmd/issueops/updatecli"
	updateadapter "issueops/internal/adapter/update"
	qualityapp "issueops/internal/application/quality"
	qualitycontract "issueops/internal/contract/quality"
	updatecontract "issueops/internal/contract/update"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	issueopscontract "issueops/internal/contract/issueops"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCommandStepHelpers(t *testing.T) {
	step := runCommandStep("", "echo", time.Second, "", "sh", "-c", "printf ok")
	if !step.OK || step.Stdout != "ok" {
		t.Fatalf("runCommandStep = %#v", step)
	}
	envStep := runCommandStepEnv("", "env", time.Second, "", []string{"HARNESSAPP_TEST_ENV=ok"}, "sh", "-c", "printf $HARNESSAPP_TEST_ENV")
	if !envStep.OK || envStep.Stdout != "ok" {
		t.Fatalf("runCommandStepEnv = %#v", envStep)
	}
	budgetStep := verification.RunEnv("", "budget", time.Second, "", nil, 2, "sh", "-c", "printf abc")
	if !budgetStep.OK || !budgetStep.StdoutTruncated {
		t.Fatalf("runCommandStepEnvWithBudget = %#v", budgetStep)
	}
	commandstep.PrintStep(selfverify.StepResult{Label: "covered", OK: true})
	if out, truncated, n := verifydomain.TailWithBudget("abcdef", 3); out == "" || !truncated || n != 6 {
		t.Fatalf("tailWithBudget = %q %v %d", out, truncated, n)
	}
	if !strings.Contains(commandstep.IndentLines("a\nb"), "  a") {
		t.Fatal("indentLines did not indent")
	}
}

func TestAppAndRootCommandHelpers(t *testing.T) {
	var buf bytes.Buffer
	fprintString(&buf, "hello")
	fprintUsage(&buf)
	if !strings.Contains(buf.String(), "hello") {
		t.Fatalf("buffer = %q", buf.String())
	}
	cmd := rootCommand()
	if cmd.Version != version || len(cmd.Runners) == 0 {
		t.Fatalf("root command = %#v", cmd)
	}
	if _, ok := cmd.Runners["install-native"]; ok {
		t.Fatal("retired install-native alias remains routed")
	}
	if rootSubcommandErrorExitCode("unknown", errors.New("bad")) != 1 {
		t.Fatal("default root subcommand exit code changed")
	}
	if code := RunRootCommand([]string{"--version"}); code != 0 {
		t.Fatalf("RunRootCommand --version = %d", code)
	}
}

func TestRunMCPCommandCleanupJSONUsesDryRunByDefaultAndApplyWhenRequested(t *testing.T) {
	binary := "/repo/bin/issueops"
	list := func() ([]updatecontract.MCPProxyProcess, error) {
		return []updatecontract.MCPProxyProcess{{
			PID:              44,
			ParentPID:        1,
			Command:          binary + " mcp",
			StartTime:        "orphan-start",
			Executable:       binary,
			IdentityVerified: true,
		}}, nil
	}
	var terminated []int
	terminate := func(pid int) error {
		terminated = append(terminated, pid)
		return nil
	}

	command := updatecli.CleanupCommand{Effects: rootCleanupEffects{list: list, terminate: terminate}}

	if err := command.Run([]string{"--json"}); err != nil {
		t.Fatal(err)
	}
	if len(terminated) != 0 {
		t.Fatalf("dry-run cleanup terminated processes: %#v", terminated)
	}

	if err := command.Run([]string{"--apply", "--json"}); err != nil {
		t.Fatal(err)
	}
	expectedTerminated := []int(nil)
	if runtime.GOOS == "darwin" {
		expectedTerminated = []int{44}
	}
	if !reflect.DeepEqual(terminated, expectedTerminated) {
		t.Fatalf("apply cleanup terminated = %#v", terminated)
	}
}

func TestHostAndPathHelpers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	t.Setenv("HARNESSAPP_BOOL", "true")
	t.Setenv("HARNESSAPP_FLOAT", "1.5")
	if err := os.MkdirAll(filepath.Join(root, "skills", skillName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "skills", skillName, "SKILL.md"), []byte("skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	if text, err := pathutil.ReadHarnessFile(issueOpsRoot(), "skills", skillName, "SKILL.md"); err != nil || text != "skill" {
		t.Fatalf("readHarnessFile = %q err=%v", text, err)
	}
	if issueOpsRoot() != root {
		t.Fatalf("issueOpsRoot = %q", issueOpsRoot())
	}
	if found, ok := pathutil.FindUp(filepath.Join(root, "skills", skillName), "SKILL.md"); !ok || found != filepath.Join(root, "skills", skillName) {
		t.Fatalf("findUp = %q %v", found, ok)
	}
	if pathutil.ResolveTarget(root) != root {
		t.Fatal("path facade wrappers failed")
	}

}

func TestUpdateAndAPIDocHelpers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	command := newUpdateCommand()
	if err := command.Run("update", []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if err := command.Run("bootstrap", []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	if err := command.Run("update", []string{"--dry-run"}); err != nil {
		t.Fatal(err)
	}
	binary := filepath.Join(root, "bin", "issueops")
	if parsed, ok := updateadapter.ParseMCPProxyProcessSnapshot("22 1 "+binary+" mcp", binary); !ok || parsed.PID != 22 {
		t.Fatalf("proxy parse %+v %v", parsed, ok)
	}

	var buf bytes.Buffer
	if err := printJSONTo(&buf, map[string]any{"ok": true}); err != nil || !strings.Contains(buf.String(), `"ok"`) {
		t.Fatalf("printJSONTo = %q err=%v", buf.String(), err)
	}
	if !isAPIDocReviewGateError(app.ErrReviewGateFailed) || !isAPIDocStaticGateError(app.ErrStaticGateFailed) {
		t.Fatal("gate error wrappers failed")
	}
	if len(reviewprompt.Schema()) == 0 {
		t.Fatal("apiDocReviewSchema empty")
	}
	if !apidoccontract.IsCandidate("src/user.controller.ts") {
		t.Fatal("isAPIDocCandidate failed")
	}
	if got := reviewfiles.Normalize(root, []string{filepath.Join(root, "src", "user.controller.ts")}); len(got) != 1 {
		t.Fatalf("normalizeAPIDocFiles = %#v", got)
	}
	_ = apidoccontract.CheckNestController("user.controller.ts", "@Controller('users')\nexport class UserController {}")
	_ = apidoccontract.CheckNestDTO("dto.ts", "export class UserDto {}")
	_ = reviewprompt.Build([]string{"a.ts"}, "diff", "extra", "")
	_ = reviewfiles.Evidence("/tmp", []string{"a.ts"})
}

func TestSelfWorkflowAndLLMHelpers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("GENIUS_THINK quality inspect"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs", "note.md"), []byte("coverage signal"), 0o644); err != nil {
		t.Fatal(err)
	}
	history := augmentcontract.SelfAugmentHistoryResult{Entries: []augmentcontract.SelfAugmentHistoryEntry{}}
	if err := applySelfAugmentHistoryRetention(&history, augmentcontract.SelfAugmentHistoryRetentionOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, ok := domain.ParseHistoryTimestamp(time.Now().UTC().Format(time.RFC3339Nano)); !ok {
		t.Fatal("parseSelfAugmentTimestamp failed")
	}
	if got := nonNilStringSlice(nil); got == nil {
		t.Fatal("nonNilStringSlice returned nil")
	}
	if got := nonNilSlowStepSlice(nil); got == nil {
		t.Fatal("nonNilSlowStepSlice returned nil")
	}
	signals := collectSelfAugmentRepoSignals(root, 1, []string{"self-verify"}, "GENIUS_THINK")
	candidates := augmentapp.Candidates(signals)
	if len(candidates) == 0 {
		t.Fatal("selfAugmentCandidates empty")
	}
	if domain.ScoreBool(true) <= domain.ScoreBool(false) {
		t.Fatal("scoreBool failed")
	}
	if !domain.AllGoalsPassed([]augmentcontract.SelfAugmentGoal{{Passed: true}}) {
		t.Fatal("allSelfAugmentGoalsPassed failed")
	}
	if domain.SelectedCandidateID(nil) != "" {
		t.Fatal("selectedCandidateID nil should be empty")
	}
	_ = docsContainTerm(root, "coverage")
	_ = augmentation.FileContainsTerm(root, "README.md", "quality")
	_ = augmentation.DirContainsTerm(root, "docs", "signal")
	_ = domain.SelectGeniusFormulas("invert the problem and use first principles")
	_ = domain.ResearchInfluences()
	domain.MarkSatisfiedCandidate(&candidates[0], signals)
	_ = domain.CandidateScore(candidates[0])
	_ = domain.CompareSlowestStepRegressions(nil, nil, 10)
	_ = domain.CompareStepBudgetRegressions(nil, nil, 10)
	if missing := domain.MissingStrings([]string{"a", "b"}, []string{"a"}); len(missing) != 1 || missing[0] != "b" {
		t.Fatalf("missingStrings = %#v", missing)
	}
	_ = domain.StepDurationStatByLabel(nil)
	_ = domain.MaxSlowStepDurationByLabel(nil)
	_ = domain.BuildStepDurationStats(map[string][]int64{"a": {1, 2}})
	result := augmentcontract.SelfAugmentResult{OK: true, Iterations: 0, Runs: []augmentcontract.SelfAugmentIteration{}}
	summary := summarizeSelfAugment(result)
	_ = domain.StepDurationStatsForCompare(summary)
	verifySummary := verifyapp.SummarizeSelfVerification(result, 95)
	_, _, _ = verifyapp.ClassifySelfVerificationFailure(result, verifySummary)
	_ = verifydomain.SelfVerifyRerunCommands("go test", 100, 95)
	_, _ = verifydomain.SelfVerifyStepRerunCommand("go test ./...")
	if verifydomain.FormatScore(95.5) == "" {
		t.Fatal("formatScore empty")
	}
	_ = verifyapp.MapGoalScores(result, 95)
	if len(verifydomain.ContractValue().RequiredFields) == 0 {
		t.Fatal("selfVerificationContract empty")
	}
	if coverage, _ := verifydomain.CoverageForLabels([]string{"go test ./..."}); coverage == nil {
		t.Fatal("selfVerificationCoverage nil")
	}
	if len(verifydomain.CoverageDefinitions()) == 0 {
		t.Fatal("selfVerificationCoverageDefinitions empty")
	}

	if err := verifydomain.ValidateLLMEvalMode("advisory"); err != nil {
		t.Fatal(err)
	}
	if verifydomain.NormalizeLLMEvalMode("") == "" {
		t.Fatal("normalizeSelfVerifyLLMEvalMode empty")
	}
	if _, _, err := verifydomain.ParseLLMEvalEnv("off"); err != nil {
		t.Fatal(err)
	}
	if _, err := llmeval.ResolveSelfVerifyLLMEvalConfig(false, false, "", false, func(string) (string, bool) { return "", false }); err != nil {
		t.Fatal(err)
	}
	if llmeval.BoundedLLMEvalError("prefix", errors.New("bad"), strings.Repeat("x", 100)) == "" {
		t.Fatal("boundedLLMEvalError empty")
	}
	_, _ = llmeval.ApplySelfVerifyLLMEval(result, selfverify.LLMEvalOptions{})
	_, _, _ = llmeval.BuildSelfVerifyLLMEvalPrompt(result)
	if llmeval.SelfVerifyLLMResponseSchemaExample() == "" || len(llmeval.SelfVerifyLLMResponseFieldTypes()) == 0 {
		t.Fatal("LLM response schema wrappers empty")
	}
}

func TestRiskMCPAndIssueOpsPolicyHelpers(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	riskStep := validateRiskQATierWithDeps(root, riskQATierDeps{
		plan: func(string) riskqaxx.RiskQATierPlan {
			return riskqaxx.RiskQATierPlan{Tier: "static", Commands: []string{"go test ./..."}, Reasons: []string{"test"}}
		},
		run: func(root string, command string) selfverify.StepResult {
			return selfverify.StepResult{OK: true, Label: "risk", Command: command}
		},
	})
	if !riskStep.OK {
		t.Fatalf("validateRiskQATierWithDeps = %#v", riskStep)
	}
	plan := riskqadomain.PlanFromPaths([]string{"cmd/issueops/issueopsapp/mcp_facade.go"})
	if plan.Tier == "" || riskqa.PlanJSON(plan) == "" {
		t.Fatalf("risk plan = %#v", plan)
	}
	_ = riskqa.Plan(root)

	if err := verifyIssueOpsRemoteArtifactLive(issueopscontract.IssueOpsRemoteArtifactVerificationRequest{Provider: "github", Kind: "pr", URL: "not-a-url"}); err == nil {
		t.Fatal("invalid remote artifact URL should fail")
	}
	if _, _, err := parseCommandPolicyFlags("policy", []string{"--workspace-root", root, "--cwd", root, "--", "git", "status"}); err != nil {
		t.Fatalf("parseCommandPolicyFlags: %v", err)
	}
	if _, _, _, err := parseCommandPolicyRunFlags([]string{"--workspace-root", root, "--cwd", root, "--", "git", "status"}); err != nil {
		t.Fatalf("parseCommandPolicyRunFlags: %v", err)
	}
	if err := runIssueOps([]string{"unknown"}); err == nil {
		t.Fatal("unknown issueops subcommand should fail")
	}
	if err := runPolicy([]string{"unknown"}); err == nil {
		t.Fatal("unknown policy subcommand should fail")
	}

	if len(mcpTools()) == 0 || len(mcpResources()) == 0 {
		t.Fatal("MCP catalog wrappers empty")
	}
	if result := mcpcli.TextResult("hello"); result["content"] == nil {
		t.Fatalf("textResult = %#v", result)
	}
	if _, rpcErr := callSDKTool(t, json.RawMessage(`{"name":"unknown","arguments":{}}`), issueOpsMCPDependencies()); rpcErr == nil {
		t.Fatal("unknown MCP tool should fail")
	}
	if _, rpcErr := handleResourceRead(json.RawMessage(`{"uri":"unknown://resource"}`)); rpcErr == nil {
		t.Fatal("unknown MCP resource should fail")
	}
	serverConn, clientConn := net.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- serveMCPStreamContext(ctx, serverConn, serverConn, io.Discard) }()
	client := mcp.NewClient(&mcp.Implementation{Name: "issueopsapp-facade-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.IOTransport{Reader: clientConn, Writer: clientConn}, nil)
	if err != nil {
		t.Fatal(err)
	}
	tools, err := session.ListTools(ctx, nil)
	if err != nil || len(tools.Tools) == 0 {
		t.Fatalf("serveMCPStream tool listing failed: tools=%#v err=%v", tools, err)
	}
	_ = session.Close()
	cancel()
	_ = clientConn.Close()
	<-done
}

func TestCLIHelpers(t *testing.T) {
	root := t.TempDir()
	writeValidZeroAudit(t, root)
	stateDir := t.TempDir()
	t.Setenv("ISSUEOPS_ROOT", root)
	t.Setenv("ISSUEOPS_STATE_DIR", stateDir)
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "install-native.sh"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("docs"), 0o644); err != nil {
		t.Fatal(err)
	}

	_ = runDocs([]string{"--json"})
	_ = runDocsWithRoot([]string{"--json"}, root)
	_ = runPreflight([]string{root})
	_ = runTrace([]string{"unknown"})
	_ = runTraceAnalyze([]string{"--help"})
	_ = runGuard([]string{"unknown"})
	_ = runGuardCheck([]string{"--repo", root, "--json"})
	_ = runQuality([]string{"unknown"})
	if err := runQualityInspectWithDeps([]string{"--json"}, qualityInspectDepsForIssueOpsAppTest()); err != nil {
		t.Fatalf("runQualityInspectWithDeps: %v", err)
	}
	_ = runInspect([]string{"--json", root})
	_ = runDoctor([]string{"--json"})

	_ = runInstall([]string{"--help"})

	_ = runProject([]string{"unknown"})

	_ = runState([]string{"unknown"})
	_ = runStatus([]string{"--repo", root, "--json"})
	_ = buildHarnessStatus(root)
	_ = runVerifyWork([]string{"--repo", root, "--json"})
	_ = buildVerifyWork(root, false, []string{"go", "test"})

	_ = runWorker([]string{"unknown"})
	_ = runWorkerEnqueue([]string{"--help"})
	_ = runWorkerRun([]string{"--help"})
	_ = runWorkerStatus([]string{"--id", "missing", "--json"})
	_ = runWorkerList([]string{"--json"})
	_ = runWorkerCancel([]string{"--id", "missing", "--json"})
}

func TestSelfVerifyHelpers(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_ROOT", t.TempDir())
	export := exportSelfVerificationCandidates()
	if export.Kind == "" {
		t.Fatalf("export = %#v", export)
	}
	catalog := verifydomain.CandidateCatalog()
	if len(catalog) == 0 {
		t.Fatal("self verification catalog empty")
	}
	_ = verifydomain.CandidateIDsByStatus(catalog, "open")
	_ = verifydomain.SelectedCandidateID(&catalog[0])
	_ = verifydomain.SelectedCandidateID(nil)
	if err := runSelfVerifyCandidatesWithDeps([]string{"--json"}, selfVerifyCandidatesDeps{
		export: func() augmentcontract.SelfVerificationCandidateExportResult { return export },
		save:   func(*augmentcontract.SelfVerificationCandidateExportResult, string) error { return nil },
	}); err != nil {
		t.Fatalf("runSelfVerifyCandidatesWithDeps: %v", err)
	}
	if err := saveSelfVerificationCandidateExport(&export, "candidate-export-test"); err != nil {
		t.Fatalf("saveSelfVerificationCandidateExport: %v", err)
	}
	baseline := augmentcontract.SelfAugmentStateSnapshot{Kind: domain.SelfVerificationSummaryKind}
	candidate := augmentcontract.SelfAugmentStateSnapshot{Kind: domain.SelfVerificationSummaryKind}
	_ = compareSelfAugmentSummariesFromSnapshots("base", "candidate", 10, baseline, candidate)
	_ = newSelfAugmentCompareResult("base", "candidate", 10)
	if _, err := compareSelfAugmentSummaries("missing-base", "missing-candidate", 10); err == nil {
		t.Fatal("missing summary compare should fail")
	}
	if _, err := selfAugmentHistory("", 1); err != nil {
		t.Fatalf("selfAugmentHistory: %v", err)
	}
	if err := runSelfVerifyPromoteWithDeps([]string{"--from-key", "a", "--baseline-key", "b", "--confirm"}, selfVerifyPromoteDeps{
		promote: func(fromKey, baselineKey string, confirm, allowFailedSource bool) (augmentcontract.SelfAugmentPromoteResult, error) {
			return augmentcontract.SelfAugmentPromoteResult{OK: true, FromKey: fromKey, BaselineKey: baselineKey}, nil
		},
	}); err != nil {
		t.Fatalf("runSelfVerifyPromoteWithDeps: %v", err)
	}
	if _, err := promoteSelfAugmentBaseline("missing", "baseline", false, false); err == nil {
		t.Fatal("promote without confirm/missing source should fail")
	}
	if _, err := readSelfAugmentStateSnapshot("missing"); err == nil {
		t.Fatal("missing snapshot read should fail")
	}
	if !domain.IsSelfVerificationSummaryKind(domain.SelfVerificationSummaryKind) || boolPtr(true) == nil {
		t.Fatal("summary kind/boolPtr wrappers failed")
	}
	result := newSelfVerifyLoopResult(1, 100, 95)
	if result.Iterations != 1 {
		t.Fatalf("newSelfVerifyLoopResult = %#v", result)
	}
	emitSelfVerifyLoopStart(nil, "self-verify", 1, 100)
	emitSelfVerifyLoopEnd(nil, "self-verify", 1, 100, true, "")
	if err := saveSelfVerificationSummary(&result, "summary-test"); err != nil {
		t.Fatalf("saveSelfVerificationSummary: %v", err)
	}
	if err := saveSelfAugmentSummary(&result, "augment-summary-test"); err != nil {
		t.Fatalf("saveSelfAugmentSummary: %v", err)
	}
	_ = domain.NewSelfVerificationSummarySnapshot(result, time.Now())
	_ = plannedSelfVerifySteps(t.TempDir(), "", 100, nil)
	_ = cachedContractGoldenStep(selfverify.StepResult{OK: false, Label: "go test"})
	_ = selfVerifyLoopDeps(issueOpsRoot())
	_ = selfVerifyStepDeps(issueOpsRoot())
	step := runCommandStep("", "adapter", time.Second, "", "sh", "-c", "printf ok")
	if !step.OK {
		t.Fatalf("runCommandStepAdapter = %#v", step)
	}
	_ = runSelfVerifyCandidates([]string{"--json"})
	_ = runSelfVerifyCompare([]string{"--baseline", "missing", "--candidate", "missing", "--json"})
	_ = runSelfVerifyHistory([]string{"--json", "--limit", "1"})
	_ = runSelfVerifyPromote([]string{"--from-key", "missing", "--baseline-key", "baseline"})
	_ = runSelfVerify([]string{"candidates", "--json"})
}

func qualityInspectDepsForIssueOpsAppTest() qualityapp.InspectDeps {
	return qualityapp.InspectDeps{
		Now:                  func() string { return "2026-01-01T00:00:00Z" },
		Coverage:             func(string) (string, error) { return "ok\tpkg\tcoverage: 100.0% of statements\n", nil },
		SelfAugmentOpenCount: func(string) (int, error) { return 0, nil },
		SelfVerifyOpenCount:  func(string) (int, error) { return 0, nil },
		PioneerCoverage: func(string) (qualitycontract.PioneerCoverage, error) {
			return qualitycontract.PioneerCoverage{
				Expected:             12,
				BenchmarkObserved:    12,
				ReproductionObserved: 12,
				IsolatedExpected:     12,
				IsolatedObserved:     12,
				IsolatedPassed:       12,
			}, nil
		},
		CodeSNR: func(string) (qualitycontract.SNRResult, error) {
			return qualitycontract.SNRResult{SignalLines: 70, NoiseLines: 30, TotalLines: 100, Ratio: 0.7}, nil
		},
	}
}

type rootCleanupEffects struct {
	list      func() ([]updatecontract.MCPProxyProcess, error)
	terminate func(int) error
}

func (e rootCleanupEffects) List() ([]updatecontract.MCPProxyProcess, error) { return e.list() }
func (e rootCleanupEffects) Terminate(pid int) error                         { return e.terminate(pid) }
func (rootCleanupEffects) CurrentPID() int                                   { return os.Getpid() }
func (rootCleanupEffects) SupportsOrphanTermination() bool                   { return runtime.GOOS == "darwin" }
