package selfaugment

import (
	"slices"
	"strings"
	"testing"
	"time"

	contract "issueops/internal/contract/selfaugment"
	selfverify "issueops/internal/contract/selfverify"
)

func TestMarkSatisfiedCandidateRequiresItsSignals(t *testing.T) {
	cases := []struct {
		id  string
		set func(*contract.SelfAugmentRepoSignals)
		// partial leaves a required signal unset for compound rules.
		partial func(*contract.SelfAugmentRepoSignals)
	}{
		{"loop-taxonomy-score-gates", func(s *contract.SelfAugmentRepoSignals) {
			s.HasSelfVerifyCLI, s.HasSelfAugmentPlanner, s.HasSelfVerificationDocs, s.HasGoalScoreSummary = true, true, true, true
		}, func(s *contract.SelfAugmentRepoSignals) {
			s.HasSelfVerifyCLI, s.HasSelfAugmentPlanner, s.HasSelfVerificationDocs = true, true, true
		}},
		{"agent-skill-executor", func(s *contract.SelfAugmentRepoSignals) { s.HasSelfAugmentSkill = true }, nil},
		{"durable-augmentation-memory", func(s *contract.SelfAugmentRepoSignals) { s.HasSelfAugmentStateCapture = true }, nil},
		{"reflexion-state-memory", func(s *contract.SelfAugmentRepoSignals) { s.HasSelfAugmentLessonCapture = true }, nil},
		{"qa-dashboard-summary", func(s *contract.SelfAugmentRepoSignals) { s.HasGoalScoreSummary = true }, nil},
		{"qa-race-tier", func(s *contract.SelfAugmentRepoSignals) { s.HasRiskQATier = true }, nil},
		{"adapter-contract-matrix", func(s *contract.SelfAugmentRepoSignals) { s.HasAdapterContractMatrix = true }, nil},
		{"repo-local-augmentation-sandbox", func(s *contract.SelfAugmentRepoSignals) { s.HasRepoLocalSandbox = true }, nil},
		{"performance-baseline", func(s *contract.SelfAugmentRepoSignals) { s.HasPerformanceBaseline = true }, nil},
		{"self-augment-signal-table", func(s *contract.SelfAugmentRepoSignals) { s.HasSelfAugmentSignalTable = true }, nil},
		{"quality-signal-harvester", func(s *contract.SelfAugmentRepoSignals) {
			s.HasQualityInspectCLI, s.HasQualityInspectSignals = true, true
		}, func(s *contract.SelfAugmentRepoSignals) { s.HasQualityInspectCLI = true }},
		{"coverage-mcp-resources", func(s *contract.SelfAugmentRepoSignals) { s.HasMCPResourceCoverage = true }, nil},
		{"coverage-host-judgement", func(s *contract.SelfAugmentRepoSignals) { s.HasHostJudgementCoverage = true }, nil},
		{"coverage-issueops-linking", func(s *contract.SelfAugmentRepoSignals) { s.HasIssueOpsLinkingBoundaryCoverage = true }, nil},
		{"coverage-issueops-inbound-adapters", func(s *contract.SelfAugmentRepoSignals) { s.HasIssueOpsInboundAdapterCoverage = true }, nil},
		{"coverage-issueops-transport-boundaries", func(s *contract.SelfAugmentRepoSignals) { s.HasToolConformanceTransportCoverage = true }, nil},
		{"state-write-locking", func(s *contract.SelfAugmentRepoSignals) { s.HasStateWriteLocking = true }, nil},
		{"worker-stuck-running-detection", func(s *contract.SelfAugmentRepoSignals) { s.HasWorkerStuckRunningDetection = true }, nil},
		{"genius-mermaid-lint", func(s *contract.SelfAugmentRepoSignals) { s.HasGeniusMermaidLint = true }, nil},
		{"install-dry-run-mode", func(s *contract.SelfAugmentRepoSignals) { s.HasInstallDryRunMode = true }, nil},
		{"cli-mcp-adapter-split", func(s *contract.SelfAugmentRepoSignals) {
			s.HasCLIAdapterSplit, s.HasMCPAdapterCatalog = true, true
		}, func(s *contract.SelfAugmentRepoSignals) { s.HasMCPAdapterCatalog = true }},
		{"dto-compatibility-contract", func(s *contract.SelfAugmentRepoSignals) { s.HasCompatibilityContract = true }, nil},
		{"candidate-refill-curriculum", func(s *contract.SelfAugmentRepoSignals) { s.HasCandidateRefill = true }, nil},
		{"policy-audit-redaction", func(s *contract.SelfAugmentRepoSignals) { s.HasCommandAuditLog = true }, nil},
		{"worker-mvp-no-shell", func(s *contract.SelfAugmentRepoSignals) { s.HasWorkerMVP = true }, nil},
		{"release-repro-pack", func(s *contract.SelfAugmentRepoSignals) { s.HasReleaseReproPack = true }, nil},
		{"release-user-readme", func(s *contract.SelfAugmentRepoSignals) { s.HasReleaseUserReadme = true }, nil},
		{"cross-platform-build-matrix", func(s *contract.SelfAugmentRepoSignals) { s.HasCrossPlatformBuildMatrix = true }, nil},
		{"distribution-decision-record", func(s *contract.SelfAugmentRepoSignals) { s.HasDistributionDecision = true }, nil},
		{"release-dogfood-notes", func(s *contract.SelfAugmentRepoSignals) { s.HasReleaseDogfoodNotes = true }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.id, func(t *testing.T) {
			open := func() contract.SelfAugmentCandidate {
				return contract.SelfAugmentCandidate{ID: tc.id, Status: contract.CandidateStatusOpen, Score: 50, WhyNow: []string{"why"}}
			}
			none := open()
			MarkSatisfiedCandidate(&none, contract.SelfAugmentRepoSignals{})
			if none.Status != contract.CandidateStatusOpen || none.Score != 50 || len(none.WhyNow) != 1 {
				t.Fatalf("no signals must leave candidate untouched: %+v", none)
			}
			if tc.partial != nil {
				var partial contract.SelfAugmentRepoSignals
				tc.partial(&partial)
				c := open()
				MarkSatisfiedCandidate(&c, partial)
				if c.Status != contract.CandidateStatusOpen {
					t.Fatalf("partial signals must not satisfy: %+v", c)
				}
			}
			var full contract.SelfAugmentRepoSignals
			tc.set(&full)
			c := open()
			MarkSatisfiedCandidate(&c, full)
			if c.Status != contract.CandidateStatusSatisfied || c.Score != 0 || len(c.SatisfactionEvidence) == 0 {
				t.Fatalf("signals must satisfy: %+v", c)
			}
			if len(c.WhyNow) != 2 || !strings.Contains(c.WhyNow[1], "Already satisfied") {
				t.Fatalf("WhyNow=%v", c.WhyNow)
			}
		})
	}
}

