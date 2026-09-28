package issueopsbranch_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core "issueops/internal/adapter/issueops"
	authorizationoutbound "issueops/internal/adapter/outbound/issueopsauthorization"
	branchapp "issueops/internal/application/issueopsbranch"
	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
	issueopsdomain "issueops/internal/domain/issueops"
)

type linkStoreForTest struct {
	records map[string]model.IssueOpsRecord
	locked  bool
	writes  int
}

func (s *linkStoreForTest) WithinLock(_ context.Context, _ string, fn func() error) error {
	s.locked = true
	defer func() { s.locked = false }()
	return fn()
}
func (s *linkStoreForTest) Load(id string) (model.IssueOpsRecord, error) {
	if !s.locked {
		panic("read outside lock")
	}
	r, ok := s.records[id]
	if !ok {
		return r, os.ErrNotExist
	}
	return r, nil
}
func (s *linkStoreForTest) Save(r model.IssueOpsRecord) (model.IssueOpsRecord, error) {
	if !s.locked {
		panic("write outside lock")
	}
	s.records[r.ID] = r
	s.writes++
	return r, nil
}
func newLinkStoreForTest(records ...model.IssueOpsRecord) (*linkStoreForTest, branchapp.WorkspaceLinker) {
	s := &linkStoreForTest{records: map[string]model.IssueOpsRecord{}}
	for _, r := range records {
		if r.IssueURL == "" {
			r.IssueURL = "https://github.com/example/repo/issues/10"
		}
		r.BranchPrepare = &model.IssueOpsBranchPrepare{LinkVerified: true}
		r.DesignReview = &model.IssueOpsDesignReview{ProblemSummary: "problem", ProposedDesign: "design", Verification: []string{"design review checked alternatives and risks"}, Approved: true, RefactorPlan: "plan", Alternatives: []string{"alternative"}, Risks: []string{"risk"}}
		s.records[r.ID] = r
	}
	return s, branchapp.WorkspaceLinker{Records: s, Authority: cycleapp.NewMutationAuthority(authorizationoutbound.CanonicalPaths{}.Same), Files: core.LinkEnvironment{}, Now: func() time.Time { return time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC) }}
}
func LinkPlan(s branchapp.WorkspaceLinker, _ string, id, path string) (model.IssueOpsRecord, error) {
	return s.Plan(context.Background(), id, path, nil)
}
func LinkWorktree(s branchapp.WorkspaceLinker, _ string, id, path string) (model.IssueOpsRecord, error) {
	return s.Worktree(context.Background(), id, path, nil)
}
func ValidateIsolatedWorktreePath(record model.IssueOpsRecord, path string) error {
	return issueopsdomain.ValidateWorktreeLocation((core.LinkEnvironment{}).ObserveWorktree(record.Repo, path))
}
func ValidateWorktreeBranch(record model.IssueOpsRecord, path string) error {
	return issueopsdomain.ValidateLinkedWorktreeBranch(record.Branch, (core.LinkEnvironment{}).WorktreeBranch(path))
}
func planBodyForTest() []byte {
	return []byte("# plan\n" + strings.Join(issueopsdomain.RequiredPlanSections, "\nbody\n") + "\nbody\n")
}
func TestLinkPlanValidatesReadinessAndPersistsAbsolutePath(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/plan")
	planDir := filepath.Join(worktree, "docs")
	if err := os.MkdirAll(planDir, 0o700); err != nil {
		t.Fatal(err)
	}
	planPath := filepath.Join(planDir, "plan.md")
	if err := os.WriteFile(planPath, planBodyForTest(), 0o600); err != nil {
		t.Fatal(err)
	}
	record := model.IssueOpsRecord{
		ID:           "io-link-plan",
		Repo:         repo,
		Branch:       "feature/plan",
		Phase:        model.IssueOpsPhasePlan,
		IssueURL:     "https://github.com/example/repo/issues/10",
		WorktreePath: worktree,
	}
	_, store := newLinkStoreForTest(record)

	got, err := LinkPlan(store, t.TempDir(), record.ID, filepath.Join("docs", "plan.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got.PlanPath != planPath {
		t.Fatalf("PlanPath=%q, want %q", got.PlanPath, planPath)
	}
	if got.Phase != model.IssueOpsPhasePlan {
		t.Fatalf("Phase=%q, want %q", got.Phase, model.IssueOpsPhasePlan)
	}
}

func TestLinkPlanIsIdempotentButRejectsPathReplacement(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/plan-cas")
	planDir := filepath.Join(worktree, "docs")
	if err := os.MkdirAll(planDir, 0o700); err != nil {
		t.Fatal(err)
	}
	linkedPlan := filepath.Join(planDir, "linked.md")
	replacementPlan := filepath.Join(planDir, "replacement.md")
	for _, path := range []string{linkedPlan, replacementPlan} {
		if err := os.WriteFile(path, planBodyForTest(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	record := model.IssueOpsRecord{
		ID:           "io-link-plan-cas",
		Repo:         repo,
		Branch:       "feature/plan-cas",
		Phase:        model.IssueOpsPhasePlan,
		IssueURL:     "https://github.com/example/repo/issues/10",
		WorktreePath: worktree,
		PlanPath:     linkedPlan,
	}
	_, store := newLinkStoreForTest(record)

	got, err := LinkPlan(store, t.TempDir(), record.ID, filepath.Join("docs", "linked.md"))
	if err != nil {
		t.Fatalf("같은 plan 재연결은 멱등이어야 한다: %v", err)
	}
	if got.PlanPath != linkedPlan {
		t.Fatalf("멱등 재연결이 plan path를 바꿨다: %q", got.PlanPath)
	}
	if _, err := LinkPlan(store, t.TempDir(), record.ID, replacementPlan); err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("다른 plan path로 교체하면 fail-closed해야 한다: %v", err)
	}
}

func TestLinkPlanRejectsBoundaryViolations(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/plan-boundary")
	outsidePlanPath := filepath.Join(filepath.Dir(worktree), "outside-plan.md")
	if err := os.WriteFile(outsidePlanPath, planBodyForTest(), 0o600); err != nil {
		t.Fatal(err)
	}
	record := model.IssueOpsRecord{
		ID:           "io-link-plan-boundary",
		Repo:         repo,
		Branch:       "feature/plan-boundary",
		Phase:        model.IssueOpsPhasePlan,
		IssueURL:     "https://github.com/example/repo/issues/10",
		WorktreePath: worktree,
	}
	for _, tc := range []struct {
		name    string
		path    string
		mutate  func(*linkStoreForTest)
		wantErr string
	}{
		{name: "empty path", path: " ", wantErr: "plan_path is required"},
		{name: "path traversal", path: "../plan.md", wantErr: "path traversal"},
		{name: "missing branch evidence", path: "plan.md", mutate: func(s *linkStoreForTest) {
			r := s.records[record.ID]
			r.BranchPrepare = nil
			s.records[record.ID] = r
		}, wantErr: "before branch evidence"},
		{name: "missing design review", path: "plan.md", mutate: func(s *linkStoreForTest) {
			r := s.records[record.ID]
			r.DesignReview.Approved = false
			s.records[record.ID] = r
		}, wantErr: "design_approval"},
		{name: "missing file", path: "missing.md", wantErr: "plan_path does not exist"},
		{name: "absolute path outside worktree", path: outsidePlanPath, wantErr: "plan_path must be inside linked worktree"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			linkStore, store := newLinkStoreForTest(record)
			if tc.mutate != nil {
				tc.mutate(linkStore)
			}
			_, err := LinkPlan(store, t.TempDir(), record.ID, tc.path)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error=%v, want substring %q", err, tc.wantErr)
			}
		})
	}

	noWorktree := record
	noWorktree.ID = "io-no-worktree"
	noWorktree.WorktreePath = ""
	_, store := newLinkStoreForTest(noWorktree)
	if _, err := LinkPlan(store, t.TempDir(), noWorktree.ID, "plan.md"); err == nil || !strings.Contains(err.Error(), "before linked worktree") {
		t.Fatalf("expected linked worktree error, got %v", err)
	}
}

// 계획의 네 필수 절은 형식이 아니라 판단 기록이다. 절이 빠진 계획은 연결 자체를
// 거부해 "읽지 않은 것"과 "읽었는데 없는 것"을 구분하지 않은 계획이 구현에
// 들어가지 못하게 한다.
func TestLinkPlanRejectsPlanWithoutRequiredSections(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/plan-sections")
	planPath := filepath.Join(worktree, "plan.md")
	if err := os.WriteFile(planPath, []byte("# plan\n## 적용되는 결정과 주의사항\n- 없음\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	record := model.IssueOpsRecord{
		ID:           "io-link-plan-sections",
		Repo:         repo,
		Branch:       "feature/plan-sections",
		Phase:        model.IssueOpsPhasePlan,
		IssueURL:     "https://github.com/example/repo/issues/10",
		WorktreePath: worktree,
	}
	_, store := newLinkStoreForTest(record)
	_, err := LinkPlan(store, t.TempDir(), record.ID, "plan.md")
	if err == nil || !strings.Contains(err.Error(), "required sections") || !strings.Contains(err.Error(), "## 성능 영향") {
		t.Fatalf("plan without required sections must be rejected with the missing titles: %v", err)
	}
}

func TestLinkWorktreeValidatesIsolationBranchAndExistingPlan(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/worktree")
	planPath := filepath.Join(worktree, "plan.md")
	if err := os.WriteFile(planPath, planBodyForTest(), 0o600); err != nil {
		t.Fatal(err)
	}
	record := model.IssueOpsRecord{
		ID:       "io-link-worktree",
		Repo:     repo,
		Branch:   "feature/worktree",
		Phase:    model.IssueOpsPhasePlan,
		PlanPath: planPath,
	}
	_, store := newLinkStoreForTest(record)

	got, err := LinkWorktree(store, t.TempDir(), record.ID, worktree)
	if err != nil {
		t.Fatal(err)
	}
	if got.WorktreePath != worktree {
		t.Fatalf("WorktreePath=%q, want %q", got.WorktreePath, worktree)
	}

	otherRepo, otherWorktree := issueOpsRepoAndWorktreeFixture(t, "feature/other")
	otherPlan := filepath.Join(filepath.Dir(otherWorktree), "outside-plan.md")
	if err := os.WriteFile(otherPlan, planBodyForTest(), 0o600); err != nil {
		t.Fatal(err)
	}
	badPlan := record
	badPlan.ID = "io-bad-plan"
	badPlan.Repo = otherRepo
	badPlan.Branch = "feature/other"
	badPlan.PlanPath = otherPlan
	_, store = newLinkStoreForTest(badPlan)
	if _, err := LinkWorktree(store, t.TempDir(), badPlan.ID, otherWorktree); err == nil || !strings.Contains(err.Error(), "plan_path must be inside linked worktree") {
		t.Fatalf("expected plan/worktree boundary error, got %v", err)
	}
}

func TestLinkWorktreeRejectsBoundaryViolations(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/worktree-boundary")
	record := model.IssueOpsRecord{
		ID:     "io-link-worktree-boundary",
		Repo:   repo,
		Branch: "feature/worktree-boundary",
		Phase:  model.IssueOpsPhasePlan,
	}
	for _, tc := range []struct {
		name    string
		path    string
		mutate  func(*linkStoreForTest)
		wantErr string
	}{
		{name: "empty path", path: " ", wantErr: "worktree_path is required"},
		{name: "path traversal", path: "../worktree", wantErr: "path traversal"},
		{name: "missing branch evidence", path: worktree, mutate: func(s *linkStoreForTest) {
			r := s.records[record.ID]
			r.BranchPrepare = nil
			s.records[record.ID] = r
		}, wantErr: "before branch evidence"},
		{name: "missing directory", path: filepath.Join(filepath.Dir(worktree), "missing"), wantErr: "does not exist or is not a directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			linkStore, store := newLinkStoreForTest(record)
			if tc.mutate != nil {
				tc.mutate(linkStore)
			}
			_, err := LinkWorktree(store, t.TempDir(), record.ID, tc.path)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error=%v, want substring %q", err, tc.wantErr)
			}
		})
	}

	mismatchRepo, mismatchWorktree := issueOpsRepoAndWorktreeFixture(t, "feature/actual")
	mismatch := record
	mismatch.ID = "io-branch-mismatch"
	mismatch.Repo = mismatchRepo
	mismatch.Branch = "feature/expected"
	_, store := newLinkStoreForTest(mismatch)
	if _, err := LinkWorktree(store, t.TempDir(), mismatch.ID, mismatchWorktree); err == nil || !strings.Contains(err.Error(), "does not match IssueOps branch") {
		t.Fatalf("expected branch mismatch error, got %v", err)
	}
}

func TestValidateIssueURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid github https", "https://github.com/user/repo/issues/1", false},
		{"valid gitlab https", "https://gitlab.com/user/repo/-/issues/1", false},
		{"valid http", "http://example.com/issues/1", false},
		{"empty", "", true},
		{"not url", "not-a-url", true},
		{"no scheme", "github.com/user/repo/issues/1", true},
		{"ftp scheme", "ftp://example.com/issues/1", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := issueopsdomain.ValidateIssueURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIssueURL(%q) error = %v, wantErr = %v", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestValidateIsolatedWorktreePath(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/isolation")
	record := model.IssueOpsRecord{Repo: repo}
	if err := ValidateIsolatedWorktreePath(record, worktree); err != nil {
		t.Fatalf("valid worktree rejected: %v", err)
	}
	if err := ValidateIsolatedWorktreePath(record, repo); err == nil || !strings.Contains(err.Error(), "isolated") {
		t.Fatalf("expected source checkout isolation error, got %v", err)
	}
	outside := filepath.Join(filepath.Dir(repo), "outside")
	if err := os.MkdirAll(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := ValidateIsolatedWorktreePath(record, outside); err == nil || !strings.Contains(err.Error(), "sibling worktree directory") {
		t.Fatalf("expected sibling worktree directory error, got %v", err)
	}
	symlink := filepath.Join(filepath.Dir(worktree), "linked")
	if err := os.Symlink(worktree, symlink); err != nil {
		t.Skipf("symlink fixture unavailable: %v", err)
	}
	if err := ValidateIsolatedWorktreePath(record, symlink); err == nil || !strings.Contains(err.Error(), "must not be a symlink") {
		t.Fatalf("expected symlink rejection, got %v", err)
	}
}

