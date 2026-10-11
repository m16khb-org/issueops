package issueops

import (
	"path/filepath"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func covIZOrcaRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID: "io-1", Repo: "/work/repo", IssueURL: "https://github.com/acme/repo/issues/7", Branch: "feat/x",
		Phase: model.IssueOpsPhasePlan,
		BranchPrepare: &model.IssueOpsBranchPrepare{
			Provider: "github", IssueURL: "https://github.com/acme/repo/issues/7", Branch: "feat/x", BaseBranch: "main", BaseSHA: "abc123",
		},
		Execution: &model.Execution{
			Mode:      model.ExecutionModeOrca,
			Workspace: model.Workspace{SourceRoot: "/work/repo", Root: "/work/repo.worktrees/feat-x", Branch: "feat/x", BaseHead: "abc123"},
			Lease:     model.WriteLease{Generation: 4, Status: model.LeaseStatusActive},
		},
	}
}

func covIZPrepareRequest() model.ExecutionPrepareRequest {
	return model.ExecutionPrepareRequest{OwnerHost: " Codex ", OwnerModel: " m1 ", OwnerEffort: " high "}
}

func TestOwnerIssueProviderPrefersPreparedProviderThenFallsBackToURL(t *testing.T) {
	for _, tc := range []struct {
		name   string
		record model.IssueOpsRecord
		want   string
	}{
		{"prepared gitlab normalized", model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{Provider: " GitLab "}, IssueURL: "https://github.com/a/b/issues/1"}, "gitlab"},
		{"unknown prepared provider falls back to github url", model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{Provider: "bitbucket"}, IssueURL: "https://github.com/a/b/issues/1"}, "github"},
		{"gitlab url", model.IssueOpsRecord{IssueURL: "https://gitlab.example.com/a/b/-/issues/1"}, "gitlab"},
		{"unknown host", model.IssueOpsRecord{IssueURL: "https://example.com/a/b/issues/1"}, ""},
		{"empty record", model.IssueOpsRecord{}, ""},
	} {
		if got := OwnerIssueProvider(tc.record); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func covIZPacket() model.OwnerContextPacket {
	return model.OwnerContextPacket{
		SchemaVersion: 1, LifecycleID: "io-1", Mode: model.ExecutionModeOrca, SourceRoot: "/src", WorktreeRoot: "/wt", WorktreeBase: "/wt-base",
		Branch: "feat/x", BaseHead: "abc", LeaseGeneration: 4,
		Issue:     model.OwnerIssue{URL: "https://github.com/a/b/issues/1", BodySHA256: strings.Repeat("a", 64)},
		OwnerHost: "codex", OwnerModel: "m", OwnerEffort: "high", VerificationReportPath: "/wt/report.md",
		RequiredDocs: []string{"AGENTS.md"}, RequiredSkills: []string{"verified-execution"}, AcceptanceIDs: []string{"AC-01", "AC-02"}, Verification: []string{"go test ./..."},
		Commands: model.OwnerCommands{Claim: "claim --sha <PACKET_SHA256>", LeaseStatus: "status"},
	}
}

func TestRenderOwnerPromptSubstitutesPlaceholdersAndBoundsSize(t *testing.T) {
	packet := covIZPacket()
	digest := strings.Repeat("b", 64)
	template := "id={LIFECYCLE_ID} gen={LEASE_GENERATION} claim={CLAIM_COMMAND}\ndocs:\n{REQUIRED_DOCS}\nac={ACCEPTANCE_IDS}\nv:\n{VERIFICATION_COMMANDS}\nskills:{REQUIRED_SKILLS} path={PACKET_PATH} sha={PACKET_SHA256}"
	got, err := RenderOwnerPrompt(packet, "/wt/packet.json", digest, template, 4096)
	if err != nil {
		t.Fatal(err)
	}
	want := "id=io-1 gen=4 claim=claim --sha " + digest + "\ndocs:\n- AGENTS.md\nac=AC-01, AC-02\nv:\n- go test ./...\nskills:- verified-execution path=/wt/packet.json sha=" + digest
	if got != want {
		t.Fatalf("prompt=%q want=%q", got, want)
	}
	packet.RequiredDocs = nil
	empty, err := RenderOwnerPrompt(packet, "p", digest, "{REQUIRED_DOCS}", 100)
	if err != nil || empty != "- none" {
		t.Fatalf("empty list rendering=%q err=%v", empty, err)
	}
	if _, err := RenderOwnerPrompt(packet, "p", digest, template, 10); err == nil || !strings.Contains(err.Error(), "exceeds 10 bytes") {
		t.Fatalf("oversized prompt accepted: %v", err)
	}
	if got, err := RenderOwnerPrompt(packet, "p", digest, "{MODE}", len("orca")); err != nil || got != "orca" {
		t.Fatalf("prompt exactly at the limit rejected: %q %v", got, err)
	}
	if _, err := RenderOwnerPrompt(packet, "p", digest, "x {NO_SUCH_VALUE} {ALSO_MISSING}", 100); err == nil || !strings.Contains(err.Error(), "{NO_SUCH_VALUE}") {
		t.Fatalf("unknown placeholder accepted or misreported: %v", err)
	}
	if got, err := RenderOwnerPrompt(packet, "p", digest, "{lower} {1X} no placeholders", 100); err != nil || got != "{lower} {1X} no placeholders" {
		t.Fatalf("non-placeholder braces were rewritten: %q %v", got, err)
	}
}

func TestRenderOwnerPromptRejectsInjectionVectorsBeforeRendering(t *testing.T) {
	digest := strings.Repeat("b", 64)
	for _, tc := range []struct {
		name   string
		mutate func(*model.OwnerContextPacket)
		want   string
	}{
		{"branch newline", func(p *model.OwnerContextPacket) { p.Branch = "a\nb" }, "branch"},
		{"carriage return in source root", func(p *model.OwnerContextPacket) { p.SourceRoot = "a\rb" }, "source_root"},
		{"placeholder in model", func(p *model.OwnerContextPacket) { p.OwnerModel = "{BRANCH}" }, "owner_model"},
		{"placeholder in command", func(p *model.OwnerContextPacket) { p.Commands.Complete = "run {X1}" }, "complete_command"},
		{"newline in issue url", func(p *model.OwnerContextPacket) { p.Issue.URL = "u\n" }, "issue_url"},
		{"blank required doc", func(p *model.OwnerContextPacket) { p.RequiredDocs = []string{"a", "  "} }, "required_docs"},
		{"multiline verification", func(p *model.OwnerContextPacket) { p.Verification = []string{"go test\nrm -rf /"} }, "verification_commands"},
		{"placeholder acceptance", func(p *model.OwnerContextPacket) { p.AcceptanceIDs = []string{"{LIFECYCLE_ID}"} }, "acceptance_ids"},
		{"placeholder skill", func(p *model.OwnerContextPacket) { p.RequiredSkills = []string{"{MODE}"} }, "required_skills"},
	} {
		packet := covIZPacket()
		tc.mutate(&packet)
		_, err := RenderOwnerPrompt(packet, "p", digest, "{MODE}", 100)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
	}
	if _, err := RenderOwnerPrompt(covIZPacket(), "/p\nq", digest, "{MODE}", 100); err == nil || !strings.Contains(err.Error(), "packet_path") {
		t.Fatalf("multiline packet path accepted: %v", err)
	}
	if _, err := RenderOwnerPrompt(covIZPacket(), "p", "{MODE}", "{MODE}", 100); err == nil || !strings.Contains(err.Error(), "packet_sha256") {
		t.Fatalf("placeholder digest accepted: %v", err)
	}
}

func TestOwnerCommandsForBindsGenerationActorAndPhaseSpecificTransitions(t *testing.T) {
	record := covIZOrcaRecord()
	record.ID = "io'1"
	paths := model.OwnerPaths{Plan: "/wt/plan.md", Report: "/wt/rep ort.md"}
	policy := model.OwnerPolicyContext{ProjectKey: "github.com/acme/repo", IssueNumber: "7"}
	cmds := OwnerCommandsFor(record, covIZPrepareRequest(), " "+strings.Repeat("c", 64)+" ", paths, policy)

	if cmds.LeaseStatus != `issueops execution status --id 'io'\''1' --json` {
		t.Fatalf("status=%s", cmds.LeaseStatus)
	}
	for _, want := range []string{"--generation 4", "--claim-current-token", "--issue-body-sha256 " + strings.Repeat("c", 64) + " ", "--context-packet-sha256 <PACKET_SHA256>", "--host codex", "--cwd '/work/repo.worktrees/feat-x'"} {
		if !strings.Contains(cmds.Claim, want) {
			t.Fatalf("claim missing %q: %s", want, cmds.Claim)
		}
	}
	if !strings.HasPrefix(cmds.LinkPlan, "issueops link-plan --id 'io'\\''1' --plan-path '/wt/plan.md' --host codex --session-id <SESSION_ID>") {
		t.Fatalf("link plan=%s", cmds.LinkPlan)
	}
	if !strings.Contains(cmds.Release, "--generation 4 ") || !strings.Contains(cmds.Release, "--session-pid <SESSION_PID>") {
		t.Fatalf("release=%s", cmds.Release)
	}
	if !strings.Contains(cmds.Complete, "--verification-report '/wt/rep ort.md'") || !strings.Contains(cmds.Complete, "--generation 4") {
		t.Fatalf("complete=%s", cmds.Complete)
	}
	if !strings.Contains(cmds.RemoteCreate, "--head 'feat/x' --base 'main'") || !strings.Contains(cmds.RemoteCreate, "--expected-generation 4") {
		t.Fatalf("remote create=%s", cmds.RemoteCreate)
	}
	if !strings.Contains(cmds.ImplementationReview, "--reviewer-host codex") || !strings.HasSuffix(cmds.EnterPR, "--to pr --host codex --session-id <SESSION_ID> --cwd '/work/repo.worktrees/feat-x' --json") {
		t.Fatalf("review=%s enterPR=%s", cmds.ImplementationReview, cmds.EnterPR)
	}
	if !strings.Contains(cmds.EnterImplement, "--to implement") || !strings.Contains(cmds.EnterAISlopClean, "--to ai-slop-clean") {
		t.Fatalf("transitions=%s / %s", cmds.EnterImplement, cmds.EnterAISlopClean)
	}
}

func TestOwnerCommandsForDirectModeLinkedPlanAndLaterPhases(t *testing.T) {
	record := covIZOrcaRecord()
	record.Execution.Mode = model.ExecutionModeDirect
	record.PlanPath = "plan.md"
	cmds := OwnerCommandsFor(record, covIZPrepareRequest(), "digest", model.OwnerPaths{}, model.OwnerPolicyContext{})
	if cmds.Claim != "none" {
		t.Fatalf("direct mode must have no claim command: %s", cmds.Claim)
	}
	if cmds.LinkPlan != "none" {
		t.Fatalf("already linked plan must not be relinked: %s", cmds.LinkPlan)
	}
	if !strings.Contains(cmds.EnterImplement, "--to implement") {
		t.Fatalf("enter implement=%s", cmds.EnterImplement)
	}
	for _, phase := range []model.IssueOpsPhase{model.IssueOpsPhaseAISlopClean, model.IssueOpsPhaseFeedback} {
		record.Phase = phase
		if got := OwnerCommandsFor(record, covIZPrepareRequest(), "digest", model.OwnerPaths{}, model.OwnerPolicyContext{}).EnterImplement; got != "none" {
			t.Fatalf("phase %s offers to re-enter implement: %s", phase, got)
		}
	}
	record.BranchPrepare = nil
	if got := OwnerCommandsFor(record, covIZPrepareRequest(), "digest", model.OwnerPaths{}, model.OwnerPolicyContext{}).RemoteCreate; !strings.Contains(got, "--base ''") {
		t.Fatalf("missing branch prepare must render an empty quoted base: %s", got)
	}
}

func TestBranchLinkCommandsOnlyExistBeforeLinkVerification(t *testing.T) {
	policy := model.OwnerPolicyContext{ProjectKey: "github.com/acme/repo", IssueNumber: "7"}
	const short = "--host codex"

	record := covIZOrcaRecord()
	record.BranchPrepare.ParentWorktree = " /parent "
	record.BranchPrepare.RemoteBranchURL = "https://github.com/acme/repo/tree/feat/x"
	await, read, verify := branchLinkCommands(record, policy, short)
	if await != "issueops branch await-link --id 'io-1' --json" || read != "gh issue develop --list 7 --repo 'acme/repo'" {
		t.Fatalf("await=%q read=%q", await, read)
	}
	wantVerify := "issueops branch prepare --id 'io-1' --provider 'github' --issue-url 'https://github.com/acme/repo/issues/7' --branch 'feat/x' --base-branch 'main' --base-sha 'abc123' --parent-worktree '/parent' --remote-branch-url 'https://github.com/acme/repo/tree/feat/x' --link-verified --host codex --json"
	if verify != wantVerify {
		t.Fatalf("verify=%s", verify)
	}

	record.BranchPrepare.BaseSHA, record.BranchPrepare.ParentWorktree, record.BranchPrepare.RemoteBranchURL = " ", "", ""
	_, _, verify = branchLinkCommands(record, policy, short)
	if strings.Contains(verify, "--base-sha") || strings.Contains(verify, "--parent-worktree") || strings.Contains(verify, "--remote-branch-url") {
		t.Fatalf("blank optional flags rendered: %s", verify)
	}

	for name, p := range map[string]model.OwnerPolicyContext{
		"no project key":   {IssueNumber: "7"},
		"no issue number":  {ProjectKey: "github.com/acme/repo"},
		"foreign host key": {ProjectKey: "gitlab.com/acme/repo", IssueNumber: "7"},
	} {
		await, read, _ = branchLinkCommands(record, p, short)
		if read != "none" || !strings.HasPrefix(await, "issueops branch await-link") {
			t.Fatalf("%s: await=%q read=%q", name, await, read)
		}
	}

	record.BranchPrepare.Provider = "GitLab"
	await, read, verify = branchLinkCommands(record, policy, short)
	if await != "none" || read != "none" || !strings.Contains(verify, "--provider 'gitlab'") {
		t.Fatalf("gitlab has no pre-link window: await=%q read=%q verify=%q", await, read, verify)
	}

	record.BranchPrepare.LinkVerified = true
	if a, r, v := branchLinkCommands(record, policy, short); a != "none" || r != "none" || v != "none" {
		t.Fatalf("verified link still has commands: %q %q %q", a, r, v)
	}
	record.BranchPrepare = nil
	if a, r, v := branchLinkCommands(record, policy, short); a != "none" || r != "none" || v != "none" {
		t.Fatalf("unprepared branch has commands: %q %q %q", a, r, v)
	}
}

func TestOwnerPacketSealsExecutionIdentityAndOwnerProfile(t *testing.T) {
	record := covIZOrcaRecord()
	snapshot := model.OwnerSnapshot{
		Issue:        model.OwnerIssue{URL: record.IssueURL, Body: "AC-01", BodySHA256: strings.Repeat("d", 64)},
		RequiredDocs: []string{"AGENTS.md"}, RequiredSkills: []string{"s"}, AcceptanceIDs: []string{"AC-01"}, VerificationCommands: []string{"go test"},
	}
	paths := model.OwnerPaths{Plan: "/p", Report: "/r", WorktreeBase: "/base"}
	policy := model.OwnerPolicyContext{ReviewerModel: "rm", ReviewerEffort: "re", ResearchModel: "xm", ResearchEffort: "xe", ReaderCheckModel: "cm", ReaderCheckEffort: "ce"}
	manifest := map[string]string{"plan": "sha"}
	packet := OwnerPacket(record, covIZPrepareRequest(), snapshot, manifest, paths, policy)

	if packet.OwnerHost != "codex" || packet.OwnerModel != "m1" || packet.OwnerEffort != "high" {
		t.Fatalf("owner profile not normalized: %+v", packet)
	}
	if packet.LifecycleID != "io-1" || packet.Mode != model.ExecutionModeOrca || packet.LeaseGeneration != 4 || packet.BaseHead != "abc123" || packet.CurrentHead != "abc123" ||
		packet.WorktreeBase != "/base" || packet.VerificationReportPath != "/r" || packet.SourceRoot != "/work/repo" || packet.WorktreeRoot != record.Execution.Workspace.Root {
		t.Fatalf("identity=%+v", packet)
	}
	if packet.ReviewerModel != "rm" || packet.ResearchEffort != "xe" || packet.ReaderCheckModel != "cm" || packet.Issue.BodySHA256 != snapshot.Issue.BodySHA256 {
		t.Fatalf("policy/snapshot=%+v", packet)
	}
	if want := OwnerCommandsFor(record, covIZPrepareRequest(), snapshot.Issue.BodySHA256, paths, policy); packet.Commands != want {
		t.Fatalf("commands differ from OwnerCommandsFor:\n%+v\n%+v", packet.Commands, want)
	}
	if packet.ArtifactManifest["plan"] != "sha" || len(packet.AcceptanceIDs) != 1 || packet.Verification[0] != "go test" {
		t.Fatalf("artifacts=%+v", packet)
	}
}

func TestValidOwnerDigestRequiresExactLowercaseSHA256Hex(t *testing.T) {
	good := strings.Repeat("0123456789abcdef", 4)
	for value, want := range map[string]bool{
		good:                               true,
		" " + good + "\n":                  true,
		strings.ToUpper(good):              false,
		good[:63]:                          false,
		good + "0":                         false,
		strings.Repeat("g", 64):            false,
		"":                                 false,
		strings.Repeat("a", 63) + "\u00e9": false,
		strings.Repeat("a", 32) + " " + good[:31]: false,
	} {
		if got := ValidOwnerDigest(value); got != want {
			t.Fatalf("ValidOwnerDigest(%q)=%t want %t", value, got, want)
		}
	}
}

func TestOwnerWorkspaceMatchesRequiresEveryIdentityFact(t *testing.T) {
	if !OwnerWorkspaceMatches(true, true, true, true, "orca", "rt", "repo", "wt") {
		t.Fatal("complete identity rejected")
	}
	for name, ok := range map[string]bool{
		"source":  OwnerWorkspaceMatches(false, true, true, true, "orca", "rt", "repo", "wt"),
		"root":    OwnerWorkspaceMatches(true, false, true, true, "orca", "rt", "repo", "wt"),
		"branch":  OwnerWorkspaceMatches(true, true, false, true, "orca", "rt", "repo", "wt"),
		"head":    OwnerWorkspaceMatches(true, true, true, false, "orca", "rt", "repo", "wt"),
		"driver":  OwnerWorkspaceMatches(true, true, true, true, "git", "rt", "repo", "wt"),
		"runtime": OwnerWorkspaceMatches(true, true, true, true, "orca", " ", "repo", "wt"),
		"repo":    OwnerWorkspaceMatches(true, true, true, true, "orca", "rt", "", "wt"),
		"wt":      OwnerWorkspaceMatches(true, true, true, true, "orca", "rt", "repo", "\t"),
	} {
		if ok {
			t.Fatalf("missing %s accepted", name)
		}
	}
}

func TestOwnerPlanErrorsExposeStableMachineFields(t *testing.T) {
	required := &OwnerPlanRequiredError{}
	if required.Error() == "" {
		t.Fatal("empty error text")
	}
	fields := required.IssueOpsErrorFields()
	if fields["code"] != "orca_plan_artifact_required" || fields["missing"].([]string)[0] != "plan" {
		t.Fatalf("fields=%v", fields)
	}
	if _, present := fields["next_command"]; present {
		t.Fatalf("empty next_command must be omitted: %v", fields)
	}
	required.NextCommand = "issueops execution replace --preview"
	if got := required.IssueOpsErrorFields()["next_command"]; got != required.NextCommand {
		t.Fatalf("next_command=%v", got)
	}

	stale := &OwnerPlanReviewStaleError{NextCommand: "issueops review"}
	fields = stale.IssueOpsErrorFields()
	if fields["code"] != "devils_advocate_review_stale" || fields["missing"].([]string)[0] != "devils_advocate_review_stale" || fields["next_command"] != "issueops review" {
		t.Fatalf("fields=%v", fields)
	}
	if !strings.Contains(stale.Error(), "different plan") {
		t.Fatalf("error=%q", stale.Error())
	}
}

func TestOwnerWorkspaceLayoutDerivesCanonicalWorktreePaths(t *testing.T) {
	record := covIZOrcaRecord()
	layout, err := OwnerWorkspaceLayout(record)
	if err != nil {
		t.Fatal(err)
	}
	want := model.OwnerWorkspaceLayout{Root: filepath.Join("/work/repo.worktrees", "feat-x"), Branch: "feat/x", BaseBranch: "main", BaseHead: "abc123"}
	if layout != want {
		t.Fatalf("layout=%+v want=%+v", layout, want)
	}

	record.BranchPrepare.BaseBranch = "epic/one"
	record.BranchPrepare.ParentWorktree = "/work/repo.worktrees/../repo.worktrees/epic-one"
	layout, err = OwnerWorkspaceLayout(record)
	if err != nil {
		t.Fatal(err)
	}
	if layout.ParentWorktree != "/work/repo.worktrees/epic-one" || layout.ExpectedParent != "/work/repo.worktrees/epic-one" {
		t.Fatalf("explicit parent layout=%+v", layout)
	}
	record.BranchPrepare.ParentWorktree = "/elsewhere/epic-one"
	layout, err = OwnerWorkspaceLayout(record)
	if err != nil || layout.ParentWorktree != "/elsewhere/epic-one" || layout.ExpectedParent != "/work/repo.worktrees/epic-one" {
		t.Fatalf("non-canonical parent must surface the expected one: %+v %v", layout, err)
	}

	record.BranchPrepare.ParentWorktree = ""
	record.Delegation = &model.IssueOpsDelegationContract{ParentCycleID: "parent"}
	layout, err = OwnerWorkspaceLayout(record)
	if err != nil || layout.ParentWorktree != "/work/repo.worktrees/epic-one" || layout.ExpectedParent != "" {
		t.Fatalf("delegated cycle must derive its parent worktree: %+v %v", layout, err)
	}
	record.Delegation = &model.IssueOpsDelegationContract{ParentCycleID: "  "}
	layout, err = OwnerWorkspaceLayout(record)
	if err != nil || layout.ParentWorktree != "" {
		t.Fatalf("blank parent cycle id must not count as delegated: %+v %v", layout, err)
	}
}

func TestOwnerWorkspaceLayoutRejectsUnsafeInputs(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*model.IssueOpsRecord)
		want   string
	}{
		{"no branch prepare", func(r *model.IssueOpsRecord) { r.BranchPrepare = nil }, "base_sha is required"},
		{"blank base sha", func(r *model.IssueOpsRecord) { r.BranchPrepare.BaseSHA = "  " }, "base_sha is required"},
		{"empty branch", func(r *model.IssueOpsRecord) { r.Branch = " " }, "execution branch is invalid"},
		{"dot branch", func(r *model.IssueOpsRecord) { r.Branch = "." }, "execution branch is invalid"},
		{"dotdot branch", func(r *model.IssueOpsRecord) { r.Branch = ".." }, "execution branch is invalid"},
		{"parent without base branch", func(r *model.IssueOpsRecord) { r.BranchPrepare.ParentWorktree = "/p"; r.BranchPrepare.BaseBranch = "" }, "base branch is invalid"},
		{"delegated with dotdot base", func(r *model.IssueOpsRecord) {
			r.Delegation = &model.IssueOpsDelegationContract{ParentCycleID: "x"}
			r.BranchPrepare.BaseBranch = ".."
		}, "base branch is invalid"},
	} {
		record := covIZOrcaRecord()
		tc.mutate(&record)
		if _, err := OwnerWorkspaceLayout(record); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err=%v", tc.name, err)
		}
	}
}

