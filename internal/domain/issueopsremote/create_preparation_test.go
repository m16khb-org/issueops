package remote

import (
	"strings"
	"testing"
)

func TestCreateAuthorityPreservesGateOrder(t *testing.T) {
	for _, tc := range []struct {
		name  string
		facts CreateAuthority
		want  string
	}{
		{"provider", CreateAuthority{Provider: "unknown"}, "remote provider must be github or gitlab"},
		{"phase", CreateAuthority{Provider: "github"}, "phase pr and no existing remote artifact"},
		{"existing artifact", CreateAuthority{Provider: "github", PhasePR: true, Artifact: true}, "phase pr and no existing remote artifact"},
		{"execution", CreateAuthority{Provider: "github", PhasePR: true, Confirm: true}, "requires IssueOps execution v1"},
		{"generation", CreateAuthority{Provider: "github", PhasePR: true, Confirm: true, Execution: true, Generation: 2, ExpectedGeneration: 1}, "current=2 expected=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := ValidateCreateAuthority(tc.facts)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want=%s", err, tc.want)
			}
		})
	}
	provider, kind, err := ValidateCreateAuthority(CreateAuthority{Provider: " GITLAB ", PhasePR: true})
	if err != nil || provider != "gitlab" || kind != "mr" {
		t.Fatalf("preview: %s/%s %v", provider, kind, err)
	}
}

func TestCreateReviewRejectsMissingAndUnobservableEvidence(t *testing.T) {
	if err := ValidateCreatePending(true); err == nil {
		t.Fatal("pending intent accepted")
	}
	if err := ValidateCreatePending(false); err != nil {
		t.Fatal(err)
	}
	if err := ValidateCreateReview("cycle", "missing", "", true); err == nil || !strings.Contains(err.Error(), "pass implementation review (missing)") {
		t.Fatalf("err=%v", err)
	}
	if err := ValidateCreateReview("cycle", "", "", true); err == nil || !strings.Contains(err.Error(), "freshness") {
		t.Fatalf("err=%v", err)
	}
	if err := ValidateCreateReview("cycle", "", "observed", true); err != nil {
		t.Fatal(err)
	}
}

func createRequestFixture() (CreateBranchAuthority, CreateRequest) {
	return CreateBranchAuthority{Prepared: true, Provider: "github", WorkspaceBranch: "42-work", BaseBranch: "main", IssueURL: "https://github.com/team/repo/issues/42"}, CreateRequest{
		Provider: "github", Head: "42-work", Base: "main", Title: " Title ", Body: " Body ", Labels: []string{" bug ", "bug"}, Assignees: []string{" maintainer "},
	}
}

func TestPrepareCreateRequestBindsBranchAndCanonicalMetadata(t *testing.T) {
	authority, candidate := createRequestFixture()
	got, err := PrepareCreateRequest(authority, candidate, false)
	if err != nil || got.ProjectKey != "github.com/team/repo" || got.Title != "Title" || got.Body != "Body" || len(got.Labels) != 1 || got.Assignees[0] != "maintainer" {
		t.Fatalf("request=%+v err=%v", got, err)
	}
	for _, tc := range []struct {
		name   string
		change func(*CreateBranchAuthority, *CreateRequest)
		secret bool
		want   string
	}{
		{"unprepared", func(a *CreateBranchAuthority, _ *CreateRequest) { a.Prepared = false }, false, "branch preparation"},
		{"head", func(_ *CreateBranchAuthority, c *CreateRequest) { c.Head = "other" }, false, "linked issue authority"},
		{"base", func(_ *CreateBranchAuthority, c *CreateRequest) { c.Base = "other" }, false, "linked issue authority"},
		{"title", func(_ *CreateBranchAuthority, c *CreateRequest) { c.Title = " " }, false, "title is required"},
		{"body limit", func(_ *CreateBranchAuthority, c *CreateRequest) { c.Body = strings.Repeat("x", 1<<20+1) }, true, "body must not exceed"},
		{"secret", func(_ *CreateBranchAuthority, _ *CreateRequest) {}, true, "secret-like content"},
		{"labels", func(_ *CreateBranchAuthority, c *CreateRequest) { c.Labels = nil }, false, "canonical labels and assignees"},
		{"assignee", func(_ *CreateBranchAuthority, c *CreateRequest) { c.Assignees = []string{"@me"} }, false, "placeholder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, c := createRequestFixture()
			tc.change(&a, &c)
			_, err := PrepareCreateRequest(a, c, tc.secret)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err=%v want=%s", err, tc.want)
			}
		})
	}
}

func TestCreateHeadRequiresResolvableSHA(t *testing.T) {
	for _, value := range []string{"", "not-a-head", strings.Repeat("g", 40), strings.Repeat("a", 39)} {
		if err := ValidateCreateHead(value); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	for _, value := range []string{strings.Repeat("a", 40), strings.Repeat("B", 64)} {
		if err := ValidateCreateHead(value); err != nil {
			t.Fatal(err)
		}
	}
}
