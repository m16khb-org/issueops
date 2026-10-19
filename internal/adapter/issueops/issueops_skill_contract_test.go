package issueops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 라우터는 phase enum과 공통 규칙을 소유하고, 단계 명령은 단계 스킬이 소유한다.
// 두 축을 각각 고정해 명령이 라우터로 되돌아오는 것과 스킬에서 사라지는 것을
// 함께 막는다.
func TestIssueOpsSkillKeepsCoreWorkflowContract(t *testing.T) {
	t.Parallel()

	skill := readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md")
	for _, want := range []string{
		"problem", "grill", "issue", "plan", "compatibility-review", "implement",
		"ai-slop-clean", "feedback", "pr", "cleanup", "RED→GREEN→SURFACE→CLEAN",
		"issueops next --json",
	} {
		if !strings.Contains(skill, want) {
			t.Fatalf("IssueOps router missing current workflow contract %q", want)
		}
	}
	stages := map[string]string{
		"issueops intent record":       readIssueOpsContractFile(t, "skills", "issueops-create-issue", "SKILL.md"),
		"issueops architecture review": readIssueOpsContractFile(t, "skills", "issueops-plan", "SKILL.md"),
		"issueops execution prepare --id \"$ISSUEOPS_ID\" $ACTOR_FLAGS --json": readIssueOpsContractFile(t, "skills", "issueops-plan", "SKILL.md"),
	}
	for want, document := range stages {
		if !strings.Contains(document, want) {
			t.Fatalf("stage skill missing owned command %q", want)
		}
	}
}

func TestIssueOpsExecutionDocumentationHasOneCurrentContract(t *testing.T) {
	t.Parallel()

	documents := map[string]string{
		"skill":        readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md"),
		"execution":    readIssueOpsContractFile(t, "skills", "issueops", "references", "execution.md"),
		"cleanup":      readIssueOpsContractFile(t, "skills", "issueops", "references", "cleanup-state.md"),
		"plan":         readIssueOpsContractFile(t, "skills", "issueops-plan", "SKILL.md"),
		"providers":    readIssueOpsContractFile(t, ".issueops", "operations", "guides", "issueops-providers.md"),
		"workflow":     readIssueOpsContractFile(t, ".issueops", "AGENT_WORKFLOW.md"),
		"operations":   readIssueOpsContractFile(t, ".issueops", "OPERATIONS.md"),
		"architecture": readIssueOpsContractFile(t, ".issueops", "ARCHITECTURE.md"),
	}
	all := joinIssueOpsContractDocuments(documents)
	for _, want := range []string{
		"one `Execution`", "canonical worktree", "exact lifecycle ID",
		"source main worktree remains available",
		"issueops execution prepare", "--confirm", "issueops execution status",
		"issueops execution claim", "--claim-current-token",
		"issueops execution release", "issueops execution replace", "--preview",
		"--reseed", "issueops execution reconcile", "issueops execution complete",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("current execution v1 contract missing %q", want)
		}
	}
	for _, removed := range removedIssueOpsExecutionTerms() {
		for name, document := range documents {
			if strings.Contains(strings.ToLower(document), removed) {
				t.Fatalf("%s retains removed execution contract term %q", name, removed)
			}
		}
	}
	if count := strings.Count(documents["execution"], "issueops execution prepare --id"); count != 2 {
		t.Fatalf("execution reference must show exactly the preview and confirm prepare commands, count=%d", count)
	}
	if count := strings.Count(documents["execution"], "issueops execution prepare --id \"$ISSUEOPS_ID\" $ACTOR_FLAGS --confirm --json"); count != 1 {
		t.Fatalf("execution reference must show one confirm command, count=%d", count)
	}
}

func TestIssueOpsDocumentationPreservesGitHubBranchOrdering(t *testing.T) {
	t.Parallel()

	operationsIndex := readIssueOpsContractFile(t, ".issueops", "OPERATIONS.md")
	if !strings.Contains(operationsIndex, "operations/guides/issueops-providers.md") {
		t.Fatal("OPERATIONS.md must route provider ordering to operations/guides/issueops-providers.md")
	}
	providers := readIssueOpsContractFile(t, ".issueops", "operations", "guides", "issueops-providers.md")
	const want = "`branch prepare` (base SHA only) → GraphQL `createLinkedBranch` with `oid=sealed base SHA` → `branch prepare --link-verified`"
	if !strings.Contains(providers, want) {
		t.Errorf("provider guide missing GitHub branch ordering %q", want)
	}
	if strings.Contains(providers, "gh issue develop --name") {
		t.Error("provider guide reintroduces the superseded GitHub branch creation command")
	}
}

