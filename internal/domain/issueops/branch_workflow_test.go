package issueops

import (
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestValidatePreparationNamesChecksBranchBeforeBase(t *testing.T) {
	cases := []struct {
		branch, base, want string
	}{
		{"", "main", "branch is required"},
		{"12-fix-x", "", "base_branch is required"},
		{"", "", "branch is required"},
	}
	for _, tc := range cases {
		err := ValidatePreparationNames(tc.branch, tc.base)
		if err == nil || !strings.HasPrefix(err.Error(), tc.want) {
			t.Errorf("(%q,%q) = %v, want %s", tc.branch, tc.base, err, tc.want)
		}
	}
	if err := ValidatePreparationNames("12-fix-x", "main"); err != nil {
		t.Fatalf("valid names rejected: %v", err)
	}
	// 브랜치 이름 검증 오류는 base 누락보다 먼저 보고된다.
	want := "issueops branch contains invalid characters"
	if err := ValidatePreparationNames("bad branch", ""); err == nil || !strings.HasPrefix(err.Error(), want) {
		t.Fatalf("got %v, want error starting %q", err, want)
	}
	for _, base := range []string{"ma in", "main~1", "-main", "main.lock"} {
		if err := ValidatePreparationNames("12-fix-x", base); err == nil || !strings.HasPrefix(err.Error(), "base_branch contains invalid characters") {
			t.Errorf("base %q = %v, want base_branch refusal", base, err)
		}
	}
	if err := ValidatePreparationNames("12-fix-x", "release/1.2"); err != nil {
		t.Fatalf("slash base rejected: %v", err)
	}
}

func TestValidateBranchRefusesNamesGitRefuses(t *testing.T) {
	// 각 사례는 git check-ref-format --branch 결과와 같다.
	refused := []string{"7-fix~1", "7-fix^", "7-fix:x", "7-fix?", "7-fix*", "7-fix[x", "7-fix\\x",
		"7-fix..x", "7-fix@{u}", "7-fix/", "7-fix//x", "7-fix.", "7-fix/.hidden", "7-fix.lock", "7-fix.lock/x", "7-fix\x01"}
	for _, branch := range refused {
		if err := ValidateBranch(branch); err == nil || !strings.Contains(err.Error(), "git branch names cannot") {
			t.Errorf("ValidateBranch(%q) = %v, want git ref-name refusal", branch, err)
		}
	}
	for _, branch := range []string{"7-로그인-수정", "7-fix/sub", "7-fix.v2", "7-fix@x", "7-fix-lock"} {
		if err := ValidateBranch(branch); err != nil {
			t.Errorf("ValidateBranch(%q) = %v, want accepted", branch, err)
		}
	}
}

func TestValidatePreparationParentPathRequiresAbsolute(t *testing.T) {
	if err := ValidatePreparationParentPath(false); err == nil || err.Error() != "parent_worktree must be an absolute path" {
		t.Fatalf("relative path error = %v", err)
	}
	if err := ValidatePreparationParentPath(true); err != nil {
		t.Fatal(err)
	}
}

func TestAdoptPreparedBranchBindsOnlyBranchlessLinkedCycle(t *testing.T) {
	linked := model.IssueOpsRecord{IssueURL: "https://example.com/o/r/issues/1"}

	adopted, err := AdoptPreparedBranch(linked, linked.IssueURL, "feature/x")
	if err != nil || adopted.Branch != "feature/x" {
		t.Fatalf("branchless cycle: %+v %v", adopted, err)
	}
	if linked.Branch != "" {
		t.Fatal("input record mutated")
	}
	same := linked
	same.Branch = "feature/x"
	if _, err := AdoptPreparedBranch(same, linked.IssueURL, "feature/x"); err != nil {
		t.Fatalf("same branch rejected: %v", err)
	}

	cases := []struct {
		name   string
		record model.IssueOpsRecord
		url    string
		branch string
		want   string
	}{
		{"unlinked", model.IssueOpsRecord{IssueURL: "  "}, "u", "b", "issue must be linked before branch prepare"},
		{"issue mismatch", linked, "https://other", "b", "issue_url does not match linked IssueOps issue"},
		{"branch mismatch", same, linked.IssueURL, "feature/y", "branch does not match IssueOps record branch"},
	}
	for _, tc := range cases {
		got, err := AdoptPreparedBranch(tc.record, tc.url, tc.branch)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%s: err = %v, want %s", tc.name, err, tc.want)
		}
		if got.ID != "" || got.Branch != "" || got.IssueURL != "" {
			t.Errorf("%s: failure must return zero record, got %+v", tc.name, got)
		}
	}
}