func TestValidateWorktreeBranch(t *testing.T) {
	_, worktree := issueOpsRepoAndWorktreeFixture(t, "feature/branch")
	if err := ValidateWorktreeBranch(model.IssueOpsRecord{Branch: "feature/branch"}, worktree); err != nil {
		t.Fatalf("matching branch rejected: %v", err)
	}
	if err := ValidateWorktreeBranch(model.IssueOpsRecord{}, worktree); err != nil {
		t.Fatalf("empty expected branch should be allowed: %v", err)
	}
	if err := ValidateWorktreeBranch(model.IssueOpsRecord{Branch: "feature/other"}, worktree); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected branch mismatch, got %v", err)
	}
	noGit := t.TempDir()
	if err := ValidateWorktreeBranch(model.IssueOpsRecord{Branch: "feature/branch"}, noGit); err == nil || !strings.Contains(err.Error(), "must be a git worktree") {
		t.Fatalf("expected git worktree error, got %v", err)
	}
}

func issueOpsRepoAndWorktreeFixture(t *testing.T, branch string) (string, string) {
	t.Helper()
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	worktree := filepath.Join(root, "repo.worktrees", strings.ReplaceAll(branch, "/", "-"))
	writeGitHeadForTest(t, repo, branch)
	writeGitHeadForTest(t, worktree, branch)
	return repo, worktree
}

