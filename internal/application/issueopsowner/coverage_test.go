package issueopsowner

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	remote "issueops/internal/contract/executionissue"
	model "issueops/internal/contract/issueops"
	prep "issueops/internal/contract/issueopspreparation"
	domain "issueops/internal/domain/issueops"
	reviewdomain "issueops/internal/domain/issueopsreview"
)

type stagedErrFiles struct {
	*ownerFiles
	err error
}

func (f *stagedErrFiles) ReadStaged(string) (map[string]string, error) { return nil, f.err }

// materializeFiles lets a test control what Materialize reports and whether the
// linked plan becomes readable afterwards.
type materializeFiles struct {
	*ownerFiles
	manifest map[string]string
	err      error
}

func (f *materializeFiles) Materialize(model.IssueOpsRecord) (map[string]string, error) {
	return f.manifest, f.err
}

type promptFailFiles struct {
	*ownerFiles
	err error
}

func (f *promptFailFiles) Write(root, path string, data []byte) error {
	if path == "/wt/prompt.md" {
		return f.err
	}
	return f.ownerFiles.Write(root, path, data)
}

func requiredPlanError(t *testing.T, err error) *domain.OwnerPlanRequiredError {
	t.Helper()
	required, ok := errors.AsType[*domain.OwnerPlanRequiredError](err)
	if !ok {
		t.Fatalf("want OwnerPlanRequiredError, got %T %v", err, err)
	}
	return required
}

func TestRequirePlanPropagatesStagedReadFailure(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	boom := errors.New("staged store unavailable")
	s.Files = &stagedErrFiles{ownerFiles: f, err: boom}
	if _, err := s.RequirePlan(r); !errors.Is(err, boom) {
		t.Fatalf("staged read error must surface unchanged: %v", err)
	}
}

func TestRequirePlanOffersQuotedStageCommandWhenBlankPlanHasLinkedPlan(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	f.plan = " \n\t"
	f.linked = model.OwnerPlanIdentity{Path: "/wt/it's plan.md", Digest: "d"}
	r.ID = "io-o'wner"
	_, err := s.RequirePlan(r)
	got := requiredPlanError(t, err).NextCommand
	want := "issueops artifact stage --id 'io-o'\\''wner' --name plan --file '/wt/it'\\''s plan.md' --json"
	if got != want {
		t.Fatalf("next command = %q want %q", got, want)
	}
}

func TestRequirePlanOmitsStageCommandWhenNoLinkedPlanExists(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	f.plan = ""
	_, err := s.RequirePlan(r)
	if got := requiredPlanError(t, err).NextCommand; got != "" {
		t.Fatalf("no linked plan means no stage command, got %q", got)
	}
}

func TestRequirePlanRejectsStaleDevilsAdvocateVerdictBeforeImplement(t *testing.T) {
	s, _, r, _, _, _ := ownerFixture(t)
	r.ID = "io-o'wner"
	r.Phase = model.IssueOpsPhasePlan
	r.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{RecordedAt: "t", Verdict: "proceed", ReviewedPlanDigest: "other"}
	_, err := s.RequirePlan(r)
	stale, ok := errors.AsType[*domain.OwnerPlanReviewStaleError](err)
	if !ok {
		t.Fatalf("want stale review error, got %v", err)
	}
	if !strings.HasPrefix(stale.NextCommand, "issueops devils-advocate review --id 'io-o'\\''wner' ") {
		t.Fatalf("next command must quote the id: %q", stale.NextCommand)
	}
}

func TestRequirePlanAcceptsReviewBoundToStagedPlanCaseInsensitively(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r.Phase = model.IssueOpsPhasePlan
	r.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{RecordedAt: "t", ReviewedPlanDigest: strings.ToUpper(digest([]byte(f.plan)))}
	identity, err := s.RequirePlan(r)
	if err != nil || identity.Digest != digest([]byte(f.plan)) || identity.Path != "" {
		t.Fatalf("unlinked bound plan identity: %+v %v", identity, err)
	}
}

func TestRequirePlanSkipsDigestBindingForInheritedParentReviewAndLaterPhases(t *testing.T) {
	for name, mutate := range map[string]func(*model.IssueOpsRecord){
		"delegated parent review": func(r *model.IssueOpsRecord) {
			r.Phase = model.IssueOpsPhasePlan
			r.DevilsAdvocateReview.ReviewerPattern = reviewdomain.ParentReviewPattern
		},
		"implement phase": func(r *model.IssueOpsRecord) { r.Phase = model.IssueOpsPhaseImplement },
	} {
		t.Run(name, func(t *testing.T) {
			s, _, r, _, _, _ := ownerFixture(t)
			r.DevilsAdvocateReview = &model.IssueOpsDevilsAdvocateReview{RecordedAt: "t", ReviewedPlanDigest: "stale"}
			mutate(&r)
			if _, err := s.RequirePlan(r); err != nil {
				t.Fatalf("stale digest must be tolerated here: %v", err)
			}
		})
	}
}

