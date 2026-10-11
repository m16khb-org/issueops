package issueopscleanup_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	app "issueops/internal/application/issueopscleanup"
	model "issueops/internal/contract/issueops"
	domain "issueops/internal/domain/issueops"
)

const (
	finishOriginalPR    = "https://github.com/acme/repo/pull/1"
	finishReplacementPR = "https://github.com/acme/repo/pull/3"
)

func finishSupersedeRecord() model.IssueOpsRecord {
	return model.IssueOpsRecord{
		ID: "cycle", Repo: "/repo", Phase: model.IssueOpsPhaseDone,
		RemoteArtifact: &model.IssueOpsRemoteArtifactVerification{Provider: "github", Kind: "pr", URL: finishOriginalPR},
	}
}

func finishSupersedeRequest(candidate string) model.CleanupFinishRequest {
	return model.CleanupFinishRequest{CompletionReflected: true, IssueClosed: true, SupersededBy: candidate}
}

func TestFinishPreviewAcceptsSupersedingArtifactInsteadOfMergeEvidence(t *testing.T) {
	env := &finishPreviewEnvironment{}
	previewer := app.FinishPreviewer{Environment: env, ObserveArtifact: func(url string) (domain.ArtifactObservation, error) {
		return domain.ArtifactObservation{URL: url, Merged: true, State: "MERGED", Body: "Supersedes #1"}, nil
	}}
	record := finishSupersedeRecord()
	record.BranchPrepare = &model.IssueOpsBranchPrepare{BaseBranch: "parent"}
	req := finishSupersedeRequest(" " + finishReplacementPR + " ")
	req.MergedBaseBranch = "main"
	inventory, result := previewer.Plan(context.Background(), record, req)
	if !result.OK || inventory.SupersededBy != finishReplacementPR || result.SupersedeError != "" {
		t.Fatalf("inventory=%+v result=%+v", inventory, result)
	}
	if len(env.calls) != 0 {
		t.Fatalf("base refs observed although the replacement already proves the merge: %v", env.calls)
	}
}

func TestFinishPreviewRefusesUnprovableSupersedingArtifact(t *testing.T) {
	merged := func(body string) func(string) (domain.ArtifactObservation, error) {
		return func(url string) (domain.ArtifactObservation, error) {
			return domain.ArtifactObservation{URL: url, Merged: true, State: "MERGED", Body: body}, nil
		}
	}
	cases := []struct {
		name      string
		candidate string
		observe   func(string) (domain.ArtifactObservation, error)
		want      string
	}{
		{name: "no candidate", observe: merged("Supersedes #1"), want: "no superseding artifact was provided"},
		{name: "observation not configured", candidate: finishReplacementPR, want: "provider observation is not configured"},
		{name: "observation fails", candidate: finishReplacementPR, observe: func(string) (domain.ArtifactObservation, error) {
			return domain.ArtifactObservation{}, errors.New("rate limited")
		}, want: "could not be observed: rate limited"},
		{name: "replacement unmerged", candidate: finishReplacementPR, observe: func(url string) (domain.ArtifactObservation, error) {
			return domain.ArtifactObservation{URL: url, State: "OPEN", Body: "Supersedes #1"}, nil
		}, want: "is not merged"},
		{name: "replacement silent about original", candidate: finishReplacementPR, observe: merged("drive-by fix"), want: "does not declare that it supersedes"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previewer := app.FinishPreviewer{Environment: &finishPreviewEnvironment{}, ObserveArtifact: tc.observe}
			inventory, result := previewer.Plan(context.Background(), finishSupersedeRecord(), finishSupersedeRequest(tc.candidate))
			if result.OK || inventory.SupersededBy != "" || !slices.Contains(result.Missing, "remote_artifact_merged") || !strings.Contains(result.SupersedeError, tc.want) {
				t.Fatalf("inventory=%+v result=%+v", inventory, result)
			}
		})
	}
}

func TestFinishPreviewClassifiesMergedBaseFromOriginRefs(t *testing.T) {
	const symrefMain = "ref: refs/heads/main\tHEAD\nabc\tHEAD\n"
	cases := []struct {
		name         string
		headsCode    int
		headsOut     string
		symrefCode   int
		symrefOut    string
		wantMissing  []string
		wantRetarget bool
	}{
		{name: "prepared base gone and default branch merged into", symrefOut: symrefMain, wantRetarget: true},
		{name: "prepared base still exists remotely", headsOut: "abc\trefs/heads/parent", symrefOut: symrefMain, wantMissing: []string{"base_branch_drifted"}},
		{name: "prepared base lookup fails", headsCode: 2, wantMissing: []string{"base_branch_drifted", "merged_base_remote_unobserved"}},
		{name: "default branch lookup fails", symrefCode: 2, wantMissing: []string{"base_branch_drifted", "merged_base_remote_unobserved"}},
		{name: "no symref line", symrefOut: "abc\tHEAD\n", wantMissing: []string{"base_branch_drifted", "merged_base_remote_unobserved"}},
		{name: "empty symref branch", symrefOut: "ref: refs/heads/\tHEAD\n", wantMissing: []string{"base_branch_drifted", "merged_base_remote_unobserved"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := &finishPreviewEnvironment{run: func(args []string) (int, string) {
				if args[0] != "ls-remote" {
					return 0, ""
				}
				if args[1] == "--symref" {
					return tc.symrefCode, tc.symrefOut
				}
				return tc.headsCode, tc.headsOut
			}}
			record := finishSupersedeRecord()
			record.BranchPrepare = &model.IssueOpsBranchPrepare{BaseBranch: "parent"}
			req := finishSupersedeRequest("")
			req.Merged, req.MergedBaseBranch = true, "main"
			_, result := (app.FinishPreviewer{Environment: env}).Plan(context.Background(), record, req)
			for _, gate := range tc.wantMissing {
				if !slices.Contains(result.Missing, gate) {
					t.Fatalf("missing=%v, want %s", result.Missing, gate)
				}
			}
			if len(tc.wantMissing) == 0 && (!result.OK || (result.RetargetedBase != nil) != tc.wantRetarget) {
				t.Fatalf("result=%+v", result)
			}
			if tc.wantRetarget && (result.RetargetedBase == nil || result.RetargetedBase.DefaultBranch != "main") {
				t.Fatalf("retarget=%+v", result.RetargetedBase)
			}
		})
	}
}