func writeGitHeadForTest(t *testing.T, path, branch string) {
	t.Helper()
	gitDir := filepath.Join(path, ".git")
	if err := os.MkdirAll(gitDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/"+branch+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestPlanRelinkKeepsIdentityBeforeReadingSectionsAndDoesNotWrite(t *testing.T) {
	repo, worktree := issueOpsRepoAndWorktreeFixture(t, "63-plan")
	plan := filepath.Join(worktree, "plan.md")
	if err := os.WriteFile(plan, []byte("in-progress edit without sections"), 0600); err != nil {
		t.Fatal(err)
	}
	before := model.IssueOpsRecord{ID: "id", Repo: repo, Branch: "63-plan", WorktreePath: worktree, PlanPath: "plan.md", UpdatedAt: "before"}
	store, s := newLinkStoreForTest(before)
	got, err := s.Plan(context.Background(), "id", plan, nil)
	if err != nil || got.PlanPath != "plan.md" || got.UpdatedAt != "before" || store.writes != 0 {
		t.Fatalf("relink changed identity or required sections: %+v, %v", got, err)
	}
	replacement := filepath.Join(worktree, "replacement.md")
	if err := os.WriteFile(replacement, []byte("no sections"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Plan(context.Background(), "id", replacement, nil); err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("identity must be checked before sections: %v", err)
	}
	if store.writes != 0 {
		t.Fatal("replacement wrote record")
	}
}

func TestWorktreeLinkRejectsResolvedSiblingEscape(t *testing.T) {
	root := t.TempDir()
	repo := filepath.Join(root, "repo")
	outside := filepath.Join(root, "outside")
	worktree := filepath.Join(root, "repo.worktrees", "63-plan")
	writeGitHeadForTest(t, repo, "main")
	writeGitHeadForTest(t, filepath.Join(outside, "63-plan"), "63-plan")
	if err := os.Symlink(outside, filepath.Dir(worktree)); err != nil {
		t.Fatal(err)
	}
	store, s := newLinkStoreForTest(model.IssueOpsRecord{ID: "id", Repo: repo, Branch: "63-plan"})
	if _, err := s.Worktree(context.Background(), "id", worktree, nil); err == nil || !strings.Contains(err.Error(), "resolve under sibling") {
		t.Fatalf("resolved escape accepted: %v", err)
	}
	if store.writes != 0 {
		t.Fatal("escape wrote record")
	}
}
