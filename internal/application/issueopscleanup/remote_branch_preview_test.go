package issueopscleanup

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

type previewBranchEnv struct {
	origin      string
	originErr   error
	remoteOID   string
	remoteErr   error
	tipReached  bool
	tipCalls    []string
	remoteCalls int
}

func (e *previewBranchEnv) RemoteRef(context.Context, string, string) (string, error) {
	e.remoteCalls++
	return e.remoteOID, e.remoteErr
}
func (e *previewBranchEnv) OriginURL(context.Context, string) (string, error) {
	return e.origin, e.originErr
}
func (e *previewBranchEnv) TipReachedBase(_ context.Context, _, oid, base string) bool {
	e.tipCalls = append(e.tipCalls, oid+"@"+base)
	return e.tipReached
}
func (e *previewBranchEnv) Delete(context.Context, string, string, string) error {
	return errors.New("preview must never delete")
}

const (
	originalPR    = "https://github.com/acme/repo/pull/1"
	replacementPR = "https://github.com/acme/repo/pull/3"
)

func mergedHeadFor(oid string) func(model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
	return func(model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
		return model.CleanupRemoteBranchArtifactHead{HeadRefName: "123-work", HeadRefOID: oid}, nil
	}
}

func unmergedArtifact(model.IssueOpsRemoteArtifactVerification) (model.CleanupRemoteBranchArtifactHead, error) {
	return model.CleanupRemoteBranchArtifactHead{}, errors.New("pull request is not merged")
}

func branchPreviewRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID: "io-test", Repo: "/repo", Branch: "123-work", Phase: model.IssueOpsPhaseDone,
		BranchPrepare:  &model.IssueOpsBranchPrepare{BaseBranch: "main"},
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: originalPR},
	}
}

func newPreviewBranchEnv() *previewBranchEnv {
	return &previewBranchEnv{origin: "git@github.com:acme/repo.git", remoteOID: "tip"}
}

func mergedReplacement(body string) func(string) (domain.ArtifactObservation, error) {
	return func(url string) (domain.ArtifactObservation, error) {
		return domain.ArtifactObservation{URL: url, Provider: "github", Merged: true, State: "MERGED", Body: body}, nil
	}
}

func TestRemoteBranchPreviewerAcceptsSupersedingArtifactWhenOriginalIsUnmerged(t *testing.T) {
	env := newPreviewBranchEnv()
	previewer := RemoteBranchPreviewer{Environment: env, VerifyMergedArtifact: unmergedArtifact, ObserveArtifact: mergedReplacement("Supersedes #1")}
	inventory, result := previewer.Plan(t.Context(), branchPreviewRecord(), model.CleanupRemoteBranchRequest{SupersededBy: " " + replacementPR + " "})
	if !result.OK || len(result.Missing) != 0 {
		t.Fatalf("result=%+v", result)
	}
	if inventory.SupersededBy != replacementPR || result.SupersededBy != replacementPR {
		t.Fatalf("superseding artifact not bound: inventory=%q result=%q", inventory.SupersededBy, result.SupersededBy)
	}
	if !result.RemoteBranchPresent || inventory.RemoteOID != "tip" {
		t.Fatalf("remote branch facts lost: %+v", inventory)
	}
}

func TestRemoteBranchPreviewerRejectsUnprovableSupersede(t *testing.T) {
	cases := []struct {
		name         string
		supersededBy string
		observe      func(string) (domain.ArtifactObservation, error)
		wantErr      string
	}{
		{name: "none provided", observe: mergedReplacement("Supersedes #1"), wantErr: "no superseding artifact was provided"},
		{name: "observation not configured", supersededBy: replacementPR, wantErr: "provider observation is not configured"},
		{name: "observation fails", supersededBy: replacementPR, observe: func(string) (domain.ArtifactObservation, error) {
			return domain.ArtifactObservation{}, errors.New("timeout")
		}, wantErr: "could not be observed: timeout"},
		{name: "replacement not merged", supersededBy: replacementPR, observe: func(url string) (domain.ArtifactObservation, error) {
			return domain.ArtifactObservation{URL: url, State: "OPEN", Body: "Supersedes #1"}, nil
		}, wantErr: "is not merged"},
		{name: "replacement does not declare supersede", supersededBy: replacementPR, observe: mergedReplacement("unrelated change"), wantErr: "does not declare that it supersedes"},
		{name: "replacement from another project", supersededBy: "https://github.com/acme/other/pull/3", observe: mergedReplacement("Supersedes #1"), wantErr: "not the original project"},
		{name: "replacement is the original", supersededBy: originalPR, observe: mergedReplacement("Supersedes #1"), wantErr: "must differ from the original"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previewer := RemoteBranchPreviewer{Environment: newPreviewBranchEnv(), VerifyMergedArtifact: unmergedArtifact, ObserveArtifact: tc.observe}
			inventory, result := previewer.Plan(t.Context(), branchPreviewRecord(), model.CleanupRemoteBranchRequest{SupersededBy: tc.supersededBy})
			if result.OK || !slices.Contains(result.Missing, "remote_artifact_merged") || inventory.SupersededBy != "" {
				t.Fatalf("unproven supersede accepted: inventory=%+v result=%+v", inventory, result)
			}
			if !strings.Contains(result.SupersedeError, tc.wantErr) {
				t.Fatalf("SupersedeError=%q, want %q", result.SupersedeError, tc.wantErr)
			}
		})
	}
}