func TestIssueOpsExecutionDocumentationPreservesParallelIndependence(t *testing.T) {
	t.Parallel()

	all := strings.ToLower(joinIssueOpsContractDocuments(map[string]string{
		"execution": readIssueOpsContractFile(t, "skills", "issueops", "references", "execution.md"),
		"router":    readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md"),
		"workflow":  readIssueOpsContractFile(t, ".issueops", "AGENT_WORKFLOW.md"),
	}))
	for _, want := range []string{
		"exact lifecycle id",
		"canonical worktree",
		"one active execution per record",
		"unrelated cycles",
		"source main worktree remains available",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("parallel execution documentation missing %q", want)
		}
	}
}

func TestIssueOpsHandoffDocsRouteToDrainAndFreshnessModel(t *testing.T) {
	t.Parallel()

	sessionChoice := readIssueOpsContractFile(t, "skills", "issueops", "references", "session-choice.md")
	planSkill := readIssueOpsContractFile(t, "skills", "issueops-plan", "SKILL.md")
	implementSkill := readIssueOpsContractFile(t, "skills", "issueops-implement", "SKILL.md")
	all := strings.ToLower(sessionChoice + "\n" + planSkill + "\n" + implementSkill)
	for _, want := range []string{
		"새 쓰기 작업과 하위 작업 dispatch를 중지",
		"대기 중인 writer가 0",
		"인계 자료 digest",
		"stale하거나 누락된 근거는 필요한 범위만 다시 확인",
		"독립 claim",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("handoff routing/freshness contract missing %q", want)
		}
	}
	if release := strings.Index(strings.ToLower(sessionChoice), "issueops execution release"); release >= 0 {
		for _, beforeRelease := range []string{"대기 중인 writer가 0", "인계 자료 digest"} {
			if index := strings.Index(strings.ToLower(sessionChoice), beforeRelease); index < 0 || index > release {
				t.Fatalf("%q must be required before execution release", beforeRelease)
			}
		}
	}
}

func TestIssueOpsHandoffRoutesEveryNewSessionThroughProductionObservation(t *testing.T) {
	t.Parallel()

	sessionChoice := strings.Join(strings.Fields(strings.ToLower(readIssueOpsContractFile(t, "skills", "issueops", "references", "session-choice.md"))), " ")
	for _, want := range []string{
		"herdr를 호출할 때는 `issueops trace handoff-delivery`를 공통 producer로 사용한다",
		"herdr 전송도 아래 `trace handoff-delivery` producer",
		"issueops trace handoff-delivery --input",
		"retry, claim, status를 바꾸지 않는다",
		"herdr_wait_state`이며 native turn 증거가 아니다",
		`"source_generation": <generation>`,
		`"process": { "pid": <receiver pid>`,
		`"started_at": "<receiver process rfc3339nano start time>"`,
		`"executable": "<absolute observed receiver executable>"`,
		"staged observation에는 launcher가 아직 반환하지 않은 process를 추측해 넣지 않는다",
		"claim holder의 `session_process`와 pid·시작 시각·executable이 모두 일치",
	} {
		if !strings.Contains(sessionChoice, want) {
			t.Fatalf("new-session observation routing contract missing %q", want)
		}
	}
}