func TestValidatePreparationUmbrellaAllowsMatchingOrUnbranchedUmbrella(t *testing.T) {
	if err := ValidatePreparationUmbrella(model.IssueOpsRecord{}, "child", "main"); err != nil {
		t.Fatalf("branchless umbrella: %v", err)
	}
	umbrella := model.IssueOpsRecord{ID: "u-1", Branch: " feature/umbrella "}
	if err := ValidatePreparationUmbrella(umbrella, "child", "feature/umbrella"); err != nil {
		t.Fatalf("matching base: %v", err)
	}
	err := ValidatePreparationUmbrella(umbrella, " child ", " main ")
	if err == nil {
		t.Fatal("base off the umbrella branch accepted")
	}
	for _, part := range []string{"child", "u-1", "feature/umbrella", `"main"`} {
		if !strings.Contains(err.Error(), part) {
			t.Errorf("error %q missing %q", err, part)
		}
	}
}

func TestApplyBranchPreparationRecordsRequestWithoutMutatingInput(t *testing.T) {
	input := model.IssueOpsRecord{ID: "r", UpdatedAt: "old"}
	steps := []model.IssueOpsBranchPrepareStep{{}}
	req := model.IssueOpsBranchPrepareRequest{
		Provider: "github", IssueURL: "u", Branch: "b", BaseBranch: "main", BaseSHA: "sha",
		ParentWorktree: "/p", RemoteBranchURL: "rb", LinkVerified: true,
	}
	got := ApplyBranchPreparation(input, req, "proj", steps, "t1", "t2")
	prep := got.BranchPrepare
	if prep == nil || prep.Provider != "github" || prep.Branch != "b" || prep.BaseBranch != "main" || prep.BaseSHA != "sha" ||
		prep.ParentWorktree != "/p" || prep.RemoteBranchURL != "rb" || prep.CodeProjectKey != "proj" ||
		!prep.LinkVerified || prep.CreatedAt != "t1" || len(prep.Steps) != 1 {
		t.Fatalf("unexpected preparation: %+v", prep)
	}
	if got.UpdatedAt != "t2" || input.BranchPrepare != nil || input.UpdatedAt != "old" {
		t.Fatalf("updatedAt/input handling wrong: %+v / %+v", got, input)
	}
}

func retargetableRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		BranchPrepare:  &model.IssueOpsBranchPrepare{BaseBranch: " main ", Retargets: []model.IssueOpsBranchRetarget{{ToBase: "main"}}},
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{URL: " https://example.com/pr/1 ", TargetBranch: "main"},
	}
}

func TestRetargetRequiresHolderWithoutExecution(t *testing.T) {
	required, err := RetargetRequiresHolder(model.IssueOpsRecord{})
	if err != nil || required {
		t.Fatalf("no execution = %v, %v", required, err)
	}
	_, err = RetargetRequiresHolder(model.IssueOpsRecord{Execution: &model.Execution{}})
	if err == nil || !strings.HasPrefix(err.Error(), "invalid IssueOps execution v1 record: ") {
		t.Fatalf("invalid execution error = %v", err)
	}
}

func TestValidateRetargetRequest(t *testing.T) {
	for _, tc := range []struct{ base, reason, want string }{
		{"", "r", "base_branch is required"},
		{"", "", "base_branch is required"},
		{"b", "", "reason is required"},
	} {
		if err := ValidateRetargetRequest(tc.base, tc.reason); err == nil || !strings.HasPrefix(err.Error(), tc.want) {
			t.Errorf("(%q,%q) = %v", tc.base, tc.reason, err)
		}
	}
	if err := ValidateRetargetRequest("b", "r"); err != nil {
		t.Fatal(err)
	}
}

