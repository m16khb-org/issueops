//go:build unix

package processlease_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	issueopsadapter "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/orca"
	"issueops/internal/adapter/outbound/processlease"
	"issueops/internal/adapter/provider/github"
	"issueops/internal/adapter/provider/gitlab"
	model "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

func TestRunnersInheritExecutionLifetime(t *testing.T) {
	for _, name := range []string{"git", "orca", "github", "gitlab"} {
		t.Run(name, func(t *testing.T) {
			repo, bin := t.TempDir(), t.TempDir()
			lease, err := processlease.Acquire(context.Background(), filepath.Join(t.TempDir(), "leases"), "cycle")
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			ctx := lease.Context(context.Background())
			executable := name
			body := "#!/bin/sh\nif [ ! -e /dev/fd/3 ]; then echo missing-lifetime-descriptor >&2; exit 41; fi\n"
			switch name {
			case "git":
				body += "[ \"$*\" = 'ls-remote --heads origin refs/heads/topic' ] || exit 42\n"
			case "orca":
				body += "printf '{}'\n"
			case "github":
				executable = "gh"
				body += "if [ \"$1 $2\" = 'issue view' ]; then printf '{\"body\":\"\"}'; exit 0; fi\n[ \"$1 $2\" = 'issue edit' ] || exit 42\nprintf updated > provider-effect\n"
			case "gitlab":
				executable = "glab"
				body += "case \"$*\" in *'--method PUT'*) printf updated > provider-effect; printf '{}';; *) printf '{\"description\":\"\"}';; esac\n"
			}
			if err := os.WriteFile(filepath.Join(bin, executable), []byte(body), 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin)
			switch name {
			case "git":
				if _, err := (issueopsadapter.LinkedBranchRemoteRef{}).Observe(ctx, repo, "topic"); err != nil {
					t.Fatal(err)
				}
			case "orca":
				if _, err := (orca.ExecRunner{}).Run(ctx, repo, time.Second*5, []string{"orca", "worktree", "remove", "wt"}); err != nil {
					t.Fatal(err)
				}
			default:
				var provider port.IssueProvider = github.NewProvider()
				url := "https://github.com/acme/repo/issues/12"
				if name == "gitlab" {
					provider = gitlab.NewProvider()
					url = "https://gitlab.example.com/acme/repo/-/issues/12"
				}
				result, err := provider.UpdateIssueBodySection(ctx, port.IssueProviderUpdateIssueBodySectionRequest{Repo: repo, IssueURL: url, Section: model.IssueBodySectionCompletion, Completion: &model.RemoteCompletionSection{ResultBody: "진행 결과"}, Confirm: true})
				if err != nil || !result.Updated {
					t.Fatalf("provider result=%+v err=%v", result, err)
				}

				if raw, err := os.ReadFile(filepath.Join(repo, "provider-effect")); err != nil || string(raw) != "updated" {
					t.Fatalf("provider effect missing: %q err=%v", raw, err)
				}
			}
			drained, err := lease.Drain(ctx)
			if err != nil {
				t.Fatal(err)
			}
			drained.Close()
		})
	}
}
