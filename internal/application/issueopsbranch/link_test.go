package issueopsbranch

import (
	"context"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	cycleapp "issueops/internal/application/issueopscycle"
	model "issueops/internal/contract/issueops"
)

type relationStore struct {
	records map[string]model.Record
	locked  bool
	saves   int
}

func (s *relationStore) WithinLock(ctx context.Context, _ string, fn func(context.Context) error) error {
	s.locked = true
	defer func() { s.locked = false }()
	return fn(ctx)
}
func (s *relationStore) Load(id string) (model.Record, error) {
	if !s.locked {
		panic("load outside lock")
	}
	r, ok := s.records[id]
	if !ok {
		return r, os.ErrNotExist
	}
	return r, nil
}
func (s *relationStore) Save(_ context.Context, r model.Record) (model.Record, error) {
	if !s.locked {
		panic("save outside lock")
	}
	s.records[r.ID] = r
	s.saves++
	return r, nil
}
func newRelationStoreForTest(records ...model.Record) (*relationStore, Linker) {
	s := &relationStore{records: map[string]model.Record{}}
	for _, r := range records {
		s.records[r.ID] = r
	}
	return s, Linker{Records: s, Authority: cycleapp.NewMutationAuthority(func(a, b string) bool { return a == b }, liveVerifier()), Now: func() time.Time { return time.Date(2026, 9, 28, 0, 0, 0, 0, time.UTC) }}
}
func TestLinkIssuePersistsURLAndAdvancesReadyRecord(t *testing.T) {
	record := model.Record{
		ID:     "io-link-issue",
		Repo:   "/repo/example",
		Branch: "feature/link-issue",
		Phase:  model.PhaseProblem,
		Intent: &model.IntentContract{RawRequest: "link issue", InterpretedIntent: "link issue", SuccessCriteria: []string{"issue linked"}, IntentClass: "trivial"},
	}
	linkStore, store := newRelationStoreForTest(record)

	got, err := store.Issue(context.Background(), record.ID, " https://github.com/example/repo/issues/10 ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.IssueURL != "https://github.com/example/repo/issues/10" {
		t.Fatalf("IssueURL=%q", got.IssueURL)
	}
	if got.Phase != model.PhasePlan {
		t.Fatalf("Phase=%q, want %q", got.Phase, model.PhasePlan)
	}
	if reloaded := linkStore.records[record.ID]; reloaded.IssueURL != got.IssueURL || reloaded.Phase != got.Phase {
		t.Fatalf("persisted record mismatch: %+v", reloaded)
	}
}

func TestLinkIssueRejectsInvalidURL(t *testing.T) {
	_, store := newRelationStoreForTest(model.Record{ID: "io-bad-url"})
	if _, err := store.Issue(context.Background(), "io-bad-url", "not-a-url", nil); err == nil || !strings.Contains(err.Error(), "http(s) URL") {
		t.Fatalf("expected issue URL validation error, got %v", err)
	}
}

