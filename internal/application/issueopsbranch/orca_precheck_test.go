package issueopsbranch

import (
	"errors"
	model "issueops/internal/contract/issueops"
	port "issueops/internal/port/issueopsbranch"
	"reflect"
	"testing"
)

func TestOrcaBranchPrecheckReadsThenChecksLocalBeforeRemote(t *testing.T) {
	for _, tt := range []struct {
		name, branch, readError, local, remote, wantCode string
		prepared                                         bool
		want                                             []string
	}{
		{name: "read error before empty branch", readError: "missing", wantCode: "orca_branch_precheck_failed", want: []string{"read"}},
		{name: "empty branch", wantCode: "orca_branch_name_taken", want: []string{"read"}},
		{name: "local stops remote", branch: "feature", local: "abc", remote: "abc", prepared: true, wantCode: "orca_branch_name_taken", want: []string{"read", "refs/heads/feature"}},
		{name: "remote collision", branch: "feature", remote: "abc", wantCode: "orca_branch_name_taken", want: []string{"read", "refs/heads/feature", "refs/remotes/origin/feature"}},
		{name: "verified GitLab remote", branch: "feature", remote: "abc", prepared: true, want: []string{"read", "refs/heads/feature", "refs/remotes/origin/feature"}},
		{name: "no refs", branch: " feature ", want: []string{"read", "refs/heads/feature", "refs/remotes/origin/feature"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var calls []string
			check := OrcaBranchPrecheck{Observations: port.OrcaBranchObservations{
				ReadRecord: func(id string) (model.IssueOpsRecord, error) {
					calls = append(calls, "read")
					if id != "cycle" {
						t.Fatal(id)
					}
					if tt.readError != "" {
						return model.IssueOpsRecord{}, errors.New(tt.readError)
					}
					record := model.IssueOpsRecord{Repo: "repo"}
					if tt.prepared {
						record.BranchPrepare = &model.IssueOpsBranchPrepare{Provider: "gitlab", Branch: "feature", BaseSHA: "abc", LinkVerified: true}
					}
					return record, nil
				},
				RefOID: func(repo, ref string) (string, bool) {
					if repo != "repo" {
						t.Fatal(repo)
					}
					calls = append(calls, ref)
					value := tt.local
					if ref == "refs/remotes/origin/feature" {
						value = tt.remote
					}
					return value, value != ""
				},
			}}
			code, err := check.Check("cycle", tt.branch)
			if code != tt.wantCode || (err != nil) != (tt.wantCode != "") || !reflect.DeepEqual(calls, tt.want) {
				t.Fatalf("code=%q err=%v calls=%v want=%v", code, err, calls, tt.want)
			}
		})
	}
}