func TestValidateOwnerWorkspaceParentReportsBothPaths(t *testing.T) {
	layout := model.OwnerWorkspaceLayout{ParentWorktree: "/actual", ExpectedParent: "/canonical"}
	if err := ValidateOwnerWorkspaceParent(layout, true); err != nil {
		t.Fatal(err)
	}
	err := ValidateOwnerWorkspaceParent(layout, false)
	if err == nil || !strings.Contains(err.Error(), `"/actual"`) || !strings.Contains(err.Error(), `"/canonical"`) {
		t.Fatalf("err=%v", err)
	}
}

func TestOwnerResumeCommandsQuoteIdentityAndBindArtifacts(t *testing.T) {
	if got := OwnerResumeNextCommand("io'1", 5, "ignored", "bodysha", "packetsha"); got != `issueops execution claim --id 'io'\''1' --generation 5 --claim-current-token --issue-body-sha256 bodysha --context-packet-sha256 packetsha` {
		t.Fatalf("resume=%s", got)
	}
	if got := ReplacementPreviewCommand("io-1", 2); got != "issueops execution replace --id 'io-1' --expected-generation 2 --preview" {
		t.Fatalf("preview=%s", got)
	}
	if got := ReplacementFinalizePreviewCommand("io-1", 2); got != "issueops execution replace --id 'io-1' --expected-generation 2 --finalize-preview" {
		t.Fatalf("finalize preview=%s", got)
	}
	if got := OwnerReseedNextCommand("io-1", 3, "orca", "/token"); got != ReplacementResumeCommand("io-1", 3) || !strings.HasSuffix(got, "--confirm") {
		t.Fatalf("orca reseed=%s", got)
	}
	if got := OwnerReseedNextCommand("io-1", 3, "direct", "/token"); got != ReplacementClaimCommand("io-1", 3, "/token") || !strings.HasSuffix(got, "--claim-current-token") {
		t.Fatalf("direct reseed=%s", got)
	}
	if got := OwnerReseedNextCommand("io-1", 3, "other", "/token"); got != "" {
		t.Fatalf("unknown mode produced command %q", got)
	}
}

