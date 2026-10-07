package issueopsapp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"issueops/internal/adapter/outbound/issueopsrecord"
	"issueops/internal/adapter/outbound/sqlstore"
	issueopscontract "issueops/internal/contract/issueops"
	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

func TestNextPersistedCLISelectionAndRootConflict(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	t.Setenv("ISSUEOPS_ID", "")
	repo := makeGitRepoForContract(t)
	foreign := makeGitRepoForContract(t)
	alias := filepath.Join(repo, "nested")
	if err := os.Mkdir(alias, 0o700); err != nil {
		t.Fatal(err)
	}
	root := issueOpsStateRoot()
	db, err := sqlstore.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	selected := issueopscontract.IssueOpsRecord{
		SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
		ID:            "io-selected", Repo: repo, Branch: "topic/one", Phase: issueopscontract.IssueOpsPhasePlan,
	}
	claim := func(id, source string) issueopscontract.IssueOpsRecord {
		return issueopscontract.IssueOpsRecord{
			SchemaVersion: issueopscontract.IssueOpsSchemaVersion,
			ID:            id, Repo: source, Branch: "topic/one", Phase: issueopscontract.IssueOpsPhasePlan,
			Execution: &issueopscontract.Execution{
				Mode: issueopscontract.ExecutionModeDirect,
				Workspace: issueopscontract.Workspace{
					SourceRoot: source, Root: repo + ".worktrees/extra/../topic-one",
					Branch: "topic/one", BaseHead: strings.Repeat("a", 40), Driver: "git", LinkedAt: "then",
				},
				Lease:     issueopscontract.WriteLease{Generation: 1, Status: issueopscontract.LeaseStatusReleased},
				Selection: selectionFixture(issueopscontract.ExecutionModeDirect),
			},
		}
	}
	for _, record := range []issueopscontract.IssueOpsRecord{
		selected, claim("io-foreign", foreign), claim("io-first", alias), claim("io-last", repo),
		{SchemaVersion: issueopscontract.IssueOpsSchemaVersion, ID: "io-alias", Repo: alias, Phase: issueopscontract.IssueOpsPhaseGrill},
	} {
		data, err := issueopsrecord.Encode(record)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Put("issueops_v1", record.ID, data); err != nil {
			t.Fatal(err)
		}
	}
	// A would-be first claimant must not count before strict validation.
	if err := db.Put("issueops_v1", "io-aaa-corrupt", []byte(`{"schema_version":0,"id":"io-aaa-corrupt","repo":"`+repo+`","execution":{"workspace":{"root":"`+repo+`.worktrees/topic-one"}}}`)); err != nil {
		t.Fatal(err)
	}
	duplicate, err := issueopsrecord.Encode(claim("io-aab-duplicate", repo))
	if err != nil {
		t.Fatal(err)
	}
	// Decode uses the last duplicate key; a raw root prefilter must not grant a claim.
	rootField, err := json.Marshal(repo + ".worktrees/extra/../topic-one")
	if err != nil {
		t.Fatal(err)
	}
	duplicate = []byte(strings.Replace(string(duplicate), `"root": `+string(rootField),
		`"root": `+string(rootField)+`, "root": "/elsewhere"`, 1))
	if err := db.Put("issueops_v1", "io-aab-duplicate", duplicate); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name, id, env, stage, conflict string
	}{
		{"root conflict", " io-selected ", "io-foreign", issueopsnextcontract.StageBlockedRoot, "io-first"},
		{"foreign", "io-foreign", "", "invalid", ""},
		{"alias", "io-alias", "", "issue", ""},
		{"environment", "", " io-alias ", "issue", ""},
		{"missing", "io-missing", "", "invalid", ""},
		{"corrupt", "io-aaa-corrupt", "", "invalid", ""},
		{"auto", "", "", "ambiguous", ""},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Setenv("ISSUEOPS_ID", scenario.env)
			output := captureStdoutForContract(t, func() error {
				return runIssueOps([]string{"next", "--cwd", repo, "--id", scenario.id, "--json"})
			})
			var result issueopsnextcontract.Result
			if err := json.Unmarshal([]byte(output), &result); err != nil {
				t.Fatal(err)
			}
			if !result.OK || result.Stage.Key != scenario.stage {
				t.Fatalf("next CLI = %+v, want stage %s", result, scenario.stage)
			}
			if scenario.conflict != "" && !strings.Contains(strings.Join(result.Warnings, "\n"), "cycle "+scenario.conflict) {
				t.Fatalf("root conflict lost strict first-ID/repo/clean matching: %+v", result)
			}
		})
	}
}
