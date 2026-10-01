package issueopsremote

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
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
						if req.ParentIssueURL != record.IssueURL || strings.TrimSpace(strings.Split(req.Body, "<!-- issueops:child-create:")[0]) != readableChildBody || req.Confirm != tc.confirm || !reflect.DeepEqual(req.Labels, []string{"bug"}) || !reflect.DeepEqual(req.Assignees, []string{"owner"}) {
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
						return []byte(readableChildBody + "\npassword=private-value"), nil
					}
					return []byte(readableChildBody), nil
				}),
				Authorize: func(_ context.Context, id string, actor model.IssueOpsActor) error {
					events = append(events, "authorize")
					assertActor(actor)
					if tc.problem == "actor" {
						return errors.New("actor refused")
					}
					return nil
				},
				Intents: &ChildCreateIntents{Store: &testChildStore{record: record, before: func(updated model.IssueOpsRecord) error {
					if len(updated.ChildCreateOperations) > 0 && updated.ChildCreateOperations[0].Status == model.IssueCreateIntentCompleted {
						events = append(events, "link")
						if len(updated.IssueLinks) != 1 || updated.IssueLinks[0].URL != "https://github.com/acme/repo/issues/2" {
							t.Fatalf("links=%+v", updated.IssueLinks)
						}
						if tc.problem == "link" {
							return errors.New("link refused")
						}
					}
					return nil
				}}, Authority: testChildAuthority{}, Now: time.Now},
				NewOperationID: func() (string, error) { return strings.Repeat("a", 32), nil },
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

const readableChildBody = `## 요약

부모 이슈 #1234에서 템플릿 렌더러 구현을 맡습니다. 끝나면 렌더러가 새 계약의 필수 절을 출력합니다.

## 완료 기준

- 렌더러 테스트가 필수 절 순서를 확인합니다.

## 범위

- 하는 것: 렌더러 구현
- 하지 않는 것: provider 정책 변경

## 선행 조건과 병합 조건

부모 브랜치에 병합한 뒤 하위 작업을 닫습니다.`

func TestChildCreatorRestartAfterReceiptFailureDoesNotCreateAgain(t *testing.T) {
	record := model.IssueOpsRecord{ID: "io-child", Repo: "/repo", IssueURL: "https://github.com/acme/repo/issues/1", Branch: "1-parent", BranchPrepare: &model.IssueOpsBranchPrepare{Branch: "1-parent"}}
	calls := 0
	store := &testChildStore{record: record, before: func(updated model.IssueOpsRecord) error {
		if updated.ChildCreateOperations[0].Status == model.IssueCreateIntentCompleted {
			return errors.New("disk full")
		}
		return nil
	}}
	service := ChildCreator{
		Records: childRecordReader(func(context.Context, string) (model.IssueOpsRecord, error) { return store.read(), nil }),
		Resolve: func(string) (ChildProvider, error) {
			return childProviderFunc(func(port.IssueProviderCreateChildRequest) (port.IssueProviderCreateChildResult, error) {
				calls++
				return port.IssueProviderCreateChildResult{ChildURL: "https://github.com/acme/repo/issues/2", HierarchyVerified: true}, nil
			}), nil
		},
		Bodies:         NewTemplateBodyResolver(nil),
		Intents:        &ChildCreateIntents{Store: store, Authority: testChildAuthority{}, Now: time.Now},
		NewOperationID: func() (string, error) { return strings.Repeat("a", 32), nil },
	}
	cmd := ChildCreateCommand{ID: record.ID, Title: "Child", Body: readableChildBody, Labels: []string{"bug"}, Assignees: []string{"owner"}, Confirm: true}
	for range 2 {
		if _, err := service.Create(context.Background(), cmd, nil); err == nil {
			t.Fatal("expected receipt/recovery error")
		}
	}
	if calls != 1 {
		t.Fatalf("create count after retry=%d; want 1", calls)
	}
}

type testChildAuthority struct{}

func (testChildAuthority) Authorize(context.Context, model.IssueOpsRecord, model.IssueOpsActor) error {
	return nil
}

type testChildStore struct {
	mu     sync.Mutex
	record model.IssueOpsRecord
	before func(model.IssueOpsRecord) error
}

func (s *testChildStore) read() model.IssueOpsRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	return cloneChildRecord(s.record)
}
func (s *testChildStore) Read(context.Context, string) (model.IssueOpsRecord, error) {
	return s.read(), nil
}
func (s *testChildStore) Update(ctx context.Context, id string, transition RecordTransition) (model.IssueOpsRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	updated, err := transition(cloneChildRecord(s.record))
	if err != nil {
		return s.record, err
	}
	if s.before != nil {
		if err := s.before(updated); err != nil {
			return s.record, err
		}
	}
	s.record = cloneChildRecord(updated)
	return cloneChildRecord(updated), nil
}
func cloneChildRecord(r model.IssueOpsRecord) model.IssueOpsRecord {
	b, _ := json.Marshal(r)
	var out model.IssueOpsRecord
	_ = json.Unmarshal(b, &out)
	return out
}