func TestMarkSatisfiedCandidateIgnoresUnknownID(t *testing.T) {
	c := contract.SelfAugmentCandidate{ID: "no-such-candidate", Status: contract.CandidateStatusOpen, Score: 7}
	MarkSatisfiedCandidate(&c, contract.SelfAugmentRepoSignals{HasSelfAugmentSkill: true, HasReleaseReproPack: true})
	if c.Status != contract.CandidateStatusOpen || c.Score != 7 {
		t.Fatalf("unknown id must stay open: %+v", c)
	}
}

func TestCandidateScoreWeightsAndClamps(t *testing.T) {
	if got := CandidateScore(contract.SelfAugmentCandidate{Impact: 100, Feasibility: 100, Novelty: 100, Risk: 0}); got != 100 {
		t.Fatalf("max score=%v", got)
	}
	if got := CandidateScore(contract.SelfAugmentCandidate{Impact: 200, Feasibility: 200, Novelty: 200, Risk: 0}); got != 100 {
		t.Fatalf("score above 100 must clamp, got %v", got)
	}
	if got := CandidateScore(contract.SelfAugmentCandidate{Impact: -100, Risk: 100}); got != 0 {
		t.Fatalf("negative score must clamp to 0, got %v", got)
	}
	got := CandidateScore(contract.SelfAugmentCandidate{Impact: 50, Feasibility: 0, Novelty: 0, Risk: 100})
	if got < 18.99 || got > 19.01 {
		t.Fatalf("impact weight 0.38 expected 19, got %v", got)
	}
	got = CandidateScore(contract.SelfAugmentCandidate{Risk: 0})
	if got < 11.99 || got > 12.01 {
		t.Fatalf("risk weight 0.12 expected 12, got %v", got)
	}
}