func TestRemoteBranchPreviewerRequiresTipEvidenceUnlessMergedHeadMatches(t *testing.T) {
	record := branchPreviewRecord()
	t.Run("merged head equals remote tip needs no ancestry probe", func(t *testing.T) {
		env := newPreviewBranchEnv()
		_, result := (RemoteBranchPreviewer{Environment: env, VerifyMergedArtifact: mergedHeadFor("TIP")}).Plan(t.Context(), record, model.CleanupRemoteBranchRequest{})
		if !result.OK || len(env.tipCalls) != 0 {
			t.Fatalf("result=%+v tipCalls=%v", result, env.tipCalls)
		}
	})
	t.Run("advanced tip reachable from base", func(t *testing.T) {
		env := newPreviewBranchEnv()
		env.tipReached = true
		_, result := (RemoteBranchPreviewer{Environment: env, VerifyMergedArtifact: mergedHeadFor("older")}).Plan(t.Context(), record, model.CleanupRemoteBranchRequest{})
		if !result.OK || !result.RemoteTipReachedBase || !slices.Equal(env.tipCalls, []string{"tip@main"}) {
			t.Fatalf("result=%+v tipCalls=%v", result, env.tipCalls)
		}
	})
	t.Run("advanced tip unreachable and unsuperseded", func(t *testing.T) {
		env := newPreviewBranchEnv()
		_, result := (RemoteBranchPreviewer{Environment: env, VerifyMergedArtifact: mergedHeadFor("older")}).Plan(t.Context(), record, model.CleanupRemoteBranchRequest{})
		if result.OK || !slices.Contains(result.Missing, "remote_tip_equals_merged_head") || result.RemoteTipReachedBase {
			t.Fatalf("result=%+v", result)
		}
	})
	t.Run("advanced tip covered by superseding artifact", func(t *testing.T) {
		env := newPreviewBranchEnv()
		previewer := RemoteBranchPreviewer{Environment: env, VerifyMergedArtifact: mergedHeadFor("older"), ObserveArtifact: mergedReplacement("This replaces #1")}
		inventory, result := previewer.Plan(t.Context(), record, model.CleanupRemoteBranchRequest{SupersededBy: replacementPR})
		if !result.OK || inventory.SupersededBy != replacementPR {
			t.Fatalf("inventory=%+v result=%+v", inventory, result)
		}
	})
	t.Run("no recorded base branch skips ancestry probe", func(t *testing.T) {
		env := newPreviewBranchEnv()
		noBase := branchPreviewRecord()
		noBase.BranchPrepare = nil
		_, result := (RemoteBranchPreviewer{Environment: env, VerifyMergedArtifact: mergedHeadFor("older")}).Plan(t.Context(), noBase, model.CleanupRemoteBranchRequest{})
		if result.OK || len(env.tipCalls) != 0 {
			t.Fatalf("result=%+v tipCalls=%v", result, env.tipCalls)
		}
	})
}

func TestRemoteBranchPreviewerChecksOriginMatchesArtifactProject(t *testing.T) {
	record := branchPreviewRecord()
	for name, configure := range map[string]func(*previewBranchEnv){
		"origin is another project": func(e *previewBranchEnv) { e.origin = "git@github.com:acme/other.git" },
		"origin unreadable":         func(e *previewBranchEnv) { e.originErr = errors.New("no origin") },
	} {
		t.Run(name, func(t *testing.T) {
			env := newPreviewBranchEnv()
			configure(env)
			_, result := (RemoteBranchPreviewer{Environment: env, VerifyMergedArtifact: mergedHeadFor("tip")}).Plan(t.Context(), record, model.CleanupRemoteBranchRequest{})
			if result.OK || !slices.Contains(result.Missing, "remote_identity_match") || result.RemoteIdentityError == "" {
				t.Fatalf("result=%+v", result)
			}
		})
	}
}

func TestRemoteBranchPreviewerWithoutMergeVerificationRefuses(t *testing.T) {
	env := newPreviewBranchEnv()
	_, result := (RemoteBranchPreviewer{Environment: env}).Plan(t.Context(), branchPreviewRecord(), model.CleanupRemoteBranchRequest{})
	if result.OK || !slices.Contains(result.Missing, "remote_artifact_merged") || result.ArtifactError != "merge verification is not configured" {
		t.Fatalf("result=%+v", result)
	}
}

func TestRemoteBranchPreviewerSkipsRemoteReadWithoutRecordedBranch(t *testing.T) {
	env := newPreviewBranchEnv()
	record := branchPreviewRecord()
	record.Branch = ""
	_, result := (RemoteBranchPreviewer{Environment: env, VerifyMergedArtifact: mergedHeadFor("tip")}).Plan(t.Context(), record, model.CleanupRemoteBranchRequest{})
	if env.remoteCalls != 0 || !slices.Contains(result.Missing, "branch_recorded") {
		t.Fatalf("remoteCalls=%d result=%+v", env.remoteCalls, result)
	}
}
