package issueopsremote

import (
	"context"
	"errors"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type fakeCommenter struct {
	posts []string
	err   error
}

func (f *fakeCommenter) Comment(_ context.Context, _, target, kind, body string) (string, error) {
	f.posts = append(f.posts, kind+" "+target+" "+body)
	return target + "#issuecomment-1", f.err
}

func runComment(t *testing.T, record model.IssueOpsRecord, commenter *fakeCommenter, on string, confirm bool) (model.RemoteCommentResult, error) {
	t.Helper()
	service := NewRemoteCommentService(mergeRecordReader{record}, func(string) (port.IssueProviderCommenter, error) { return commenter, nil })
	return service.Comment(context.Background(), record.ID, on, "리뷰 부탁드립니다.", "", confirm)
}

func TestRemoteCommentPreviewsThenPostsOnTheCycleTarget(t *testing.T) {
	record := completedMergeRecord()
	record.IssueURL = "https://github.com/acme/repo/issues/3"
	commenter := &fakeCommenter{}
	preview, err := runComment(t, record, commenter, "issue", false)
	if err != nil || !preview.OK || preview.Posted || len(commenter.posts) != 0 || preview.TargetURL != record.IssueURL {
		t.Fatalf("preview = %+v, err = %v, posts = %v", preview, err, commenter.posts)
	}
	posted, err := runComment(t, record, commenter, "pr", true)
	if err != nil || !posted.Posted || len(commenter.posts) != 1 || !strings.HasPrefix(commenter.posts[0], "pr "+record.RemoteArtifact.URL) {
		t.Fatalf("posted = %+v, err = %v, posts = %v", posted, err, commenter.posts)
	}
}

func TestRemoteCommentRefusesAMissingTargetAndWarnsAfterAFailedPost(t *testing.T) {
	record := completedMergeRecord()
	record.IssueURL = "https://github.com/acme/repo/issues/3"
	record.RemoteArtifact = nil
	if _, err := runComment(t, record, &fakeCommenter{}, "pr", true); err == nil || !strings.Contains(err.Error(), "no verified PR/MR") {
		t.Fatalf("missing PR err = %v", err)
	}
	failing := &fakeCommenter{err: errors.New("502")}
	if _, err := runComment(t, record, failing, "issue", true); err == nil || !strings.Contains(err.Error(), "may have been posted") {
		t.Fatalf("failed post err = %v", err)
	}
}

// A --body-file edited after the preview no longer matches the previewed
// digest, so confirm refuses it without posting.
func TestRemoteCommentRefusesTextThatChangedSinceThePreview(t *testing.T) {
	record := completedMergeRecord()
	record.IssueURL = "https://github.com/acme/repo/issues/3"
	commenter := &fakeCommenter{}
	service := NewRemoteCommentService(mergeRecordReader{record}, func(string) (port.IssueProviderCommenter, error) { return commenter, nil })
	preview, err := service.Comment(context.Background(), record.ID, "issue", "리뷰 부탁드립니다.", "", false)
	if err != nil || preview.BodySHA256 == "" {
		t.Fatalf("preview = %+v, err = %v", preview, err)
	}
	if _, err := service.Comment(context.Background(), record.ID, "issue", "다른 내용입니다.", preview.BodySHA256, true); err == nil || !strings.Contains(err.Error(), "changed since the preview") || len(commenter.posts) != 0 {
		t.Fatalf("err = %v, posts = %v", err, commenter.posts)
	}
	if _, err := service.Comment(context.Background(), record.ID, "issue", "리뷰 부탁드립니다.", preview.BodySHA256, true); err != nil || len(commenter.posts) != 1 {
		t.Fatalf("matching confirm err = %v, posts = %v", err, commenter.posts)
	}
}