func TestCandidatesCatalogDefaultsAndSatisfaction(t *testing.T) {
	quality := contract.SelfAugmentCandidate{ID: "quality-extra", Title: "q", Impact: 100, Feasibility: 100, Novelty: 100}
	open := Candidates(contract.SelfAugmentRepoSignals{}, []contract.SelfAugmentCandidate{quality})
	seen := map[string]bool{}
	for _, c := range open {
		if seen[c.ID] {
			t.Fatalf("duplicate candidate id %q", c.ID)
		}
		seen[c.ID] = true
		if c.Status != contract.CandidateStatusOpen {
			t.Fatalf("%s must be open without signals: %q", c.ID, c.Status)
		}
		if c.VerificationKind == "" {
			t.Fatalf("%s has no verification kind", c.ID)
		}
		if c.Score != CandidateScore(c) || c.Score <= 0 {
			t.Fatalf("%s score=%v", c.ID, c.Score)
		}
	}
	if !seen["quality-extra"] {
		t.Fatal("quality candidates must be appended")
	}
	byID := map[string]contract.SelfAugmentCandidate{}
	for _, c := range open {
		byID[c.ID] = c
	}
	if byID["quality-extra"].VerificationKind != contract.VerificationToolSignal || byID["release-user-readme"].VerificationKind != contract.VerificationDocArtifact {
		t.Fatalf("kinds: quality=%q readme=%q", byID["quality-extra"].VerificationKind, byID["release-user-readme"].VerificationKind)
	}

	satisfied := Candidates(contract.SelfAugmentRepoSignals{HasSelfAugmentSkill: true}, nil)
	for _, c := range satisfied {
		want := contract.CandidateStatusOpen
		if c.ID == "agent-skill-executor" {
			want = contract.CandidateStatusSatisfied
		}
		if c.Status != want {
			t.Fatalf("%s status=%q want %q", c.ID, c.Status, want)
		}
		if want == contract.CandidateStatusSatisfied && c.Score != 0 {
			t.Fatalf("satisfied score=%v", c.Score)
		}
	}
}

func TestSelectedCandidateID(t *testing.T) {
	if got := SelectedCandidateID(nil); got != "" {
		t.Fatalf("nil candidate id=%q", got)
	}
	if got := SelectedCandidateID(&contract.SelfAugmentCandidate{ID: "x"}); got != "x" {
		t.Fatalf("id=%q", got)
	}
}

func TestNewCandidateExportSnapshotCopiesResultFields(t *testing.T) {
	at := time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC)
	result := contract.SelfVerificationCandidateExportResult{
		OK: true, LoopKind: "k", KoreanName: "n", IssueOpsRoot: "/r", SourcePath: "/s", CandidateCount: 2,
		OpenCandidateIDs: []string{"a"}, SatisfiedCandidateIDs: []string{"b"},
		SelectedCandidate: &selfverify.SelfVerificationCandidate{ID: "a"},
		Candidates:        []selfverify.SelfVerificationCandidate{{ID: "a"}, {ID: "b"}},
	}
	snap := NewCandidateExportSnapshot(result, at)
	if snap.SchemaVersion != 1 || snap.Kind != contract.SelfVerificationCandidateExportKind || snap.GeneratedAt != "2026-01-02T03:04:05.000000006Z" {
		t.Fatalf("snapshot header=%+v", snap)
	}
	if !snap.OK || snap.LoopKind != "k" || snap.IssueOpsRoot != "/r" || snap.SourcePath != "/s" || snap.CandidateCount != 2 ||
		!slices.Equal(snap.OpenCandidateIDs, []string{"a"}) || !slices.Equal(snap.SatisfiedCandidateIDs, []string{"b"}) ||
		snap.SelectedCandidate == nil || snap.SelectedCandidate.ID != "a" || len(snap.Candidates) != 2 {
		t.Fatalf("snapshot=%+v", snap)
	}
}

func TestParseHistoryTimestamp(t *testing.T) {
	for _, bad := range []string{"", "   ", "yesterday", "2026-01-02"} {
		if _, ok := ParseHistoryTimestamp(bad); ok {
			t.Fatalf("%q must not parse", bad)
		}
	}
	nano, ok := ParseHistoryTimestamp("2026-01-02T03:04:05.5Z")
	if !ok || nano.Nanosecond() != 500_000_000 {
		t.Fatalf("nano=%v ok=%v", nano, ok)
	}
	plain, ok := ParseHistoryTimestamp("2026-01-02T03:04:05+09:00")
	if !ok || !plain.Equal(time.Date(2026, 1, 1, 18, 4, 5, 0, time.UTC)) {
		t.Fatalf("plain=%v ok=%v", plain, ok)
	}
}