func TestRequirePlanRejectsLinkedPlanWhoseDigestDiffersFromStaged(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r.PlanPath = "/wt/plan.md"
	f.linked = model.OwnerPlanIdentity{Path: "/wt/plan.md", Digest: "different"}
	_, err := s.RequirePlan(r)
	if got := requiredPlanError(t, err).NextCommand; got != "" {
		t.Fatalf("drifted linked plan must not offer a stage command: %q", got)
	}
}

func TestRequirePlanRejectsUnreadableLinkedPlan(t *testing.T) {
	s, _, r, _, _, _ := ownerFixture(t)
	r.PlanPath = "/wt/plan.md"
	_, err := s.RequirePlan(r)
	requiredPlanError(t, err)
}

func TestRequirePlanReturnsLinkedIdentityWhenDigestsAgree(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r.PlanPath = "/wt/plan.md"
	f.linked = model.OwnerPlanIdentity{Path: "/wt/plan.md", Digest: digest([]byte(f.plan))}
	identity, err := s.RequirePlan(r)
	if err != nil || identity != f.linked {
		t.Fatalf("identity=%+v err=%v", identity, err)
	}
}

func TestMaterializePlanRejectsManifestDigestForDifferentPlan(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	s.Files = &materializeFiles{ownerFiles: f, manifest: map[string]string{"plan": "0000"}}
	if _, _, err := s.MaterializePlan(r); err == nil {
		t.Fatal("manifest plan digest differing from staged plan must be rejected")
	} else {
		requiredPlanError(t, err)
	}
}

func TestMaterializePlanRejectsManifestWithoutPlanEntry(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	s.Files = &materializeFiles{ownerFiles: f, manifest: map[string]string{}}
	_, _, err := s.MaterializePlan(r)
	requiredPlanError(t, err)
}