func TestLinkChildPersistsProviderNeutralGraph(t *testing.T) {
	parent := model.Record{
		ID:       "io-parent",
		Repo:     "/repo/example",
		Branch:   "1-demo",
		Phase:    model.PhasePlan,
		IssueURL: "https://github.com/example/repo/issues/10",
	}
	gitlab := model.Record{
		ID:       "io-gitlab",
		Repo:     "/repo/gitlab",
		Branch:   "20-gitlab",
		Phase:    model.PhasePlan,
		IssueURL: "https://gitlab.example/group/project/-/issues/20",
	}
	generic := model.Record{
		ID:       "io-generic",
		Repo:     "/repo/generic",
		Branch:   "10-generic",
		Phase:    model.PhasePlan,
		IssueURL: "https://tracker.example/acme/repo/issues/10",
	}
	linkStore, store := newRelationStoreForTest(parent, gitlab, generic)

	record, err := store.Child(context.Background(), parent.ID, "https://github.com/example/repo/issues/11", "write child graph tests", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.IssueLinks) != 1 {
		t.Fatalf("expected one child issue link, got %+v", record.IssueLinks)
	}
	link := record.IssueLinks[0]
	if link.Type != "child" || link.URL != "https://github.com/example/repo/issues/11" || link.Title != "write child graph tests" || link.Provider != "github" {
		t.Fatalf("unexpected child issue link: %+v", link)
	}
	if link.CreatedAt == "" {
		t.Fatalf("child issue link should record created_at: %+v", link)
	}

	reloaded := linkStore.records[parent.ID]
	if len(reloaded.IssueLinks) != 1 || reloaded.IssueLinks[0].URL != link.URL {
		t.Fatalf("reloaded child issue links mismatch: %+v", reloaded.IssueLinks)
	}
	if _, err := store.Child(context.Background(), parent.ID, link.URL, "duplicate", nil); err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("expected duplicate child link rejection, got %v", err)
	}
	if _, err := store.Child(context.Background(), parent.ID, "https://tracker.example/acme/repo/issues/12", "generic tracker child", nil); err == nil || !strings.Contains(err.Error(), "provider") {
		t.Fatalf("generic child under GitHub parent should be rejected as provider mismatch, got %v", err)
	}
	if _, err := store.Child(context.Background(), parent.ID, "https://github.com/other/repo/issues/12", "other repo child", nil); err == nil || !strings.Contains(err.Error(), "parent issue project") {
		t.Fatalf("GitHub child from another repo should be rejected, got %v", err)
	}
	if _, err := store.Child(context.Background(), parent.ID, "https://github.com/example/repo/issues/not-a-number", "bad child", nil); err == nil || !strings.Contains(err.Error(), "does not end in an issue number") {
		t.Fatalf("GitHub child with nonnumeric issue should be rejected, got %v", err)
	}
	if _, err := store.Child(context.Background(), gitlab.ID, "https://gitlab.example/other/project/-/issues/21", "other project child", nil); err == nil || !strings.Contains(err.Error(), "parent issue project") {
		t.Fatalf("GitLab child from another project should be rejected, got %v", err)
	}
	if _, err := store.Child(context.Background(), gitlab.ID, "https://gitlab.example/group/project/-/issues/not-a-number", "bad child", nil); err == nil || !strings.Contains(err.Error(), "does not end in an issue number") {
		t.Fatalf("GitLab child with nonnumeric issue should be rejected, got %v", err)
	}
	if _, err := store.Child(context.Background(), gitlab.ID, "https://gitlab.example/group/project/-/issues/21", "same project child", nil); err != nil {
		t.Fatalf("GitLab child in same project should be accepted: %v", err)
	}
	gitlabWorkItem := model.Record{
		ID:       "io-gitlab-work-item",
		Repo:     "/repo/gitlab",
		Branch:   "21-gitlab",
		Phase:    model.PhasePlan,
		IssueURL: "https://gitlab.example/group/project/-/issues/20",
	}
	_, workItemStore := newRelationStoreForTest(gitlabWorkItem)
	if _, err := workItemStore.Child(context.Background(), gitlabWorkItem.ID, "https://gitlab.example/group/project/-/work_items/22", "same project work item child", nil); err != nil {
		t.Fatalf("GitLab child work item in same project should be accepted: %v", err)
	}
	generic, err = store.Child(context.Background(), generic.ID, "https://tracker.example/acme/repo/issues/12", "generic tracker child", nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := generic.IssueLinks[0].Provider; got != "" {
		t.Fatalf("generic issue URL should not infer a provider, got %q", got)
	}
	if _, err := store.Child(context.Background(), parent.ID, "not-a-url", "bad", nil); err == nil || !strings.Contains(err.Error(), "child_url") {
		t.Fatalf("expected child URL validation error, got %v", err)
	}
}

func TestLinkRelatedPersistsProviderAndRejectsDuplicates(t *testing.T) {
	record := model.Record{
		ID:       "io-related",
		Repo:     "/repo/example",
		Branch:   "feature/related",
		Phase:    model.PhasePlan,
		IssueURL: "https://github.com/example/repo/issues/10",
	}
	_, store := newRelationStoreForTest(record)

	got, err := store.Related(context.Background(), record.ID, " follows-up ", " https://github.com/example/repo/issues/11 ", " follow up ", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.IssueLinks) != 1 {
		t.Fatalf("IssueLinks=%+v", got.IssueLinks)
	}
	link := got.IssueLinks[0]
	if link.Type != "follows-up" || link.URL != "https://github.com/example/repo/issues/11" || link.Title != "follow up" || link.Provider != "github" {
		t.Fatalf("unexpected related link: %+v", link)
	}
	if link.CreatedAt == "" {
		t.Fatalf("related link should include CreatedAt: %+v", link)
	}
	if _, err := store.Related(context.Background(), record.ID, "follows-up", link.URL, "duplicate", nil); err == nil || !strings.Contains(err.Error(), "already linked") {
		t.Fatalf("expected duplicate related link rejection, got %v", err)
	}
	if _, err := store.Related(context.Background(), record.ID, "parent", "https://github.com/example/repo/issues/12", "bad type", nil); err == nil || !strings.Contains(err.Error(), "invalid link type") {
		t.Fatalf("expected invalid link type rejection, got %v", err)
	}
	if _, err := store.Related(context.Background(), record.ID, "blocks", "not-a-url", "bad url", nil); err == nil || !strings.Contains(err.Error(), "related_url") {
		t.Fatalf("expected related URL validation error, got %v", err)
	}
}