func TestSortHistoryEntriesOrdersNewestFirstWithFallbacks(t *testing.T) {
	entries := []contract.SelfAugmentHistoryEntry{
		{Key: "invalid-gen-new-update", GeneratedAt: "bogus", UpdatedAt: "2026-05-01T00:00:00Z"},
		{Key: "old", GeneratedAt: "2026-01-01T00:00:00Z"},
		{Key: "tie-b", GeneratedAt: "2026-03-01T00:00:00Z", UpdatedAt: "2026-03-02T00:00:00Z"},
		{Key: "tie-a", GeneratedAt: "2026-03-01T00:00:00Z", UpdatedAt: "2026-03-02T00:00:00Z"},
		{Key: "tie-newer-update", GeneratedAt: "2026-03-01T00:00:00Z", UpdatedAt: "2026-03-09T00:00:00Z"},
		{Key: "tie-no-update", GeneratedAt: "2026-03-01T00:00:00Z"},
		{Key: "invalid-gen-old-update", GeneratedAt: "", UpdatedAt: "2026-02-01T00:00:00Z"},
		{Key: "newest", GeneratedAt: "2026-04-01T00:00:00Z"},
	}
	SortHistoryEntries(entries)
	var got []string
	for _, e := range entries {
		got = append(got, e.Key)
	}
	want := []string{"newest", "tie-newer-update", "tie-a", "tie-b", "tie-no-update", "old", "invalid-gen-new-update", "invalid-gen-old-update"}
	if !slices.Equal(got, want) {
		t.Fatalf("order=%v want %v", got, want)
	}
}

func TestValidateHistoryRequest(t *testing.T) {
	if err := ValidateHistoryRequest(-1, contract.SelfAugmentHistoryRetentionOptions{}); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Fatalf("negative limit err=%v", err)
	}
	if err := ValidateHistoryRequest(5, contract.SelfAugmentHistoryRetentionOptions{Confirm: true}); err == nil || !strings.Contains(err.Error(), "prune-retention") {
		t.Fatalf("confirm without prune err=%v", err)
	}
	if err := ValidateHistoryRequest(0, contract.SelfAugmentHistoryRetentionOptions{Limit: 3, PruneRequested: true}); err != nil {
		t.Fatalf("valid request err=%v", err)
	}
}

