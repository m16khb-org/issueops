package issueopscompletion

import (
	"errors"
	"strings"
	"testing"

	completioncontract "issueops/internal/contract/issueopscompletion"
)

func TestDenialExposesCodeAndCause(t *testing.T) {
	err := ValidateActive(Snapshot{Phase: "plan"}, completioncontract.Command{}, true)
	if CodeOf(err) != DenyPhase {
		t.Fatalf("code = %q", CodeOf(err))
	}
	if err.Error() != "execution completion requires pr phase" {
		t.Fatalf("Error() = %q", err.Error())
	}
	denial, ok := errors.AsType[*Denial](err)
	if !ok || denial.Unwrap() == nil || denial.Unwrap().Error() != err.Error() {
		t.Fatalf("Unwrap must expose the cause: %v", denial)
	}
	if CodeOf(errors.New("plain")) != "" || CodeOf(nil) != "" {
		t.Fatal("non-denial errors have no code")
	}
}

func TestValidateActiveDeniesEachBrokenPrecondition(t *testing.T) {
	process := &completioncontract.ProcessReceipt{PID: 7, StartedAt: "s", Executable: "/bin/codex"}
	holder := completioncontract.Actor{Host: "codex", SessionID: "s1", AgentID: "a1", Process: process}
	active := Snapshot{Phase: "pr", Lease: completioncontract.Lease{Generation: 3, Status: "active", Holder: &holder}}
	command := completioncontract.Command{Generation: 3, Actor: holder}

	if err := ValidateActive(active, command, true); err != nil {
		t.Fatalf("baseline must pass: %v", err)
	}
	sameWithPaddingAndCase := command
	sameWithPaddingAndCase.Actor = completioncontract.Actor{Host: " CODEX ", SessionID: " s1 ", AgentID: " a1 ", Process: &completioncontract.ProcessReceipt{PID: 7, StartedAt: "s", Executable: "/bin/codex"}}
	if err := ValidateActive(active, sameWithPaddingAndCase, true); err != nil {
		t.Fatalf("host case and padding must not matter: %v", err)
	}

	for _, test := range []struct {
		name   string
		mutate func(*Snapshot, *completioncontract.Command)
		cwd    bool
		code   DenyCode
	}{
		{"released lease", func(s *Snapshot, _ *completioncontract.Command) { s.Lease.Status = "released" }, true, DenyAuthority},
		{"other generation", func(_ *Snapshot, c *completioncontract.Command) { c.Generation = 2 }, true, DenyAuthority},
		{"different agent", func(_ *Snapshot, c *completioncontract.Command) { c.Actor.AgentID = "a2" }, true, DenyAuthority},
		{"different pid", func(_ *Snapshot, c *completioncontract.Command) {
			c.Actor.Process = &completioncontract.ProcessReceipt{PID: 8, StartedAt: "s", Executable: "/bin/codex"}
		}, true, DenyAuthority},
		{"command lacks process", func(_ *Snapshot, c *completioncontract.Command) { c.Actor.Process = nil }, true, DenyAuthority},
		{"lease lacks holder", func(s *Snapshot, _ *completioncontract.Command) { s.Lease.Holder = nil }, true, DenyAuthority},
		{"non-canonical cwd", func(*Snapshot, *completioncontract.Command) {}, false, DenyCWD},
	} {
		t.Run(test.name, func(t *testing.T) {
			snapshot, cmd := active, command
			test.mutate(&snapshot, &cmd)
			if got := CodeOf(ValidateActive(snapshot, cmd, test.cwd)); got != test.code {
				t.Fatalf("code = %q, want %q", got, test.code)
			}
		})
	}
}