func TestValidateRetargetRecordRequiresPreparedBranchArtifactAndNewBase(t *testing.T) {
	if err := ValidateRetargetRecord(retargetableRecord(), "develop"); err != nil {
		t.Fatalf("valid retarget rejected: %v", err)
	}
	noPrepare := retargetableRecord()
	noPrepare.BranchPrepare = nil
	noArtifact := retargetableRecord()
	noArtifact.RemoteArtifact = nil
	blankURL := retargetableRecord()
	blankURL.RemoteArtifact.URL = "  "
	for name, tc := range map[string]struct {
		record model.IssueOpsRecord
		base   string
		want   string
	}{
		"unprepared":  {noPrepare, "develop", "branch must be prepared before retarget"},
		"no artifact": {noArtifact, "develop", "a verified remote artifact is required before retarget"},
		"blank url":   {blankURL, "develop", "a verified remote artifact is required before retarget"},
		"same base":   {retargetableRecord(), "main", `base_branch "main" is already the prepared base`},
	} {
		if err := ValidateRetargetRecord(tc.record, tc.base); err == nil || err.Error() != tc.want {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestValidateRetargetArtifactDetectsConcurrentChange(t *testing.T) {
	current := model.IssueOpsRemoteArtifactVerification{URL: "u", Labels: []string{"a"}}
	same := model.IssueOpsRemoteArtifactVerification{URL: "u", Labels: []string{"a"}}
	if err := ValidateRetargetArtifact(&same, current); err != nil {
		t.Fatalf("equal artifact: %v", err)
	}
	changed := model.IssueOpsRemoteArtifactVerification{URL: "u", Labels: []string{"b"}}
	for name, observed := range map[string]*model.IssueOpsRemoteArtifactVerification{"nil": nil, "changed": &changed} {
		if err := ValidateRetargetArtifact(observed, current); !errors.Is(err, ErrRetargetObservationStale) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestValidateRetargetTarget(t *testing.T) {
	if err := ValidateRetargetTarget("develop", " develop "); err != nil {
		t.Fatalf("trimmed match rejected: %v", err)
	}
	err := ValidateRetargetTarget("develop", "main")
	if err == nil || err.Error() != `remote artifact targets "main", not "develop"` {
		t.Fatalf("mismatch error = %v", err)
	}
}

func TestValidateRetargetOriginPrecedence(t *testing.T) {
	obsErr := errors.New("boom")
	cases := []struct {
		name                       string
		repo, base, obsRepo, obsBs string
		stated, present            bool
		obsErr                     error
		stale                      bool
		cause                      error
		want                       string
	}{
		{name: "unstated", repo: "r", base: "b", obsRepo: "r", obsBs: "b", stale: true, present: true},
		{name: "repo drift", stated: true, repo: "r", base: "b", obsRepo: "x", obsBs: "b", stale: true},
		{name: "base drift beats observation error", stated: true, repo: "r", base: "b", obsRepo: "r", obsBs: "x", obsErr: obsErr, stale: true},
		{name: "observation error", stated: true, repo: "r", base: "b", obsRepo: "r", obsBs: "b", obsErr: obsErr, cause: obsErr},
		{name: "absent", stated: true, repo: "r", base: "b", obsRepo: "r", obsBs: "b", want: `base_branch "b" is absent from origin`},
	}
	for _, tc := range cases {
		err := ValidateRetargetOrigin(tc.repo, tc.base, tc.obsRepo, tc.obsBs, tc.stated, tc.present, tc.obsErr)
		switch {
		case err == nil:
			t.Errorf("%s: nil error", tc.name)
		case tc.stale && !errors.Is(err, ErrRetargetObservationStale):
			t.Errorf("%s: not stale: %v", tc.name, err)
		case tc.cause != nil && !errors.Is(err, tc.cause):
			t.Errorf("%s: cause lost: %v", tc.name, err)
		case tc.want != "" && err.Error() != tc.want:
			t.Errorf("%s: %v", tc.name, err)
		}
	}
	if err := ValidateRetargetOrigin("r", "b", "r", "b", true, true, nil); err != nil {
		t.Fatalf("healthy origin: %v", err)
	}
}

func TestApplyBranchRetargetAppendsHistoryAndLeavesInputUntouched(t *testing.T) {
	input := retargetableRecord()
	got := ApplyBranchRetarget(input, "develop", "why", "obs", "upd")
	if got.BranchPrepare.BaseBranch != "develop" || got.RemoteArtifact.TargetBranch != "develop" || got.UpdatedAt != "upd" {
		t.Fatalf("retarget not applied: %+v %+v", got.BranchPrepare, got.RemoteArtifact)
	}
	want := model.IssueOpsBranchRetarget{FromBase: "main", ToBase: "develop", Reason: "why", ArtifactURL: "https://example.com/pr/1", ObservedAt: "obs"}
	history := got.BranchPrepare.Retargets
	if len(history) != 2 || history[0].ToBase != "main" || history[1] != want {
		t.Fatalf("history = %+v", history)
	}
	if input.BranchPrepare.BaseBranch != " main " || len(input.BranchPrepare.Retargets) != 1 ||
		input.RemoteArtifact.TargetBranch != "main" || input.UpdatedAt != "" {
		t.Fatalf("input mutated: %+v %+v", input.BranchPrepare, input.RemoteArtifact)
	}
}