func TestOwnerResumePlanRequiredOffersReplacementPreviewOnlyForALeasedExecution(t *testing.T) {
	for name, tc := range map[string]struct {
		record model.IssueOpsRecord
		want   string
	}{
		"no execution":    {model.IssueOpsRecord{ID: "io-1"}, ""},
		"zero generation": {model.IssueOpsRecord{ID: "io-1", Execution: &model.Execution{}}, ""},
		"leased":          {model.IssueOpsRecord{ID: "io-1", Execution: &model.Execution{Lease: model.WriteLease{Generation: 6}}}, ReplacementPreviewCommand("io-1", 6)},
	} {
		err := OwnerResumePlanRequired(tc.record)
		typed, ok := err.(*OwnerPlanRequiredError)
		if !ok || typed.NextCommand != tc.want {
			t.Fatalf("%s: %#v", name, err)
		}
	}
}

func TestCompleteOwnerArtifactIdentityRequiresVersionAndAllDigests(t *testing.T) {
	digest := strings.Repeat("e", 64)
	complete := func() *model.OrcaBinding {
		return &model.OrcaBinding{ArtifactIdentityVersion: model.OrcaArtifactIdentityVersion, IssueBodySHA256: digest, ContextPacketSHA256: digest, OwnerPromptSHA256: digest}
	}
	if !CompleteOwnerArtifactIdentity(complete()) {
		t.Fatal("complete identity rejected")
	}
	if CompleteOwnerArtifactIdentity(nil) {
		t.Fatal("nil binding accepted")
	}
	for name, mutate := range map[string]func(*model.OrcaBinding){
		"version": func(b *model.OrcaBinding) { b.ArtifactIdentityVersion = 0 },
		"issue":   func(b *model.OrcaBinding) { b.IssueBodySHA256 = "short" },
		"packet":  func(b *model.OrcaBinding) { b.ContextPacketSHA256 = "" },
		"prompt":  func(b *model.OrcaBinding) { b.OwnerPromptSHA256 = strings.ToUpper(digest) },
	} {
		binding := complete()
		mutate(binding)
		if CompleteOwnerArtifactIdentity(binding) {
			t.Fatalf("%s: incomplete identity accepted", name)
		}
	}
}

