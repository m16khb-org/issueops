package issueops

import (
	model "issueops/internal/contract/issueops"
	"reflect"
	"strings"
	"testing"
)

func TestOrcaBranchScopesPreserveLocalFirstAndRejectEmptyName(t *testing.T) {
	scopes, err := OrcaBranchScopes(" feature ")
	if err != nil || len(scopes) != 2 || scopes[0].Ref != "refs/heads/feature" || scopes[0].Remote || scopes[1].Ref != "refs/remotes/origin/feature" || !scopes[1].Remote {
		t.Fatalf("scopes=%+v err=%v", scopes, err)
	}
	if scopes, err := OrcaBranchScopes(" \t"); err == nil || scopes != nil {
		t.Fatalf("empty scopes=%+v err=%v", scopes, err)
	}
}
func TestOrcaBranchRefOnlyAllowsExactVerifiedGitLabRemote(t *testing.T) {
	for _, tt := range []struct {
		name            string
		edit            func(*model.IssueOpsBranchPrepare)
		remote          bool
		oid             string
		absent, allowed bool
	}{
		{name: "exact remote", remote: true, oid: " ABC ", allowed: true},
		{name: "local remains occupied", oid: "abc"},
		{name: "no preparation", remote: true, oid: "abc", absent: true},
		{name: "GitHub", remote: true, oid: "abc", edit: func(p *model.IssueOpsBranchPrepare) { p.Provider = "github" }},
		{name: "unverified", remote: true, oid: "abc", edit: func(p *model.IssueOpsBranchPrepare) { p.LinkVerified = false }},
		{name: "another branch", remote: true, oid: "abc", edit: func(p *model.IssueOpsBranchPrepare) { p.Branch = "different" }},
		{name: "empty base", remote: true, oid: "", edit: func(p *model.IssueOpsBranchPrepare) { p.BaseSHA = " " }},
		{name: "different commit", remote: true, oid: "def"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			prepared := &model.IssueOpsBranchPrepare{Provider: " GITLAB ", LinkVerified: true, Branch: " feature ", BaseSHA: " abc "}
			if tt.edit != nil {
				tt.edit(prepared)
			}
			before := *prepared
			if tt.absent {
				prepared = nil
			}
			scopes, _ := OrcaBranchScopes("feature")
			scope := scopes[0]
			if tt.remote {
				scope = scopes[1]
			}
			err := ValidateOrcaBranchRef(prepared, scope, tt.oid)
			if (err == nil) != tt.allowed {
				t.Fatalf("err=%v allowed=%t", err, tt.allowed)
			}
			if err != nil && (!strings.Contains(err.Error(), scope.Where) || !strings.Contains(err.Error(), scope.Remedy)) {
				t.Fatalf("lost actionable refusal: %v", err)
			}
			if prepared != nil && !reflect.DeepEqual(*prepared, before) {
				t.Fatal("validation mutated preparation")
			}
		})
	}
}
