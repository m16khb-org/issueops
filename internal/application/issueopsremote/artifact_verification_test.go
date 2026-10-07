package issueopsremote

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	model "issueops/internal/contract/issueops"
)

type artifactStoreForTest struct {
	records map[string]model.IssueOpsRecord
}

func newArtifactStoreForTest(record model.IssueOpsRecord) (*artifactStoreForTest, *ArtifactVerificationService) {
	store := &artifactStoreForTest{records: map[string]model.IssueOpsRecord{record.ID: record}}
	return store, NewArtifactVerificationService(store, artifactAuthorityForTest{}, nil, nil, time.Now)
}

func (s *artifactStoreForTest) Read(_ context.Context, id string) (model.IssueOpsRecord, error) {
	record, ok := s.records[id]
	if !ok {
		return model.IssueOpsRecord{OK: false, ID: id}, os.ErrNotExist
	}
	record.OK = true
	return record, nil
}

func (s *artifactStoreForTest) Update(ctx context.Context, id string, transition RecordTransition) (model.IssueOpsRecord, error) {
	record, err := s.Read(ctx, id)
	if err != nil {
		return record, err
	}
	record, err = transition(record)
	if err != nil {
		return model.IssueOpsRecord{}, err
	}
	record.OK = true
	s.records[record.ID] = record
	return record, nil
}

func TestValidateChecksWithoutPersisting(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-123456789abc",
		Repo:     t.TempDir(),
		Branch:   "1-demo",
		Phase:    model.IssueOpsPhasePR,
		IssueURL: "https://github.com/example/repo/issues/1",
	}
	fake, store := newArtifactStoreForTest(record)

	got, err := store.Validate(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pull_request",
		URL:       "https://github.com/example/repo/pull/7",
		Labels:    []string{" enhancement ", "issueops"},
		Assignees: []string{" sample "},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != record.ID || got.RemoteArtifact != nil {
		t.Fatalf("validate should return original record without persisting artifact: %+v", got)
	}
	if persisted := fake.records[record.ID]; persisted.RemoteArtifact != nil {
		t.Fatalf("validate should not persist artifact, got %+v", persisted.RemoteArtifact)
	}
}

func TestValidateRejectsInvalidRecord(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-123456789abc",
		Repo:     t.TempDir(),
		Branch:   "1-demo",
		Phase:    model.IssueOpsPhaseFeedback,
		IssueURL: "https://github.com/example/repo/issues/1",
	}
	_, store := newArtifactStoreForTest(record)

	got, err := store.Validate(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/7",
		Labels:    []string{"issueops"},
		Assignees: []string{"sample"},
	})
	if err == nil || !strings.Contains(err.Error(), "before pr phase") {
		t.Fatalf("expected phase validation error, got record %+v err %v", got, err)
	}
	if got.OK {
		t.Fatalf("invalid validation should return ok=false record: %+v", got)
	}
}

func TestVerifyRemoteArtifactURLMatchesProvider(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-123456789abc",
		Repo:     t.TempDir(),
		Branch:   "2-gitlab-mr",
		Phase:    model.IssueOpsPhasePR,
		IssueURL: "https://gitlab.example/group/project/-/issues/2",
	}
	fake, store := newArtifactStoreForTest(record)

	if _, err := store.Record(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "github",
		Kind:      "pr",
		URL:       "https://github.com/example/repo/pull/9",
		Labels:    []string{"issueops"},
		Assignees: []string{"sample"},
	}, model.IssueOpsActor{}); err == nil || !strings.Contains(err.Error(), "match linked issue provider") {
		t.Fatalf("expected provider mismatch error, got %v", err)
	}
	if _, err := store.Record(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "pr",
		URL:       "https://gitlab.example/group/project/-/merge_requests/4",
		Labels:    []string{"issueops"},
		Assignees: []string{"sample"},
	}, model.IssueOpsActor{}); err == nil || !strings.Contains(err.Error(), "gitlab remote artifact kind must be mr") {
		t.Fatalf("expected gitlab kind error, got %v", err)
	}
	got, err := store.Record(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "merge_request",
		URL:       "https://gitlab.example/group/project/-/merge_requests/4",
		Labels:    []string{"issueops"},
		Assignees: []string{"sample"},
	}, model.IssueOpsActor{})
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteArtifact == nil || got.RemoteArtifact.Provider != "gitlab" || got.RemoteArtifact.Kind != "mr" {
		t.Fatalf("expected gitlab mr artifact, got %+v", got.RemoteArtifact)
	}
	if persisted := fake.records[record.ID]; persisted.RemoteArtifact == nil || persisted.RemoteArtifact.Provider != "gitlab" {
		t.Fatalf("expected persisted gitlab artifact, got %+v", persisted.RemoteArtifact)
	}
}