func TestValidateOwnerResumeGenerationRequiresLeasedOrcaExecution(t *testing.T) {
	orca := model.IssueOpsRecord{Execution: &model.Execution{Mode: model.ExecutionModeOrca, Lease: model.WriteLease{Generation: 1}}}
	if err := ValidateOwnerResumeGeneration(orca); err != nil {
		t.Fatal(err)
	}
	direct := orca
	direct.Execution = &model.Execution{Mode: model.ExecutionModeDirect, Lease: model.WriteLease{Generation: 1}}
	for name, record := range map[string]model.IssueOpsRecord{
		"no execution": {},
		"direct":       direct,
		"generation 0": {Execution: &model.Execution{Mode: model.ExecutionModeOrca}},
	} {
		if err := ValidateOwnerResumeGeneration(record); err == nil {
			t.Fatalf("%s accepted", name)
		}
	}
}

func TestValidateOwnerResumeProfileComparesHostModelAndEffort(t *testing.T) {
	packet := model.OwnerContextPacket{OwnerHost: "codex", OwnerModel: "m", OwnerEffort: "high"}
	binding := &model.OrcaBinding{OwnerHost: "codex", OwnerModel: "m", OwnerEffort: "high"}
	if err := ValidateOwnerResumeProfile(packet, binding); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*model.OrcaBinding){
		"host":   func(b *model.OrcaBinding) { b.OwnerHost = "claude" },
		"model":  func(b *model.OrcaBinding) { b.OwnerModel = "n" },
		"effort": func(b *model.OrcaBinding) { b.OwnerEffort = "low" },
	} {
		changed := *binding
		mutate(&changed)
		if err := ValidateOwnerResumeProfile(packet, &changed); err == nil {
			t.Fatalf("%s drift accepted", name)
		}
	}
}