func TestMaterializePlanPropagatesMaterializeFailure(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	boom := errors.New("copy failed")
	s.Files = &materializeFiles{ownerFiles: f, err: boom}
	if _, _, err := s.MaterializePlan(r); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

func TestMaterializePlanRejectsWhenSealedPlanCannotBeLinkedBack(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	// Materialize reports the right digest but never makes the plan linkable.
	s.Files = &materializeFiles{ownerFiles: f, manifest: map[string]string{"plan": digest([]byte(f.plan))}}
	_, _, err := s.MaterializePlan(r)
	requiredPlanError(t, err)
}

func TestMaterializePlanKeepsAlreadyLinkedPlanIdentity(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r.PlanPath = "/wt/plan.md"
	f.linked = model.OwnerPlanIdentity{Path: "/wt/plan.md", Digest: digest([]byte(f.plan))}
	identity, manifest, err := s.MaterializePlan(r)
	if err != nil || identity != f.linked || manifest["plan"] != identity.Digest {
		t.Fatalf("identity=%+v manifest=%v err=%v", identity, manifest, err)
	}
}

func orcaRecord(t *testing.T, f *ownerFiles, r model.IssueOpsRecord) model.IssueOpsRecord {
	t.Helper()
	r.PlanPath = "/wt/plan.md"
	r.Execution.Orca = &model.OrcaBinding{OwnerHost: "codex", OwnerModel: "gpt-6-sol", OwnerEffort: "high"}
	f.linked = model.OwnerPlanIdentity{Path: r.PlanPath, Digest: digest([]byte(f.plan))}
	return r
}

func TestResealIsNoOpForNonOrcaExecution(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r.Execution.Mode = model.ExecutionModeDirect
	got, err := s.Reseal(t.Context(), r)
	if err != nil || got != (model.ReplacementArtifacts{}) || len(f.events) != 0 {
		t.Fatalf("direct execution must not reseal: %+v %v %v", got, err, f.events)
	}
	r.Execution = nil
	if got, err := s.Reseal(t.Context(), r); err != nil || got != (model.ReplacementArtifacts{}) {
		t.Fatalf("unprepared execution must not reseal: %+v %v", got, err)
	}
}

func TestResealRequiresLinkedPlanPath(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r = orcaRecord(t, f, r)
	r.PlanPath = "  "
	_, err := s.Reseal(t.Context(), r)
	requiredPlanError(t, err)
	if len(f.events) != 0 {
		t.Fatalf("plan path check must precede any file access: %v", f.events)
	}
}

func TestResealPropagatesPlanPreflightFailure(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r = orcaRecord(t, f, r)
	f.plan = ""
	_, err := s.Reseal(t.Context(), r)
	requiredPlanError(t, err)
}

func TestResealFailsWithoutRemoteIssueReader(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r = orcaRecord(t, f, r)
	s.ReadIssue = nil
	_, err := s.Reseal(t.Context(), r)
	if err == nil || !strings.Contains(err.Error(), "without a remote issue reader") {
		t.Fatalf("err=%v", err)
	}
}

func TestResealWrapsRemoteReadFailure(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r = orcaRecord(t, f, r)
	boom := errors.New("network down")
	s.ReadIssue = func(context.Context, string, remote.ExecutionIssueSnapshotRequest) (remote.ExecutionIssueSnapshot, error) {
		return remote.ExecutionIssueSnapshot{}, boom
	}
	_, err := s.Reseal(t.Context(), r)
	if !errors.Is(err, boom) || !strings.Contains(err.Error(), "could not be read for resealing") || len(f.writes) != 0 {
		t.Fatalf("err=%v writes=%d", err, len(f.writes))
	}
}

func TestResealRejectsMaterializedPlanAtDifferentPath(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r = orcaRecord(t, f, r)
	r.PlanPath = "/elsewhere/plan.md"
	f.linked.Path = "/wt/plan.md"
	_, err := s.Reseal(t.Context(), r)
	requiredPlanError(t, err)
	if len(f.writes) != 0 {
		t.Fatal("no artifacts may be written for a mismatched plan path")
	}
}

func TestResealRewritesPacketAndPromptBoundToCurrentIssueBody(t *testing.T) {
	s, f, r, _, intent, _ := ownerFixture(t)
	r = orcaRecord(t, f, r)
	got, err := s.Reseal(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	if got.IssueBodySHA256 != intent.IssueBodySHA256 || got.ContextPacketSHA256 != digest(f.writes["/wt/packet.json"]) || got.OwnerPromptSHA256 != digest(f.writes["/wt/prompt.md"]) {
		t.Fatalf("receipt does not describe written artifacts: %+v", got)
	}
	var packet model.OwnerContextPacket
	if err := json.Unmarshal(f.writes["/wt/packet.json"], &packet); err != nil {
		t.Fatal(err)
	}
	if packet.OwnerHost != "codex" || packet.OwnerModel != "gpt-6-sol" || packet.OwnerEffort != "high" {
		t.Fatalf("owner profile must come from the Orca binding: %+v", packet)
	}
}

func TestReseedRequiresReplacementGeneration(t *testing.T) {
	s, _, _, _, _, _ := ownerFixture(t)
	called := false
	s.ReadRecord = func(string) (model.IssueOpsRecord, error) { called = true; return model.IssueOpsRecord{}, nil }
	_, err := s.Reseed(t.Context(), "io-owner", model.Execution{Lease: model.WriteLease{Generation: 1}})
	if err == nil || !strings.Contains(err.Error(), "replacement generation") || called {
		t.Fatalf("err=%v readRecord=%v", err, called)
	}
}

func TestReseedPropagatesRecordReadFailure(t *testing.T) {
	s, _, _, _, _, _ := ownerFixture(t)
	boom := errors.New("record store down")
	s.ReadRecord = func(string) (model.IssueOpsRecord, error) { return model.IssueOpsRecord{}, boom }
	if _, err := s.Reseed(t.Context(), "io-owner", model.Execution{Lease: model.WriteLease{Generation: 2}}); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

func TestReseedRejectsRecordWhoseGenerationIsNotThePredecessor(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r = orcaRecord(t, f, r)
	r.Execution.Lease.Generation = 5
	s.ReadRecord = func(string) (model.IssueOpsRecord, error) { return r, nil }
	_, err := s.Reseed(t.Context(), r.ID, model.Execution{Mode: model.ExecutionModeOrca, Orca: r.Execution.Orca, Lease: model.WriteLease{Generation: 2}})
	if err == nil || !strings.Contains(err.Error(), "stale lease generation") || len(f.writes) != 0 {
		t.Fatalf("err=%v", err)
	}
}

func TestReseedRequiresOrcaOnBothRecordAndReplacement(t *testing.T) {
	for name, mutate := range map[string]func(*model.IssueOpsRecord, *model.Execution){
		"record is direct":       func(r *model.IssueOpsRecord, _ *model.Execution) { r.Execution.Mode = model.ExecutionModeDirect },
		"record lacks binding":   func(r *model.IssueOpsRecord, _ *model.Execution) { r.Execution.Orca = nil },
		"replacement is direct":  func(_ *model.IssueOpsRecord, e *model.Execution) { e.Mode = model.ExecutionModeDirect },
		"replacement lacks Orca": func(_ *model.IssueOpsRecord, e *model.Execution) { e.Orca = nil },
	} {
		t.Run(name, func(t *testing.T) {
			s, f, r, _, _, _ := ownerFixture(t)
			r = orcaRecord(t, f, r)
			next := model.Execution{Mode: model.ExecutionModeOrca, Orca: r.Execution.Orca, Lease: model.WriteLease{Generation: 2}}
			mutate(&r, &next)
			s.ReadRecord = func(string) (model.IssueOpsRecord, error) { return r, nil }
			_, err := s.Reseed(t.Context(), r.ID, next)
			if err == nil || !strings.Contains(err.Error(), "require an Orca execution") || len(f.writes) != 0 {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestReseedSealsArtifactsForTheReplacementGeneration(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r = orcaRecord(t, f, r)
	s.ReadRecord = func(string) (model.IssueOpsRecord, error) { return r, nil }
	next := r.Execution
	replacement := *next
	replacement.Lease.Generation = 2
	got, err := s.Reseed(t.Context(), r.ID, replacement)
	if err != nil {
		t.Fatal(err)
	}
	var packet model.OwnerContextPacket
	if err := json.Unmarshal(f.writes["/wt/packet.json"], &packet); err != nil {
		t.Fatal(err)
	}
	if packet.LeaseGeneration != 2 || got.ContextPacketSHA256 != digest(f.writes["/wt/packet.json"]) {
		t.Fatalf("packet generation=%d receipt=%+v", packet.LeaseGeneration, got)
	}
}

func TestReadSnapshotRejectsUnusableIssueLinks(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*Service, *model.IssueOpsRecord)
		want   string
	}{
		"missing reader": {func(s *Service, _ *model.IssueOpsRecord) { s.ReadIssue = nil }, "reader is unavailable"},
		"no provider":    {func(_ *Service, r *model.IssueOpsRecord) { r.IssueURL = "https://example.com/issues/1" }, "linked GitHub or GitLab issue is required"},
		"blank url":      {func(_ *Service, r *model.IssueOpsRecord) { r.IssueURL = " " }, "linked GitHub or GitLab issue is required"},
	} {
		t.Run(name, func(t *testing.T) {
			s, _, r, _, _, _ := ownerFixture(t)
			tc.mutate(&s, &r)
			if _, err := s.ReadSnapshot(t.Context(), r); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
	}
}

func TestReadSnapshotWrapsRemoteFailureAndValidatesBody(t *testing.T) {
	s, _, r, _, _, _ := ownerFixture(t)
	boom := errors.New("403")
	reader := s.ReadIssue
	s.ReadIssue = func(context.Context, string, remote.ExecutionIssueSnapshotRequest) (remote.ExecutionIssueSnapshot, error) {
		return remote.ExecutionIssueSnapshot{}, boom
	}
	if _, err := s.ReadSnapshot(t.Context(), r); !errors.Is(err, boom) || !strings.Contains(err.Error(), "read remote issue snapshot") {
		t.Fatalf("err=%v", err)
	}
	for name, snap := range map[string]remote.ExecutionIssueSnapshot{
		"other issue":     {URL: "https://github.com/acme/repo/issues/999", Body: "AC-01: x\n```sh\ngo test\n```\n"},
		"no acceptance":   {URL: r.IssueURL, Body: "## Verification\n```sh\ngo test ./...\n```\n"},
		"no verification": {URL: r.IssueURL, Body: "AC-01: no commands here"},
	} {
		t.Run(name, func(t *testing.T) {
			s.ReadIssue = func(context.Context, string, remote.ExecutionIssueSnapshotRequest) (remote.ExecutionIssueSnapshot, error) {
				return snap, nil
			}
			_, err := s.ReadSnapshot(t.Context(), r)
			if err == nil {
				t.Fatal("invalid remote issue must be rejected")
			}
			if name == "no acceptance" && !strings.Contains(err.Error(), "acceptance IDs") {
				t.Fatalf("acceptance check must be the rejecting reason: %v", err)
			}
		})
	}
	s.ReadIssue = reader
	snapshot, err := s.ReadSnapshot(t.Context(), r)
	if err != nil || len(snapshot.AcceptanceIDs) != 1 || len(snapshot.VerificationCommands) != 1 || snapshot.Issue.BodySHA256 != digest([]byte(snapshot.Issue.Body)) {
		t.Fatalf("valid snapshot: %+v %v", snapshot, err)
	}
}

func TestBuildRejectsMissingExecutionIdentity(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r.Execution.Lease.Generation = 0
	if _, err := s.Build(r, model.ExecutionPrepareRequest{}, model.OwnerSnapshot{}, nil); err == nil || !strings.Contains(err.Error(), "execution identity is unavailable") {
		t.Fatalf("generation 0: %v", err)
	}
	r.Execution = nil
	if _, err := s.Build(r, model.ExecutionPrepareRequest{}, model.OwnerSnapshot{}, nil); err == nil || !strings.Contains(err.Error(), "execution identity is unavailable") {
		t.Fatalf("nil execution: %v", err)
	}
	if len(f.writes) != 0 {
		t.Fatal("nothing may be written without an execution identity")
	}
}

func TestBuildFailsWhenModelResolverMissingAndWritesNothing(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	s.ResolveModel = nil
	if _, err := s.Build(r, model.ExecutionPrepareRequest{OwnerHost: "codex"}, model.OwnerSnapshot{}, nil); err == nil || !strings.Contains(err.Error(), "resolver is not configured") || len(f.writes) != 0 {
		t.Fatalf("err=%v", err)
	}
}

func TestBuildPropagatesPromptWriteFailureAfterPacketWrite(t *testing.T) {
	s, f, r, _, _, _ := ownerFixture(t)
	r = orcaRecord(t, f, r)
	boom := errors.New("prompt disk full")
	s.Files = &promptFailFiles{ownerFiles: f, err: boom}
	snapshot, err := s.ReadSnapshot(t.Context(), r)
	if err != nil {
		t.Fatal(err)
	}
	f.events = nil
	artifacts, err := s.Build(r, model.ExecutionPrepareRequest{ID: r.ID, OwnerHost: "codex", OwnerModel: "gpt-6-sol", OwnerEffort: "high"}, snapshot, map[string]string{"plan": digest([]byte(f.plan))})
	if !errors.Is(err, boom) || artifacts != (model.OwnerArtifacts{}) {
		t.Fatalf("artifacts=%+v err=%v", artifacts, err)
	}
	if !slices.Equal(f.events, []string{"write:/wt/packet.json"}) {
		t.Fatalf("packet must be written before the prompt attempt: %v", f.events)
	}
}

func TestReadPreparationEvidenceProjectsIssueIdentityAndBody(t *testing.T) {
	s, _, r, snapshot, intent, _ := ownerFixture(t)
	snapshot.Record.IssueURL = r.IssueURL
	snapshot.Record.BranchPrepare = json.RawMessage(`{"provider":"github","issue_url":"` + r.IssueURL + `","link_verified":true}`)
	got, err := s.ReadPreparationEvidence(t.Context(), snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if got.IssueURL != r.IssueURL || got.Provider != "github" || got.Issue != 480 || got.BodySHA256 != intent.IssueBodySHA256 || got.IssueBody == "" {
		t.Fatalf("evidence=%+v", got)
	}
}

func TestReadPreparationEvidenceRejectsUnverifiableIssueLink(t *testing.T) {
	s, _, r, snapshot, _, _ := ownerFixture(t)
	snapshot.Record.IssueURL = r.IssueURL
	// no verified branch-prepare evidence at all
	if _, err := s.ReadPreparationEvidence(t.Context(), snapshot); err == nil {
		t.Fatal("missing branch issue identity must be rejected")
	}
	snapshot.Record.IssueURL = "https://github.com/acme/repo/issues/1"
	snapshot.Record.BranchPrepare = json.RawMessage(`{"provider":"github","issue_url":"` + r.IssueURL + `","link_verified":true}`)
	if _, err := s.ReadPreparationEvidence(t.Context(), snapshot); err == nil {
		t.Fatal("record/branch issue URL mismatch must be rejected")
	}
}

func TestReadPreparationEvidenceRequiresRawRecordAndRemoteIssue(t *testing.T) {
	s, _, _, snapshot, _, _ := ownerFixture(t)
	if _, err := s.ReadPreparationEvidence(t.Context(), prep.Snapshot{}); err == nil || !strings.Contains(err.Error(), "raw record snapshot is required") {
		t.Fatalf("err=%v", err)
	}
	if _, err := s.ReadPreparationEvidence(t.Context(), prep.Snapshot{RecordRaw: []byte("{")}); err == nil {
		t.Fatal("corrupt raw record must be rejected")
	}
	boom := errors.New("remote 500")
	s.ReadIssue = func(context.Context, string, remote.ExecutionIssueSnapshotRequest) (remote.ExecutionIssueSnapshot, error) {
		return remote.ExecutionIssueSnapshot{}, boom
	}
	if _, err := s.ReadPreparationEvidence(t.Context(), snapshot); !errors.Is(err, boom) {
		t.Fatalf("remote failure: %v", err)
	}
}

func TestPrepareRejectsCandidateThatDoesNotMatchSealedIntent(t *testing.T) {
	for name, mutate := range map[string]func(*prep.IntentReceipt){
		"no workspace receipt": func(r *prep.IntentReceipt) { r.Workspace = nil },
		"source differs":       func(r *prep.IntentReceipt) { r.Workspace.Workspace.SourceRoot = "/other" },
		"root differs":         func(r *prep.IntentReceipt) { r.Workspace.Workspace.Root = "/other-wt" },
		"branch differs":       func(r *prep.IntentReceipt) { r.Workspace.Workspace.Branch = "other" },
		"base head differs":    func(r *prep.IntentReceipt) { r.Workspace.Workspace.BaseHead = "def" },
		"driver is not orca":   func(r *prep.IntentReceipt) { r.Workspace.Workspace.Driver = "git" },
		"runtime id missing":   func(r *prep.IntentReceipt) { r.Workspace.RuntimeID = " " },
		"repo id missing":      func(r *prep.IntentReceipt) { r.Workspace.RepoID = "" },
		"worktree id missing":  func(r *prep.IntentReceipt) { r.Workspace.WorktreeID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			s, f, _, snapshot, intent, receipt := ownerFixture(t)
			mutate(&receipt)
			_, err := s.Prepare(t.Context(), snapshot, prep.Command{}, intent, receipt)
			if err == nil || !strings.Contains(err.Error(), "does not match the sealed intent") || len(f.events) != 0 {
				t.Fatalf("err=%v events=%v", err, f.events)
			}
		})
	}
}

func TestPrepareRequiresRawRecordAfterWorkspaceMatches(t *testing.T) {
	s, f, _, _, intent, receipt := ownerFixture(t)
	_, err := s.Prepare(t.Context(), prep.Snapshot{}, prep.Command{}, intent, receipt)
	if err == nil || !strings.Contains(err.Error(), "raw record snapshot is required") || len(f.events) != 0 {
		t.Fatalf("err=%v events=%v", err, f.events)
	}
}

func TestPreparePropagatesTokenFailureBeforePlanMaterialization(t *testing.T) {
	s, f, _, snapshot, intent, receipt := ownerFixture(t)
	s.Files = &tokenFailFiles{ownerFiles: f, err: errors.New("token exists with different owner")}
	_, err := s.Prepare(t.Context(), snapshot, prep.Command{}, intent, receipt)
	if err == nil || !strings.Contains(err.Error(), "token exists") {
		t.Fatalf("err=%v", err)
	}
	for _, e := range f.events {
		if e == "materialize" || strings.HasPrefix(e, "write:") {
			t.Fatalf("token failure must stop materialization/writes: %v", f.events)
		}
	}
}

type tokenFailFiles struct {
	*ownerFiles
	err error
}

func (f *tokenFailFiles) CreateOrAdoptToken(model.IssueOpsRecord) (string, error) { return "", f.err }

func TestOwnerArtifactDirFallsBackToBranchPrepareIssue(t *testing.T) {
	for name, tc := range map[string]struct {
		record model.IssueOpsRecord
		want   string
	}{
		"record issue wins": {model.IssueOpsRecord{IssueURL: "https://github.com/a/b/issues/7", BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "https://github.com/a/b/issues/8"}}, ".issueops/issues/7/artifact"},
		"branch issue":      {model.IssueOpsRecord{IssueURL: "not-an-issue", BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "https://github.com/a/b/issues/8"}}, ".issueops/issues/8/artifact"},
		"neither":           {model.IssueOpsRecord{IssueURL: "not-an-issue"}, ""},
		"branch unnumbered": {model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "nope"}}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			if got := OwnerArtifactDir(tc.record); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

func TestPolicyContextCarriesIssueIdentityFromBranchPrepare(t *testing.T) {
	r := model.IssueOpsRecord{BranchPrepare: &model.IssueOpsBranchPrepare{IssueURL: "https://github.com/acme/repo/issues/42"}}
	policy, err := PolicyContext(r, model.ExecutionPrepareRequest{OwnerHost: "codex"}, fixedRoleModel)
	if err != nil || policy.IssueNumber != "42" || policy.ProjectKey == "" {
		t.Fatalf("policy=%+v err=%v", policy, err)
	}
	bare, err := PolicyContext(model.IssueOpsRecord{}, model.ExecutionPrepareRequest{OwnerHost: "codex"}, fixedRoleModel)
	if err != nil || bare.IssueNumber != "" || bare.ProjectKey != "" {
		t.Fatalf("no branch prepare means no project identity: %+v %v", bare, err)
	}
}

func TestWriterlessCommandIsEmptyWithoutExecution(t *testing.T) {
	if got := WriterlessCommand(model.IssueOpsRecord{ID: "io-1"}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestWorkspaceHelpersRejectCorruptRawRecord(t *testing.T) {
	s, _, _, _, _, _ := ownerFixture(t)
	if _, err := s.ResolveWorkspace(prep.Snapshot{RecordRaw: []byte("{")}, true); err == nil {
		t.Fatal("ResolveWorkspace must reject corrupt raw record")
	}
	if err := s.MaterializeDirect(prep.Snapshot{}, prep.WorkspaceReceipt{}); err == nil {
		t.Fatal("MaterializeDirect must reject missing raw record")
	}
}

func TestMaterializeDirectPropagatesMaterializeFailure(t *testing.T) {
	s, f, _, snapshot, _, _ := ownerFixture(t)
	boom := errors.New("cannot copy")
	s.Files = &materializeFiles{ownerFiles: f, err: boom}
	if err := s.MaterializeDirect(snapshot, prep.WorkspaceReceipt{Root: "/wt"}); !errors.Is(err, boom) {
		t.Fatalf("err=%v", err)
	}
}

// ---- resume ----

func mutatePacket(t *testing.T, f *resumeFiles, r *model.IssueOpsRecord, mutate func(*model.OwnerContextPacket)) {
	t.Helper()
	var packet model.OwnerContextPacket
	if err := json.Unmarshal(f.data["/wt/packet.json"], &packet); err != nil {
		t.Fatal(err)
	}
	mutate(&packet)
	data, err := json.Marshal(packet)
	if err != nil {
		t.Fatal(err)
	}
	f.data["/wt/packet.json"] = data
	r.Execution.Orca.ContextPacketSHA256 = digest(data)
}

func TestResumeRejectsUnsealedOrDegenerateInputs(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*resumeFiles, *model.IssueOpsRecord)
		want   string
	}{
		"artifact dir missing": {func(_ *resumeFiles, r *model.IssueOpsRecord) { r.Execution.Workspace.ArtifactDir = " " }, domain.ErrSealedArtifactDirMissing.Error()},
		"token file missing":   {func(f *resumeFiles, _ *model.IssueOpsRecord) { delete(f.data, "/wt/token") }, "read current generation claim token"},
		"token oversized":      {func(f *resumeFiles, _ *model.IssueOpsRecord) { f.data["/wt/token"] = []byte(strings.Repeat("a", 257)) }, "oversized"},
		"token empty":          {func(f *resumeFiles, _ *model.IssueOpsRecord) { f.data["/wt/token"] = []byte(" \n") }, "empty"},
		"binding incomplete": {func(_ *resumeFiles, r *model.IssueOpsRecord) { r.Execution.Orca.ArtifactIdentityVersion = 0 },
			"sealed Orca artifact identity is not durable; run issueops execution replace --id 'io-owner' --expected-generation 1 --preview"},
		"packet unreadable": {func(f *resumeFiles, _ *model.IssueOpsRecord) { delete(f.data, "/wt/packet.json") }, "read sealed context packet"},
		"packet tampered":   {func(f *resumeFiles, _ *model.IssueOpsRecord) { f.data["/wt/packet.json"] = []byte("{}") }, "sealed context packet identity changed"},
		"prompt unreadable": {func(f *resumeFiles, _ *model.IssueOpsRecord) { delete(f.data, "/wt/prompt.md") }, "read sealed owner prompt"},
		"prompt tampered":   {func(f *resumeFiles, _ *model.IssueOpsRecord) { f.data["/wt/prompt.md"] = []byte("edited") }, "sealed owner prompt identity changed"},
		"issue body binding": {func(_ *resumeFiles, r *model.IssueOpsRecord) {
			r.Execution.Orca.IssueBodySHA256 = digest([]byte("other"))
		}, "sealed issue body identity changed"},
		"profile drift": {func(_ *resumeFiles, r *model.IssueOpsRecord) { r.Execution.Orca.OwnerModel = "another-model" }, "owner profile does not match"},
		"direct mode":   {func(_ *resumeFiles, r *model.IssueOpsRecord) { r.Execution.Mode = model.ExecutionModeDirect }, "no longer matches an Orca execution generation"},
	} {
		t.Run(name, func(t *testing.T) {
			reader, f, r := resumeFixture(t)
			tc.mutate(f, &r)
			_, err := reader.Read(r)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want %q", err, tc.want)
			}
		})
	}
}

func TestResumeRejectsPacketThatIsValidDigestButNotJSON(t *testing.T) {
	reader, f, r := resumeFixture(t)
	f.data["/wt/packet.json"] = []byte("not json")
	r.Execution.Orca.ContextPacketSHA256 = digest(f.data["/wt/packet.json"])
	if _, err := reader.Read(r); err == nil || !strings.Contains(err.Error(), "parse sealed context packet") {
		t.Fatalf("err=%v", err)
	}
}

func TestResumeRejectsPacketBoundToDifferentGenerationOrPaths(t *testing.T) {
	for name, mutate := range map[string]func(*model.OwnerContextPacket){
		"generation": func(p *model.OwnerContextPacket) { p.LeaseGeneration = 9 },
		"source":     func(p *model.OwnerContextPacket) { p.SourceRoot = "/elsewhere" },
		"worktree":   func(p *model.OwnerContextPacket) { p.WorktreeRoot = "/elsewhere" },
		"branch":     func(p *model.OwnerContextPacket) { p.Branch = "other" },
		"issue url":  func(p *model.OwnerContextPacket) { p.Issue.URL = "https://github.com/acme/repo/issues/1" },
	} {
		t.Run(name, func(t *testing.T) {
			reader, f, r := resumeFixture(t)
			mutatePacket(t, f, &r, mutate)
			if _, err := reader.Read(r); err == nil || !strings.Contains(err.Error(), "execution identity mismatch") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}

func TestResumeSealedPlanMustAgreeAcrossManifestSealedCopyAndDurablePlan(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*resumeFiles, *model.IssueOpsRecord)
		packet func(*model.OwnerContextPacket)
	}{
		"manifest lacks plan":     {packet: func(p *model.OwnerContextPacket) { delete(p.ArtifactManifest, "plan") }},
		"manifest digest invalid": {packet: func(p *model.OwnerContextPacket) { p.ArtifactManifest["plan"] = "xyz" }},
		"sealed copy changed":     {mutate: func(f *resumeFiles, _ *model.IssueOpsRecord) { f.data["/wt/plan.md"] = []byte("# edited\n") }},
		"sealed copy missing":     {mutate: func(f *resumeFiles, _ *model.IssueOpsRecord) { delete(f.data, "/wt/plan.md") }},
		"durable plan differs":    {mutate: func(f *resumeFiles, _ *model.IssueOpsRecord) { f.linked.Digest = digest([]byte("other")) }},
		"durable plan unreadable": {mutate: func(f *resumeFiles, _ *model.IssueOpsRecord) { f.linked = model.OwnerPlanIdentity{} }},
	} {
		t.Run(name, func(t *testing.T) {
			reader, f, r := resumeFixture(t)
			if tc.packet != nil {
				mutatePacket(t, f, &r, tc.packet)
			}
			if tc.mutate != nil {
				tc.mutate(f, &r)
			}
			_, err := reader.Read(r)
			required := requiredPlanError(t, err)
			if want := domain.ReplacementPreviewCommand("io-owner", 1); required.NextCommand != want {
				t.Fatalf("next command %q want %q", required.NextCommand, want)
			}
		})
	}
}

func TestResumeVerifiesEveryNonPlanManifestArtifact(t *testing.T) {
	extra := []byte("report body")
	setup := func(t *testing.T) (ResumeReader, *resumeFiles, model.IssueOpsRecord) {
		reader, f, r := resumeFixture(t)
		mutatePacket(t, f, &r, func(p *model.OwnerContextPacket) { p.ArtifactManifest["report"] = digest(extra) })
		return reader, f, r
	}
	reader, f, r := setup(t)
	f.data["/wt/report.md"] = extra
	if _, err := reader.Read(r); err != nil {
		t.Fatalf("matching extra artifact must resume: %v", err)
	}
	reader, f, r = setup(t)
	f.data["/wt/report.md"] = []byte("tampered")
	if _, err := reader.Read(r); err == nil || !strings.Contains(err.Error(), "sealed artifact report digest mismatch") {
		t.Fatalf("tampered artifact: %v", err)
	}
	reader, _, r = setup(t)
	if _, err := reader.Read(r); err == nil || !strings.Contains(err.Error(), "read sealed artifact report") {
		t.Fatalf("missing artifact: %v", err)
	}
}