func TestVerifyGitLabRemoteArtifactURLShape(t *testing.T) {
	record := model.IssueOpsRecord{
		ID:       "io-123456789abc",
		Repo:     t.TempDir(),
		Branch:   "2-gitlab-mr",
		Phase:    model.IssueOpsPhasePR,
		IssueURL: "https://gitlab.example/group/project/-/issues/2",
	}
	_, store := newArtifactStoreForTest(record)

	if _, err := store.Record(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://github.com/example/repo/pull/2",
		Labels:    []string{"bug"},
		Assignees: []string{"sample"},
	}, model.IssueOpsActor{}); err == nil || !strings.Contains(err.Error(), "GitLab merge request URL") {
		t.Fatalf("gitlab remote artifact should reject GitHub PR URL, got %v", err)
	}
	if _, err := store.Record(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://gitlab.example/group/project/-/merge_requests/not-a-number",
		Labels:    []string{"bug"},
		Assignees: []string{"100"},
	}, model.IssueOpsActor{}); err == nil || !strings.Contains(err.Error(), "GitLab merge request URL") {
		t.Fatalf("gitlab remote artifact should reject nonnumeric MR URL, got %v", err)
	}
	if _, err := store.Record(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://gitlab.example/other/project/-/merge_requests/2",
		Labels:    []string{"bug"},
		Assignees: []string{"100"},
	}, model.IssueOpsActor{}); err == nil || !strings.Contains(err.Error(), "linked issue project") {
		t.Fatalf("gitlab remote artifact should reject MR URL from another project, got %v", err)
	}
	if _, err := store.Record(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://gitlab.example/group/project/-/merge_requests/2",
		Labels:    []string{"bug"},
		Assignees: []string{"self"},
	}, model.IssueOpsActor{}); err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("gitlab remote artifact should reject placeholder assignee, got %v", err)
	}
	got, err := store.Record(context.Background(), record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
		Provider:  "gitlab",
		Kind:      "mr",
		URL:       "https://gitlab.example/group/project/-/merge_requests/2",
		Labels:    []string{"bug"},
		Assignees: []string{"sample"},
	}, model.IssueOpsActor{})
	if err != nil {
		t.Fatalf("gitlab remote artifact should accept GitLab MR URL: %v", err)
	}
	if got.RemoteArtifact == nil || got.RemoteArtifact.Provider != "gitlab" || got.RemoteArtifact.Kind != "mr" {
		t.Fatalf("unexpected gitlab remote artifact: %+v", got.RemoteArtifact)
	}
}

func TestVerifyRejectsMalformedLinkedIssue(t *testing.T) {
	for _, tt := range []struct {
		name   string
		record model.IssueOpsRecord
		want   string
	}{
		{
			name: "malformed linked issue project",
			record: model.IssueOpsRecord{ID: "io-malformed", Phase: model.IssueOpsPhasePR,
				IssueURL: "https://gitlab.example/-/issues/2"},
			want: "project authority",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, store := newArtifactStoreForTest(tt.record)
			_, err := store.Record(context.Background(), tt.record.ID, model.IssueOpsRemoteArtifactVerificationRequest{
				Provider: "gitlab", Kind: "mr", URL: "https://gitlab.example/group/project/-/merge_requests/2",
				Labels: []string{"bug"}, Assignees: []string{"sample"}, TargetBranch: "main",
			}, model.IssueOpsActor{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Verify error = %v, want %q", err, tt.want)
			}
		})
	}
}

func (s *artifactStoreForTest) WithinTransaction(ctx context.Context, _ string, fn func(context.Context) error) error {
	return fn(ctx)
}

type artifactAuthorityForTest struct{}

func (artifactAuthorityForTest) Authorize(context.Context, model.IssueOpsRecord, model.IssueOpsActor) error {
	return nil
}

// cancellationAwareArtifactStore mirrors sqlstore's span: a canceled context
// fails before the transaction body runs.
type cancellationAwareArtifactStore struct {
	*artifactStoreForTest
	reads int
}

func (s *cancellationAwareArtifactStore) WithinTransaction(ctx context.Context, id string, fn func(context.Context) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.artifactStoreForTest.WithinTransaction(ctx, id, fn)
}

func (s *cancellationAwareArtifactStore) Read(ctx context.Context, id string) (model.IssueOpsRecord, error) {
	s.reads++
	return s.artifactStoreForTest.Read(ctx, id)
}

func TestValidateHonorsCallerCancellation(t *testing.T) {
	store := &cancellationAwareArtifactStore{artifactStoreForTest: &artifactStoreForTest{records: map[string]model.IssueOpsRecord{"io-1": {ID: "io-1"}}}}
	service := NewArtifactVerificationService(store, artifactAuthorityForTest{}, nil, nil, time.Now)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := service.Validate(ctx, "io-1", model.IssueOpsRemoteArtifactVerificationRequest{})
	if !errors.Is(err, context.Canceled) || store.reads != 0 {
		t.Fatalf("Validate err=%v reads=%d, want context.Canceled before any read", err, store.reads)
	}
}