func TestIssueOpsHandoffPrefersHerdrInsideHerdrSession(t *testing.T) {
	t.Parallel()

	normalize := func(text string) string {
		return strings.Join(strings.Fields(strings.ToLower(text)), " ")
	}
	sessionChoice := normalize(readIssueOpsContractFile(t, "skills", "issueops", "references", "session-choice.md"))
	for _, want := range []string{
		"`herdr_env=1`",
		"`herdr_socket_path`",
		"env는 먼저 확인할 런처만 정한다",
		"herdr 세션이 아니거나 herdr가 사용 불가임이 확인됐으면",
	} {
		if !strings.Contains(sessionChoice, want) {
			t.Fatalf("Herdr session preference contract missing %q", want)
		}
	}
	for name, summary := range map[string]struct {
		path []string
		want string
	}{
		"issueops":           {[]string{"skills", "issueops", "SKILL.md"}, "준비한 뒤 herdr를 확인한다"},
		"issueops-plan":      {[]string{"skills", "issueops-plan", "SKILL.md"}, "준비 세션이 herdr 세션이면"},
		"issueops-implement": {[]string{"skills", "issueops-implement", "SKILL.md"}, "herdr를 먼저 확인한다"},
		"workflow":           {[]string{".issueops", "AGENT_WORKFLOW.md"}, "herdr 세션이면 herdr를 먼저 확인"},
	} {
		if !strings.Contains(normalize(readIssueOpsContractFile(t, summary.path...)), summary.want) {
			t.Fatalf("%s must summarize the Herdr session preference %q", name, summary.want)
		}
	}
}

// 자동 세션 선택 순서는 Herdr 확인이 먼저고 그 뒤에 현재 세션 또는 수동 두 번째
// 터미널이다. 새 세션 인수는 `replace --preview` → `--reseed` →
// `claim --claim-current-token` 체인 하나다.
func TestIssueOpsAutomaticSessionSelectionOrderAndTakeoverChain(t *testing.T) {
	t.Parallel()

	normalize := func(text string) string { return strings.Join(strings.Fields(text), " ") }
	skill := normalize(readIssueOpsContractFile(t, "skills", "issueops", "SKILL.md"))
	_, section, found := strings.Cut(skill, "## 환경별 자동 세션 인계")
	if !found {
		t.Fatal("issueops skill is missing the automatic session handoff section")
	}
	section, _, _ = strings.Cut(section, " ## ")
	herdr := strings.Index(section, "준비한 뒤 Herdr를 확인한다")
	current := strings.Index(section, "Herdr를 쓸 수 없으면 현재 세션에서 이어가거나, 사용자가 직접 연 두 번째 터미널에서 수동 인계한다")
	if herdr < 0 || current < 0 || herdr >= current {
		t.Fatalf("Herdr check must precede the current-session or manual second-terminal fallback: herdr=%d current=%d", herdr, current)
	}

	sessionChoice := normalize(readIssueOpsContractFile(t, "skills", "issueops", "references", "session-choice.md"))
	const chain = "`execution replace --id ID --expected-generation N --preview`를 실행한 뒤, 반환된 `--reseed` 명령과 `claim --claim-current-token`으로 인수한다"
	if !strings.Contains(sessionChoice, chain) {
		t.Fatalf("session-choice must pin the replace --preview -> --reseed -> claim --claim-current-token chain %q", chain)
	}
}

func TestIssueOpsHandoffLaunchesEveryNativeHostWithPermissionBypass(t *testing.T) {
	t.Parallel()

	sessionChoice := readIssueOpsContractFile(t, "skills", "issueops", "references", "session-choice.md")
	for _, flag := range []string{
		"--dangerously-skip-permissions",
		"--dangerously-bypass-approvals-and-sandbox",
		"--permission-preset full-access",
		"--auto-approve",
		"opencode --standalone --auto",
	} {
		if count := strings.Count(sessionChoice, flag); count < 3 {
			t.Fatalf("explicit launcher, new-session, and Herdr procedures must each require %q: count=%d", flag, count)
		}
	}
}

func TestIssueOpsCmuxHandoffIsExplicitAndFollowsTheDefaultFallback(t *testing.T) {
	t.Parallel()

	sessionChoice := strings.Join(strings.Fields(strings.ToLower(readIssueOpsContractFile(t, "skills", "issueops", "references", "session-choice.md"))), " ")
	fallback := strings.Index(sessionChoice, "herdr를 쓸 수 없음이 확인됐으면 현재 세션에서 이어가는 `current`다")
	explicit := strings.Index(sessionChoice, "사용자가 명시한 cmux")
	if fallback < 0 || explicit < 0 || explicit <= fallback {
		t.Fatalf("explicit cmux procedure must follow the default Herdr -> current fallback: fallback=%d explicit=%d", fallback, explicit)
	}
	if strings.Contains(sessionChoice[:explicit], "cmux") {
		t.Fatal("the default launcher selection must not probe or select cmux")
	}
	for _, want := range []string{
		"issueops execution handoff-cmux",
		"--cmux-executable",
		"--socket",
		"--window",
		"cmux omo",
		"raw_input",
		"native_turn_observed",
		"owner_claimed",
		"다시 실행하지 않는다",
		"다른 런처나 current로 전환하지 않는다",
		"endpoint incarnation",
	} {
		if !strings.Contains(sessionChoice[explicit:], want) {
			t.Fatalf("explicit cmux handoff contract missing %q", want)
		}
	}
}