func TestValidateComparisonRequest(t *testing.T) {
	cases := []struct {
		base, cand string
		pct        float64
		want       string
	}{
		{" ", "c", 0, "baseline-key"},
		{"b", "", 0, "candidate-key"},
		{"b", "c", -0.1, "non-negative"},
		{"b", "c", 0, ""},
	}
	for _, tc := range cases {
		err := ValidateComparisonRequest(tc.base, tc.cand, tc.pct)
		if tc.want == "" {
			if err != nil {
				t.Fatalf("%+v err=%v", tc, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%+v err=%v want %q", tc, err, tc.want)
		}
	}
}

func TestHistorySnapshotDiagnostics(t *testing.T) {
	good := contract.SelfAugmentStateSnapshot{Kind: SelfVerificationSummaryKind, SchemaVersion: 1, GeneratedAt: "2026-01-01T00:00:00Z"}
	if skip, warn := HistorySnapshotDiagnostics("k", good); skip != "" || warn != "" {
		t.Fatalf("good skip=%q warn=%q", skip, warn)
	}
	wrongKind := good
	wrongKind.Kind = "other"
	if skip, _ := HistorySnapshotDiagnostics("k", wrongKind); skip != "kind:other" {
		t.Fatalf("skip=%q", skip)
	}
	wrongSchema := good
	wrongSchema.SchemaVersion = 2
	if skip, _ := HistorySnapshotDiagnostics("k", wrongSchema); skip != "schema:2" {
		t.Fatalf("skip=%q", skip)
	}
	badTime := good
	badTime.GeneratedAt = "nope"
	if skip, warn := HistorySnapshotDiagnostics("k1", badTime); skip != "" || warn != "invalid_generated_at:k1" {
		t.Fatalf("skip=%q warn=%q", skip, warn)
	}
}

func TestNewHistoryEntryNormalizesNilCollections(t *testing.T) {
	entry := NewHistoryEntry(contract.SelfAugmentStateSnapshot{GeneratedAt: "g", OK: true, Iterations: 3, BaseSeed: 9, ElapsedMS: 12,
		Summary: contract.SelfAugmentSummary{TotalRuns: 1, TotalSteps: 4, FailedSteps: 2}})
	if entry.StepLabels == nil || len(entry.StepLabels) != 0 || entry.SlowestSteps == nil || len(entry.SlowestSteps) != 0 {
		t.Fatalf("nil collections must become empty: %+v", entry)
	}
	if entry.GeneratedAt != "g" || !entry.OK || entry.Iterations != 3 || entry.BaseSeed != 9 || entry.ElapsedMS != 12 || entry.TotalRuns != 1 || entry.TotalSteps != 4 || entry.FailedSteps != 2 {
		t.Fatalf("entry=%+v", entry)
	}
	withData := NewHistoryEntry(contract.SelfAugmentStateSnapshot{Summary: contract.SelfAugmentSummary{
		StepLabels: []string{"a"}, SlowestSteps: []contract.SelfAugmentSlowStep{{Label: "a", DurationMS: 1}}}})
	if !slices.Equal(withData.StepLabels, []string{"a"}) || len(withData.SlowestSteps) != 1 {
		t.Fatalf("entry=%+v", withData)
	}
}

func TestDeletedHistoryRecommendation(t *testing.T) {
	if got := DeletedHistoryRecommendation(3, 10); got != "deleted 3 history checkpoint(s) beyond retention-limit=10" {
		t.Fatalf("got %q", got)
	}
}

func TestSevereLessonSeverity(t *testing.T) {
	for _, s := range []string{"error", " Major ", "CRITICAL", "blocker"} {
		if !SevereLessonSeverity(s) {
			t.Fatalf("%q must be severe", s)
		}
	}
	for _, s := range []string{"", "warning", "minor", "info"} {
		if SevereLessonSeverity(s) {
			t.Fatalf("%q must not be severe", s)
		}
	}
}

func TestRecentLessonWindowBoundaries(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	cases := map[string]bool{
		now.Add(-29 * 24 * time.Hour).Format(time.RFC3339Nano):           true,
		now.Add(-30 * 24 * time.Hour).Format(time.RFC3339Nano):           true,
		now.Add(-30*24*time.Hour - time.Second).Format(time.RFC3339Nano): false,
		now.Add(time.Hour).Format(time.RFC3339Nano):                      true,
		"  " + now.Add(-time.Hour).Format(time.RFC3339Nano) + "  ":       true,
		"not-a-time": false,
		"":           false,
	}
	for in, want := range cases {
		if got := RecentLesson(in, now); got != want {
			t.Fatalf("RecentLesson(%q)=%v want %v", in, got, want)
		}
	}
}

func TestLessonPenalizesCandidate(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	base := contract.SelfAugmentLessonStateSnapshot{
		Kind: contract.SelfAugmentationLessonKind, CandidateID: "c", Severity: "error", GeneratedAt: now.Add(-time.Hour).Format(time.RFC3339Nano),
	}
	if !LessonPenalizesCandidate(base, now) {
		t.Fatal("severe recent lesson must penalize")
	}
	mutations := map[string]func(*contract.SelfAugmentLessonStateSnapshot){
		"kind":     func(s *contract.SelfAugmentLessonStateSnapshot) { s.Kind = "other" },
		"no id":    func(s *contract.SelfAugmentLessonStateSnapshot) { s.CandidateID = "" },
		"severity": func(s *contract.SelfAugmentLessonStateSnapshot) { s.Severity = "info" },
		"stale": func(s *contract.SelfAugmentLessonStateSnapshot) {
			s.GeneratedAt = now.Add(-90 * 24 * time.Hour).Format(time.RFC3339Nano)
		},
	}
	for name, mutate := range mutations {
		s := base
		mutate(&s)
		if LessonPenalizesCandidate(s, now) {
			t.Fatalf("%s must not penalize", name)
		}
	}
}

func TestApplyLessonPenaltiesOnlyDemotesOpenCandidatesAtThreshold(t *testing.T) {
	candidates := []contract.SelfAugmentCandidate{
		{ID: "below", Status: contract.CandidateStatusOpen, Score: 90},
		{ID: "hit", Status: contract.CandidateStatusOpen, Score: 90},
		{ID: "floor", Status: contract.CandidateStatusOpen, Score: 20},
		{ID: "done", Status: contract.CandidateStatusSatisfied, Score: 90},
	}
	warnings := ApplyLessonPenalties(candidates, map[string]int{"below": 1, "hit": 2, "floor": 3, "done": 5})
	if candidates[0].Score != 90 || candidates[3].Score != 90 {
		t.Fatalf("untouched scores changed: %+v", candidates)
	}
	if candidates[1].Score != 60 || candidates[2].Score != 0 {
		t.Fatalf("hit=%v floor=%v", candidates[1].Score, candidates[2].Score)
	}
	if len(warnings) != 2 || !strings.Contains(warnings[0], `"hit" score 90.0 -> 60.0 after 2`) || !strings.Contains(warnings[1], "20.0 -> 0.0") {
		t.Fatalf("warnings=%v", warnings)
	}
	if got := ApplyLessonPenalties(nil, nil); got == nil || len(got) != 0 {
		t.Fatalf("empty warnings must be non-nil empty: %v", got)
	}
}

func TestPreparePromotionValidatesKeysAndDryRunFlag(t *testing.T) {
	if _, err := PreparePromotion(" ", "b", true, "/d"); err == nil || !strings.Contains(err.Error(), "from-key") {
		t.Fatalf("err=%v", err)
	}
	if _, err := PreparePromotion("f", "", true, "/d"); err == nil || !strings.Contains(err.Error(), "baseline-key") {
		t.Fatalf("err=%v", err)
	}
	dry, err := PreparePromotion("f", "b", false, "/d")
	if err != nil || !dry.DryRun || dry.Confirm || dry.OK || dry.StateDir != "/d" || dry.FromKey != "f" || dry.BaselineKey != "b" {
		t.Fatalf("dry=%+v err=%v", dry, err)
	}
	live, err := PreparePromotion("f", "b", true, "/d")
	if err != nil || live.DryRun || !live.Confirm || live.OK {
		t.Fatalf("live=%+v err=%v", live, err)
	}
}

func TestObservePromotionSourceGate(t *testing.T) {
	passing := contract.SelfAugmentStateSnapshot{OK: true, GeneratedAt: "g", Summary: contract.SelfAugmentSummary{TerminationEligible: true, TotalRuns: 4}}
	failing := contract.SelfAugmentStateSnapshot{OK: true, GeneratedAt: "g2", Summary: contract.SelfAugmentSummary{TerminationEligible: false}}

	dry, _ := PreparePromotion("f", "b", false, "/d")
	got, err := ObservePromotionSource(dry, failing, false)
	if err != nil || !got.OK || got.SourcePassed || got.SnapshotGeneratedAt != "g2" {
		t.Fatalf("dry-run must report without refusing: %+v err=%v", got, err)
	}

	live, _ := PreparePromotion("f", "b", true, "/d")
	got, err = ObservePromotionSource(live, failing, false)
	if err == nil || !strings.Contains(err.Error(), "refusing to promote") || got.OK || got.SourcePassed {
		t.Fatalf("failed source must be refused: %+v err=%v", got, err)
	}
	got, err = ObservePromotionSource(live, failing, true)
	if err != nil || got.SourcePassed {
		t.Fatalf("override: %+v err=%v", got, err)
	}
	got, err = ObservePromotionSource(live, passing, false)
	if err != nil || !got.SourcePassed || got.Summary.TotalRuns != 4 {
		t.Fatalf("passing: %+v err=%v", got, err)
	}
	notOK := passing
	notOK.OK = false
	if _, err := ObservePromotionSource(live, notOK, false); err == nil {
		t.Fatal("snapshot with ok=false must be refused")
	}
}

func TestIsSelfVerificationSummaryKind(t *testing.T) {
	if !IsSelfVerificationSummaryKind(SelfVerificationSummaryKind) || IsSelfVerificationSummaryKind("") || IsSelfVerificationSummaryKind("self_verification") {
		t.Fatal("kind check wrong")
	}
}

func TestSelectGeniusFormulas(t *testing.T) {
	if got := SelectGeniusFormulas("  \n"); got == nil || len(got) != 0 {
		t.Fatalf("blank text must give empty non-nil: %v", got)
	}
	if got := SelectGeniusFormulas("nothing relevant"); got == nil || len(got) != 0 {
		t.Fatalf("no match must give empty non-nil: %v", got)
	}
	got := SelectGeniusFormulas("복잡성 해결 매트릭스 and 문제 재정의 알고리즘")
	if !slices.Equal(got, []string{"문제 재정의 알고리즘", "복잡성 해결 매트릭스"}) {
		t.Fatalf("formulas must be in catalog order: %v", got)
	}
}

func TestStepDurationStatsForCompare(t *testing.T) {
	stored := contract.SelfAugmentSummary{
		StepDurationStats: []contract.SelfAugmentStepDurationStat{{Label: "a", P95DurationMS: 5}},
		SlowestSteps:      []contract.SelfAugmentSlowStep{{Label: "ignored", DurationMS: 99}},
	}
	got := StepDurationStatsForCompare(stored)
	if len(got) != 1 || got[0].Label != "a" {
		t.Fatalf("stored stats must win: %+v", got)
	}
	got[0].Label = "mutated"
	if stored.StepDurationStats[0].Label != "a" {
		t.Fatal("returned stats must be a copy")
	}

	derived := StepDurationStatsForCompare(contract.SelfAugmentSummary{SlowestSteps: []contract.SelfAugmentSlowStep{
		{Label: "b", DurationMS: 10}, {Label: "b", DurationMS: 30}, {Label: "", DurationMS: 500}, {Label: "a", DurationMS: 7},
	}})
	if len(derived) != 2 || derived[0].Label != "a" || derived[1].Label != "b" {
		t.Fatalf("derived=%+v", derived)
	}
	if derived[1].Count != 2 || derived[1].MinDurationMS != 10 || derived[1].MaxDurationMS != 30 || derived[1].AverageDurationMS != 20 || derived[1].P95DurationMS != 30 {
		t.Fatalf("b stats=%+v", derived[1])
	}
}

func TestStepDurationStatByLabelSkipsEmptyLabels(t *testing.T) {
	m := StepDurationStatByLabel([]contract.SelfAugmentStepDurationStat{{Label: "", Count: 1}, {Label: "x", Count: 2}, {Label: "x", Count: 3}})
	if len(m) != 1 || m["x"].Count != 3 {
		t.Fatalf("m=%+v", m)
	}
}

func TestFinalizeSummary(t *testing.T) {
	empty := contract.SelfAugmentSummary{}
	FinalizeSummary(&empty, true)
	if empty.MinimumGoalScore != 0 || !empty.TerminationEligible {
		t.Fatalf("empty goals: %+v", empty)
	}
	FinalizeSummary(&empty, false)
	if empty.TerminationEligible {
		t.Fatal("failed run must not be eligible")
	}

	passing := contract.SelfAugmentSummary{GoalScores: []selfverify.SelfVerificationGoalScore{{Score: 98, Passed: true}, {Score: 96, Passed: true}}}
	FinalizeSummary(&passing, true)
	if passing.MinimumGoalScore != 96 || !passing.TerminationEligible {
		t.Fatalf("passing: %+v", passing)
	}

	oneFailed := contract.SelfAugmentSummary{GoalScores: []selfverify.SelfVerificationGoalScore{{Score: 100, Passed: true}, {Score: 40, Passed: false}}}
	FinalizeSummary(&oneFailed, true)
	if oneFailed.MinimumGoalScore != 40 || oneFailed.TerminationEligible {
		t.Fatalf("one failed goal must block termination: %+v", oneFailed)
	}
}

func TestCompareSnapshotsSummaryRegressions(t *testing.T) {
	baseline := contract.SelfAugmentStateSnapshot{OK: true, ElapsedMS: 100, Summary: contract.SelfAugmentSummary{TerminationEligible: true, MinimumGoalScore: 98}}
	candidate := contract.SelfAugmentStateSnapshot{OK: false, ElapsedMS: 200, Summary: contract.SelfAugmentSummary{MinimumGoalScore: 90, FailedSteps: 2}}
	result := CompareSnapshots("b", "c", 10, baseline, candidate, "/d")
	for _, want := range []string{"candidate_not_ok", "candidate_not_termination_eligible", "minimum_goal_score_decreased_by_8.00", "failed_steps_increased_by_2", "elapsed_ms_increased_by_100.00_pct"} {
		if !slices.Contains(result.Regressions, want) {
			t.Fatalf("missing %q in %v", want, result.Regressions)
		}
	}
	if !result.Regressed || !result.OK {
		t.Fatalf("result=%+v", result)
	}

	steady := CompareSnapshots("b", "c", 10, baseline, baseline, "/d")
	if steady.Regressed || len(steady.Regressions) != 0 {
		t.Fatalf("identical snapshots must not regress: %v", steady.Regressions)
	}
}

func TestCompareStepRegressionsThresholds(t *testing.T) {
	slow := CompareSlowestStepRegressions(
		[]contract.SelfAugmentSlowStep{{Label: "a", DurationMS: 100}, {Label: "zero", DurationMS: 0}, {Label: "b", DurationMS: 100}, {Label: "c", DurationMS: 100}},
		[]contract.SelfAugmentSlowStep{{Label: "a", DurationMS: 150}, {Label: "zero", DurationMS: 50}, {Label: "b", DurationMS: 110}, {Label: "new", DurationMS: 999}, {Label: "c", DurationMS: 50}},
		20)
	if len(slow) != 1 || slow[0].Label != "a" || slow[0].DeltaMS != 50 || slow[0].DeltaPct != 50 {
		t.Fatalf("slow=%+v (zero baseline, new label, improvements and under-threshold must be ignored)", slow)
	}

	budget := CompareStepBudgetRegressions(
		[]contract.SelfAugmentStepDurationStat{{Label: "big", P95DurationMS: 100, Count: 2}, {Label: "tiny", P95DurationMS: 10}, {Label: "zero"}, {Label: "faster", P95DurationMS: 100}},
		[]contract.SelfAugmentStepDurationStat{{Label: "big", P95DurationMS: 200, Count: 3}, {Label: "tiny", P95DurationMS: 30}, {Label: "zero", P95DurationMS: 500}, {Label: "faster", P95DurationMS: 50}, {Label: "added", P95DurationMS: 500}},
		20)
	if len(budget) != 1 || budget[0].Label != "big" || budget[0].Metric != "p95_duration_ms" || budget[0].DeltaMS != 100 || budget[0].BaselineCount != 2 || budget[0].CandidateCount != 3 {
		t.Fatalf("budget=%+v (tiny delta below %dms floor must be ignored)", budget, stepBudgetMinRegressionMS)
	}

	ordered := CompareSlowestStepRegressions(
		[]contract.SelfAugmentSlowStep{{Label: "x", DurationMS: 100}, {Label: "y", DurationMS: 100}, {Label: "z", DurationMS: 100}},
		[]contract.SelfAugmentSlowStep{{Label: "x", DurationMS: 200}, {Label: "y", DurationMS: 300}, {Label: "z", DurationMS: 200}}, 0)
	if len(ordered) != 3 || ordered[0].Label != "y" || ordered[1].Label != "x" || ordered[2].Label != "z" {
		t.Fatalf("ordered=%+v", ordered)
	}
	budgetOrdered := CompareStepBudgetRegressions(
		[]contract.SelfAugmentStepDurationStat{{Label: "x", P95DurationMS: 100}, {Label: "y", P95DurationMS: 100}, {Label: "z", P95DurationMS: 100}},
		[]contract.SelfAugmentStepDurationStat{{Label: "x", P95DurationMS: 200}, {Label: "y", P95DurationMS: 300}, {Label: "z", P95DurationMS: 200}}, 0)
	if len(budgetOrdered) != 3 || budgetOrdered[0].Label != "y" || budgetOrdered[1].Label != "x" || budgetOrdered[2].Label != "z" {
		t.Fatalf("budgetOrdered=%+v", budgetOrdered)
	}
}

func TestValidateHistoryRetentionBoundaries(t *testing.T) {
	if err := ValidateHistoryRetention(-1, false, false); err == nil {
		t.Fatal("negative limit must fail")
	}
	if err := ValidateHistoryRetention(0, true, true); err == nil || !strings.Contains(err.Error(), "positive") {
		t.Fatalf("err=%v", err)
	}
	if err := ValidateHistoryRetention(0, false, false); err != nil {
		t.Fatalf("zero limit without prune is allowed: %v", err)
	}
}