func TestValidateArtifactRejectsMalformedOrUnsupportedURLs(t *testing.T) {
	gitlab := func(r *completioncontract.RecordSnapshot, issue, artifact string) {
		r.IssueURL = issue
		r.Artifact.Provider, r.Artifact.Kind, r.Artifact.URL = "gitlab", "mr", artifact
	}
	for _, test := range []struct {
		name   string
		mutate func(*completioncontract.RecordSnapshot)
		want   string
	}{
		{"unverified", func(r *completioncontract.RecordSnapshot) { r.Artifact.VerifiedAt = " " }, "artifact verification"},
		{"no base branch", func(r *completioncontract.RecordSnapshot) { r.BaseBranch = " " }, "prepared target branch"},
		{"unknown provider", func(r *completioncontract.RecordSnapshot) { r.Artifact.Provider = "bitbucket" }, "must be github or gitlab"},
		{"github with mr kind", func(r *completioncontract.RecordSnapshot) { r.Artifact.Kind = "mr" }, "kind must match its provider"},
		{"gitlab with pr kind", func(r *completioncontract.RecordSnapshot) { r.Artifact.Provider = "gitlab" }, "kind must match its provider"},
		{"http scheme", func(r *completioncontract.RecordSnapshot) { r.Artifact.URL = "http://github.com/acme/repo/pull/7" }, "canonical HTTPS"},
		{"embedded whitespace", func(r *completioncontract.RecordSnapshot) { r.Artifact.URL = "https://github.com/acme/repo/pull/7 " }, "canonical HTTPS"},
		{"query string", func(r *completioncontract.RecordSnapshot) { r.Artifact.URL = "https://github.com/acme/repo/pull/7?x=1" }, "canonical HTTPS"},
		{"userinfo", func(r *completioncontract.RecordSnapshot) { r.Artifact.URL = "https://u@github.com/acme/repo/pull/7" }, "canonical HTTPS"},
		{"empty fragment marker", func(r *completioncontract.RecordSnapshot) { r.Artifact.URL = "https://github.com/acme/repo/pull/7#" }, "canonical HTTPS"},
		{"github issue path", func(r *completioncontract.RecordSnapshot) { r.Artifact.URL = "https://github.com/acme/repo/issues/7" }, "GitHub pull request URL"},
		{"github non-numeric pr", func(r *completioncontract.RecordSnapshot) { r.Artifact.URL = "https://github.com/acme/repo/pull/seven" }, "GitHub pull request URL"},
		{"github extra segment", func(r *completioncontract.RecordSnapshot) { r.Artifact.URL = "https://github.com/acme/repo/x/pull/7" }, "GitHub pull request URL"},
		{"gitlab on github.com", func(r *completioncontract.RecordSnapshot) {
			gitlab(r, "https://gitlab.example.com/acme/repo/-/issues/1", "https://github.com/acme/repo/-/merge_requests/7")
		}, "GitLab merge request URL"},
		{"gitlab issue path", func(r *completioncontract.RecordSnapshot) {
			gitlab(r, "https://gitlab.example.com/acme/repo/-/issues/1", "https://gitlab.example.com/acme/repo/-/issues/7")
		}, "GitLab merge request URL"},
		{"gitlab missing separator", func(r *completioncontract.RecordSnapshot) {
			gitlab(r, "https://gitlab.example.com/acme/repo/-/issues/1", "https://gitlab.example.com/acme/repo/merge_requests/7")
		}, "GitLab merge request URL"},
		{"gitlab other project", func(r *completioncontract.RecordSnapshot) {
			gitlab(r, "https://gitlab.example.com/acme/repo/-/issues/1", "https://gitlab.example.com/acme/other/-/merge_requests/7")
		}, "linked issue project"},
		{"empty labels", func(r *completioncontract.RecordSnapshot) { r.Artifact.Labels = nil }, "labels are not canonical"},
		{"duplicate labels", func(r *completioncontract.RecordSnapshot) { r.Artifact.Labels = []string{"a", "a"} }, "labels are not canonical"},
		{"empty assignees", func(r *completioncontract.RecordSnapshot) { r.Artifact.Assignees = nil }, "assignees are not canonical"},
		{"padded assignee", func(r *completioncontract.RecordSnapshot) { r.Artifact.Assignees = []string{"bob "} }, "assignees are not canonical"},
		{"@me assignee", func(r *completioncontract.RecordSnapshot) { r.Artifact.Assignees = []string{"@ME"} }, "verified provider user"},
	} {
		t.Run(test.name, func(t *testing.T) {
			record := currentArtifactRecord()
			test.mutate(&record)
			err := ValidateArtifact(record, record.Artifact.URL)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestValidateArtifactAcceptsGitLabNestedGroupsAndWorkItems(t *testing.T) {
	record := currentArtifactRecord()
	record.IssueURL = "https://gitlab.example.com:8443/acme/team/repo/-/work_items/5"
	record.Artifact.Provider, record.Artifact.Kind = "gitlab", "mr"
	record.Artifact.URL = "https://gitlab.example.com:8443/acme/team/repo/-/merge_requests/9"
	if err := ValidateArtifact(record, " "+record.Artifact.URL+" "); err != nil {
		t.Fatal(err)
	}
	record.Artifact.URL = "https://gitlab.example.com/acme/team/repo/-/merge_requests/9"
	if err := ValidateArtifact(record, record.Artifact.URL); err == nil || !strings.Contains(err.Error(), "linked issue project") {
		t.Fatalf("a different port is a different project: %v", err)
	}
}