func TestIssueOpsOrchestrationBindsOmoAgentsToCanonicalWorktrees(t *testing.T) {
	t.Parallel()

	all := strings.ToLower(joinIssueOpsContractDocuments(map[string]string{
		"orchestration": readIssueOpsContractFile(t, "skills", "issueops", "references", "orchestration.md"),
		"host-testing":  readIssueOpsContractFile(t, ".issueops", "testing", "cli-mcp-and-hosts.md"),
	}))
	for _, want := range []string{
		"team_create.members[].worktreepath",
		"resident child process `cwd`",
		"plain `task`",
		"pi_session_id",
		"issueops execution whoami --json",
		"branded `omo -p`",
	} {
		if !strings.Contains(all, want) {
			t.Fatalf("Omo worktree orchestration contract missing %q", want)
		}
	}
}

func TestIssueOpsCurrentSurfacesDoNotNameRemovedCommands(t *testing.T) {
	t.Parallel()

	for _, parts := range [][]string{
		{"internal", "adapter", "inbound", "catalog", "cli", "usage.go"},
		{"cmd", "issueops", "issueopscli", "issueops_cli_support.go"},
		{"internal", "domain", "commandparse", "issueops.go"},
		{"internal", "contract", "mcp", "issueops_catalog.go"},
	} {
		content := readIssueOpsContractFile(t, parts...)
		for _, removed := range removedIssueOpsCurrentCommandTerms() {
			if strings.Contains(strings.ToLower(content), removed) {
				t.Fatalf("%s retains removed handoff surface %q", filepath.Join(parts...), removed)
			}
		}
	}
}

// .issueops/SUB_AGENT_PATTERNS.md is routed from the issueops skill as the
// sub-agent decision contract, so it must not resurrect removed decision
// commands or MCP tools either.
func TestSubAgentPatternsDocDoesNotNameRemovedIssueOpsSurfaces(t *testing.T) {
	t.Parallel()

	content := strings.ToLower(readIssueOpsContractFile(t, ".issueops", "SUB_AGENT_PATTERNS.md"))
	for _, removed := range removedIssueOpsExecutionTerms() {
		if strings.Contains(content, removed) {
			t.Fatalf(".issueops/SUB_AGENT_PATTERNS.md retains removed surface %q", removed)
		}
	}
}

func removedIssueOpsCurrentCommandTerms() []string {
	return []string{
		"migrate-v9", "execution decide", "worktree prepare", "handoff start",
		"handoff claim", "handoff complete", "force-release", "resume --repo",
		"issueops heartbeat", "reconcile-create", "prepare-worktree-tools",
		"execution " + "resume", "switch" + "-mode", "--direct-" + "reason", "--owner-" + "host",
		"--issue-snapshot" + "-file", "--preserve-" + "terminal",
	}
}

func removedIssueOpsExecutionTerms() []string {
	return append(removedIssueOpsCurrentCommandTerms(),
		"execution_handoff", "ownership_epoch", "ownership_dispatch", "owner_orienting",
		"owner_active", "cleanup_pending_human_decision", "cleanup_executing",
		"--orchestrator", "prep-only", "issueops_record_execution_decision",
		"issueops_record_compatibility_review", "issueops_regress_for_replan",
	)
}

func joinIssueOpsContractDocuments(documents map[string]string) string {
	parts := make([]string, 0, len(documents))
	for _, document := range documents {
		parts = append(parts, document)
	}
	return strings.Join(parts, "\n")
}

func readIssueOpsContractFile(t *testing.T, parts ...string) string {
	t.Helper()
	path := filepath.Join(append([]string{"..", "..", ".."}, parts...)...)
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(content)
}