func TestIssueLinkUsesCompleteReadinessAndNeverRegressesPhase(t *testing.T) {
	for _, tc := range []struct {
		name   string
		phase  model.Phase
		intent *model.IntentContract
		want   model.Phase
	}{
		{"missing intent", model.PhaseProblem, nil, model.PhaseProblem},
		{"missing preparation", model.PhaseProblem, &model.IntentContract{RawRequest: "fix", InterpretedIntent: "fix", SuccessCriteria: []string{"fixed"}}, model.PhaseProblem},
		{"later phase", model.PhaseImplement, &model.IntentContract{RawRequest: "fix", InterpretedIntent: "fix", SuccessCriteria: []string{"fixed"}, IntentClass: "trivial"}, model.PhaseImplement},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, s := newRelationStoreForTest(model.Record{ID: "id", Phase: tc.phase, Intent: tc.intent})
			got, err := s.Issue(context.Background(), "id", "https://github.com/acme/repo/issues/1", nil)
			if err != nil || got.Phase != tc.want || got.UpdatedAt != "2026-09-28T00:00:00Z" {
				t.Fatalf("link = %+v, %v", got, err)
			}
		})
	}
}

func TestRelationsAuthorizeBeforeValidationAndDoNotWriteOnRefusal(t *testing.T) {
	holder := model.NativeActor{Host: "codex", SessionID: "holder", SessionProcess: &model.NativeProcessReceipt{PID: 42, StartedAt: "2026-08-28T00:00:00Z", Executable: "/usr/bin/codex"}}
	r, root := retargetActorRecord(t, model.LeaseStatusActive, &holder)
	r.ID = "id"
	r.IssueURL = "https://github.com/acme/repo/issues/1"
	store, s := newRelationStoreForTest(r)
	actor := model.Actor{Host: "codex", SessionID: "other", CWD: root, NativeProcessAncestry: []model.NativeProcessReceipt{*holder.SessionProcess}}
	calls := []func() (model.Record, error){
		func() (model.Record, error) { return s.Issue(context.Background(), "id", "bad", &actor) },
		func() (model.Record, error) {
			return s.Child(context.Background(), "id", "bad", "title", &actor)
		},
		func() (model.Record, error) {
			return s.Related(context.Background(), "id", "bad", "bad", "title", &actor)
		},
	}
	for _, call := range calls {
		if _, err := call(); err == nil || strings.Contains(err.Error(), "URL") || strings.Contains(err.Error(), "invalid link type") {
			t.Fatalf("authority must reject first: %v", err)
		}
	}
	if store.saves != 0 || !reflect.DeepEqual(store.records["id"], r) {
		t.Fatal("refusal changed record")
	}
	actor.SessionID = "holder"
	if _, err := s.Related(context.Background(), "id", "blocks", "https://github.com/acme/repo/issues/2", "title", &actor); err != nil {
		t.Fatal(err)
	}
	before := store.records["id"]
	if _, err := s.Related(context.Background(), "id", "blocks", "https://github.com/acme/repo/issues/2", "duplicate", &actor); err == nil {
		t.Fatal("duplicate accepted")
	}
	if !reflect.DeepEqual(store.records["id"], before) || store.saves != 1 {
		t.Fatal("duplicate changed record")
	}
	if _, err := s.Related(context.Background(), "id", "depends-on", "https://github.com/acme/repo/issues/2", "other relation", &actor); err != nil {
		t.Fatal(err)
	}
	if len(store.records["id"].IssueLinks) != 2 {
		t.Fatal("different relation lost")
	}
}
