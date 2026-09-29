package issueopsremote

import (
	"context"
	"errors"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
	"reflect"
	"strings"
	"testing"
)

type childRecordReader func(context.Context, string) (model.IssueOpsRecord, error)

func (f childRecordReader) Read(ctx context.Context, id string) (model.IssueOpsRecord, error) {
	return f(ctx, id)
}

type childProviderFunc func(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error)

func (f childProviderFunc) CreateChild(req port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
	return f(req)
}

func TestChildCreatorOrdersValidationAuthorityCreationAndLink(t *testing.T) {
	cases := []struct {
		name, problem, wantError string
		confirm, leased          bool
		events                   []string
	}{
		{name: "parent first", problem: "parent", wantError: "linked parent", confirm: true, leased: true, events: []string{"read"}},
		{name: "branch before input", problem: "branch", wantError: "branch prepare", confirm: true, leased: true, events: []string{"read"}},
		{name: "title", problem: "title", wantError: "child title", confirm: true, leased: true, events: []string{"read"}},
		{name: "labels", problem: "labels", wantError: "child label", confirm: true, leased: true, events: []string{"read"}},
		{name: "assignees", problem: "assignees", wantError: "child assignee", confirm: true, leased: true, events: []string{"read"}},
		{name: "body before authority", problem: "body", wantError: "secret-like", confirm: true, leased: true, events: []string{"read", "resolve", "body"}},
		{name: "ancestry before authority", problem: "ancestry", wantError: "ancestry refused", confirm: true, leased: true, events: []string{"read", "resolve", "body", "observe"}},
		{name: "actor before creation", problem: "actor", wantError: "actor refused", confirm: true, leased: true, events: []string{"read", "resolve", "body", "observe", "authorize"}},
		{name: "provider failure", problem: "create", wantError: "provider refused", confirm: true, leased: true, events: []string{"read", "resolve", "body", "observe", "authorize", "create"}},
		{name: "unverified hierarchy", problem: "hierarchy", wantError: "verify child hierarchy", confirm: true, leased: true, events: []string{"read", "resolve", "body", "observe", "authorize", "create"}},
		{name: "missing child url", problem: "url", wantError: "verify child hierarchy", confirm: true, leased: true, events: []string{"read", "resolve", "body", "observe", "authorize", "create"}},
		{name: "link failure", problem: "link", wantError: "link refused", confirm: true, leased: true, events: []string{"read", "resolve", "body", "observe", "authorize", "create", "link"}},
		{name: "preview", leased: true, events: []string{"read", "resolve", "body", "create"}},
		{name: "unleased confirm", confirm: true, events: []string{"read", "resolve", "body", "create", "link"}},
		{name: "leased confirm", confirm: true, leased: true, events: []string{"read", "resolve", "body", "observe", "authorize", "create", "link"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var events []string
			record := model.IssueOpsRecord{ID: "io-child", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/1", Branch: "1-parent", BranchPrepare: &model.IssueOpsBranchPrepare{Branch: "1-parent"}}
			if tc.leased {
				record.Execution = &model.Execution{}
			}
			cmd := ChildCreateCommand{ID: record.ID, Title: "Child", BodyFile: "body.md", Labels: []string{" bug ", "bug"}, Assignees: []string{" owner ", "owner"}, Confirm: tc.confirm, Actor: model.IssueOpsActor{Host: "codex", SessionID: "session", CWD: "/worktree"}}
			switch tc.problem {
			case "parent":
				record.IssueURL = ""
				cmd.Title = ""
			case "branch":
				record.BranchPrepare = nil
				cmd.Title = ""
			case "title":
				cmd.Title = ""
			case "labels":
				cmd.Labels = nil
			case "assignees":
				cmd.Assignees = nil
			}
			ancestry := []model.NativeProcessReceipt{{PID: 321}}
			assertActor := func(actor model.IssueOpsActor) {
				t.Helper()
				if tc.leased {
					want := cmd.Actor
					want.NativeProcessAncestry = ancestry
					if !reflect.DeepEqual(actor, want) {
						t.Fatalf("actor=%+v want=%+v", actor, want)
					}
				} else if !reflect.DeepEqual(actor, model.IssueOpsActor{}) {
					t.Fatalf("unleased actor changed: %+v", actor)
				}
			}
			service := ChildCreator{
				Records: childRecordReader(func(_ context.Context, id string) (model.IssueOpsRecord, error) {
					events = append(events, "read")
					if id != record.ID {
						t.Fatal(id)
					}
					return record, nil
				}),
				Resolve: func(provider string) (ChildProvider, error) {
					events = append(events, "resolve")
					if provider != "github" {
						t.Fatal(provider)
					}
					return childProviderFunc(func(req port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
						events = append(events, "create")
						if req.ParentIssueURL != record.IssueURL || req.Body != "Body" || req.Confirm != tc.confirm || !reflect.DeepEqual(req.Labels, []string{"bug"}) || !reflect.DeepEqual(req.Assignees, []string{"owner"}) {
							t.Fatalf("request=%+v", req)
						}
						result := port.IssueProviderCreateChildResult{HierarchyVerified: true, ChildURL: "https://github.com/acme/repo/issues/2"}
						switch tc.problem {
						case "create":
							return result, errors.New("provider refused")
						case "hierarchy":
							result.HierarchyVerified = false
						case "url":
							result.ChildURL = " "
						}
						return result, nil
					}), nil
				},
				Bodies: NewTemplateBodyResolver(func(path string) ([]byte, error) {
					events = append(events, "body")
					if path != "body.md" {
						t.Fatal(path)
					}
					if tc.problem == "body" {
						return []byte("password=private-value"), nil
					}
					return []byte("Body"), nil
				}),
				Authorize: func(_ context.Context, id string, actor model.IssueOpsActor) error {
					events = append(events, "authorize")
					assertActor(actor)
					if tc.problem == "actor" {
						return errors.New("actor refused")
					}
					return nil
				},
				Link: func(_ context.Context, id, url, title string, actor model.IssueOpsActor) error {
					events = append(events, "link")
					assertActor(actor)
					if id != record.ID || url != "https://github.com/acme/repo/issues/2" || title != "Child" {
						t.Fatalf("link %s %s %s", id, url, title)
					}
					if tc.problem == "link" {
						return errors.New("link refused")
					}
					return nil
				},
			}
			_, err := service.Create(context.Background(), cmd, func() ([]model.NativeProcessReceipt, error) {
				events = append(events, "observe")
				if tc.problem == "ancestry" {
					return nil, errors.New("ancestry refused")
				}
				return ancestry, nil
			})
			if tc.wantError == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("err=%v want=%s", err, tc.wantError)
			}
			if !reflect.DeepEqual(events, tc.events) {
				t.Fatalf("effects=%v want=%v", events, tc.events)
			}
		})
	}
}
