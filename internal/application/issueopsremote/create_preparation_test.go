package issueopsremote

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	contract "issueops/internal/contract/issueopspublication"
)

type preparationObserver struct {
	record            model.IssueOpsRecord
	events            []string
	authorizeError    error
	fingerprint, head string
}

func (o *preparationObserver) inspect(receipt model.NativeProcessReceipt) (string, model.NativeProcessReceipt, error) {
	o.events = append(o.events, "actor")
	return "live", receipt, nil
}
func (o *preparationObserver) Read(context.Context, string) (model.IssueOpsRecord, error) {
	o.events = append(o.events, "read")
	return o.record, nil
}
func (o *preparationObserver) Authorize(context.Context, model.IssueOpsRecord, model.IssueOpsActor) error {
	o.events = append(o.events, "authorize")
	return o.authorizeError
}
func (o *preparationObserver) Fingerprint(context.Context, model.IssueOpsRecord) string {
	o.events = append(o.events, "fingerprint")
	return o.fingerprint
}
func (o *preparationObserver) Head(context.Context, model.IssueOpsRecord) string {
	o.events = append(o.events, "head")
	return o.head
}

func preparationFixture() (*preparationObserver, contract.CreateCommand) {
	process := contract.ProcessReceipt{PID: 42, StartedAt: "then", Executable: "/bin/codex"}
	return &preparationObserver{
		record: model.IssueOpsRecord{ID: "cycle", Phase: model.IssueOpsPhasePR, Repo: "/repo", Branch: "42-work", IssueURL: "https://github.com/team/repo/issues/42",
			BranchPrepare:        &model.IssueOpsBranchPrepare{Provider: "github", BaseBranch: "main"},
			Execution:            &model.Execution{Workspace: model.Workspace{Root: "/worktree", Branch: "42-work"}, Lease: model.WriteLease{Generation: 3, Status: model.LeaseStatusActive}},
			ImplementationReview: &model.IssueOpsImplementationReview{Verdict: "pass", ReviewedFingerprint: "fingerprint"},
		}, fingerprint: "fingerprint", head: strings.Repeat("a", 40),
	}, contract.CreateCommand{Actor: contract.Actor{Host: " CODEX ", SessionID: " session ", SessionProcess: &process, ProcessAncestry: []contract.ProcessReceipt{process}}, ID: "cycle", Provider: "github", Head: "42-work", Base: "main", Title: " title ", Body: " body ", Labels: []string{"bug"}, Assignees: []string{"maintainer"}, ExpectedGeneration: 3, Confirm: true}
}

func TestPreparationOrdersObservationsAndPreservesNormalizedActor(t *testing.T) {
	observer, command := preparationFixture()
	got, err := NewCreatePreparation(observer, observer, observer.inspect).Prepare(context.Background(), command)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(observer.events, ",") != "actor,read,authorize,fingerprint,head" {
		t.Fatalf("events=%v", observer.events)
	}
	if got.Request.Repo != "/worktree" || got.Request.Title != "title" || got.Request.ExpectedHeadSHA != strings.Repeat("a", 40) || !got.Request.Draft || got.Command.Actor.Host != "codex" {
		t.Fatalf("prepared=%+v", got)
	}
}

func TestPreparationRejectsInvalidIdentityBeforeRecordRead(t *testing.T) {
	observer, command := preparationFixture()
	command.Actor.Host = "unsupported"
	_, err := NewCreatePreparation(observer, observer, observer.inspect).Prepare(context.Background(), command)
	if err == nil || !strings.Contains(err.Error(), "native actor host must") || len(observer.events) != 0 {
		t.Fatalf("events=%v err=%v", observer.events, err)
	}
}

func TestPreparationPreviewSkipsExecutionObservations(t *testing.T) {
	observer, command := preparationFixture()
	command.Confirm = false
	observer.record.Execution = nil
	got, err := NewCreatePreparation(observer, observer, observer.inspect).Prepare(context.Background(), command)
	if err != nil || strings.Join(observer.events, ",") != "read" || got.Request.Repo != "/repo" || got.Request.ExpectedHeadSHA != "" {
		t.Fatalf("prepared=%+v events=%v err=%v", got, observer.events, err)
	}
}

func TestPreparationStopsBeforeUnnecessaryEffects(t *testing.T) {
	for _, tc := range []struct {
		name         string
		change       func(*preparationObserver, *contract.CreateCommand)
		want, events string
	}{
		{"generation", func(_ *preparationObserver, c *contract.CreateCommand) { c.ExpectedGeneration = 2 }, "stale lease generation", "actor,read"},
		{"actor", func(o *preparationObserver, _ *contract.CreateCommand) { o.authorizeError = errors.New("wrong actor") }, "wrong actor", "actor,read,authorize"},
		{"pending", func(o *preparationObserver, _ *contract.CreateCommand) {
			o.record.Execution.Pending = &model.ExternalIntent{}
		}, "already pending", "actor,read,authorize"},
		{"stale review", func(o *preparationObserver, _ *contract.CreateCommand) { o.fingerprint = "changed" }, "implementation_review_stale", "actor,read,authorize,fingerprint"},
		{"missing fingerprint", func(o *preparationObserver, _ *contract.CreateCommand) { o.fingerprint = "" }, "freshness", "actor,read,authorize,fingerprint"},
		{"branch", func(_ *preparationObserver, c *contract.CreateCommand) { c.Base = "other" }, "linked issue authority", "actor,read,authorize,fingerprint"},
		{"head", func(o *preparationObserver, _ *contract.CreateCommand) { o.head = "" }, "resolvable canonical worktree HEAD", "actor,read,authorize,fingerprint,head"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o, c := preparationFixture()
			tc.change(o, &c)
			_, err := NewCreatePreparation(o, o, o.inspect).Prepare(context.Background(), c)
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Join(o.events, ",") != tc.events {
				t.Fatalf("events=%v err=%v", o.events, err)
			}
		})
	}
}
