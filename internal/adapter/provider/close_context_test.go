//go:build unix

package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"issueops/internal/adapter/outbound/processlease"
	"issueops/internal/adapter/provider/github"
	"issueops/internal/adapter/provider/gitlab"
	"issueops/internal/port"
)

func closeProviderCases() map[string]func(context.Context, string) error {
	return map[string]func(context.Context, string) error{
		"github-issue": func(ctx context.Context, repo string) error {
			_, err := github.NewProvider().CloseIssue(ctx, port.IssueProviderCloseIssueRequest{Repo: repo, IssueURL: "https://github.com/acme/repo/issues/12", Confirm: true})
			return err
		},
		"github-pr": func(ctx context.Context, repo string) error {
			_, err := github.NewProvider().ClosePullRequest(ctx, port.IssueProviderClosePullRequestRequest{Repo: repo, ArtifactURL: "https://github.com/acme/repo/pull/12", Kind: "pr", Confirm: true})
			return err
		},
		"gitlab-issue": func(ctx context.Context, repo string) error {
			_, err := gitlab.NewProvider().CloseIssue(ctx, port.IssueProviderCloseIssueRequest{Repo: repo, IssueURL: "https://gitlab.example.com/acme/repo/-/issues/12", Confirm: true})
			return err
		},
		"gitlab-mr": func(ctx context.Context, repo string) error {
			_, err := gitlab.NewProvider().ClosePullRequest(ctx, port.IssueProviderClosePullRequestRequest{Repo: repo, ArtifactURL: "https://gitlab.example.com/acme/repo/-/merge_requests/12", Kind: "mr", Confirm: true})
			return err
		},
	}
}

func TestCloseProviderCanceledContextStartsNoCommand(t *testing.T) {
	for name, call := range closeProviderCases() {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			marker := filepath.Join(dir, "started")
			t.Setenv("CLOSE_STARTED", marker)
			for _, binary := range []string{"gh", "glab"} {
				if err := os.WriteFile(filepath.Join(dir, binary), []byte("#!/bin/sh\n: > \"$CLOSE_STARTED\"\nprintf '%s' '{\"state\":\"closed\"}'\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := call(ctx, dir); err == nil || !strings.Contains(err.Error(), "context canceled") {
				t.Errorf("canceled close error=%v", err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("canceled request started a command: %v", err)
			}
		})
	}
}

func TestCloseProviderCommandsInheritCleanupLifetime(t *testing.T) {
	for name, call := range closeProviderCases() {
		for stage := 1; stage <= 3; stage++ {
			t.Run(fmt.Sprintf("%s/command%d", name, stage), func(t *testing.T) {
				dir := t.TempDir()
				pidfile := filepath.Join(dir, "child.pid")
				t.Setenv("CLOSE_PID", pidfile)
				t.Setenv("CLOSE_COUNT", filepath.Join(dir, "count"))
				t.Setenv("CLOSE_CHILD_STAGE", strconv.Itoa(stage))
				script := `#!/bin/sh
n=0
if [ -f "$CLOSE_COUNT" ]; then IFS= read -r n < "$CLOSE_COUNT"; fi
n=$((n+1))
printf '%s\n' "$n" > "$CLOSE_COUNT"
if [ "$n" = "$CLOSE_CHILD_STAGE" ]; then
 sleep 30 </dev/null >/dev/null 2>&1 &
 printf '%s' "$!" > "$CLOSE_PID"
fi
if [ "$n" = 1 ]; then printf '%s' '{"state":"opened"}'; else printf '%s' '{"state":"closed"}'; fi
`
				for _, binary := range []string{"gh", "glab"} {
					if err := os.WriteFile(filepath.Join(dir, binary), []byte(script), 0700); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				stopped := false
				stop := func() {
					if stopped {
						return
					}
					data, err := os.ReadFile(pidfile)
					if err != nil {
						return
					}
					pid, err := strconv.Atoi(string(data))
					if err != nil {
						return
					}
					process, err := os.FindProcess(pid)
					if err == nil {
						_ = process.Kill()
						stopped = true
					}
				}
				t.Cleanup(stop)
				lease, err := processlease.Acquire(context.Background(), filepath.Join(dir, "locks"), "cleanup")
				if err != nil {
					t.Fatal(err)
				}
				defer lease.Close()
				if err := call(lease.Context(context.Background()), dir); err != nil {
					t.Fatal(err)
				}
				next, err := lease.Drain(context.Background())
				if next != nil {
					_ = next.Close()
				}
				if !errors.Is(err, processlease.ErrBusy) {
					t.Fatalf("provider command did not preserve child lifetime: %v", err)
				}
				stop()
				deadline := time.Now().Add(3 * time.Second)
				for {
					next, err := processlease.Acquire(context.Background(), filepath.Join(dir, "locks"), "cleanup")
					if err == nil {
						_ = next.Close()
						break
					}
					if time.Now().After(deadline) {
						t.Fatalf("child lifetime did not drain: %v", err)
					}
					time.Sleep(10 * time.Millisecond)
				}
			})
		}
	}
}

func TestCloseProviderCancellationStopsEachCommandStage(t *testing.T) {
	for name, call := range closeProviderCases() {
		for stage := 1; stage <= 3; stage++ {
			t.Run(fmt.Sprintf("%s/command%d", name, stage), func(t *testing.T) {
				dir := t.TempDir()
				marker, counter := filepath.Join(dir, "started"), filepath.Join(dir, "count")
				t.Setenv("CLOSE_STARTED", marker)
				t.Setenv("CLOSE_COUNT", counter)
				t.Setenv("CLOSE_BLOCK_STAGE", strconv.Itoa(stage))
				script := `#!/bin/sh
n=0
if [ -f "$CLOSE_COUNT" ]; then IFS= read -r n < "$CLOSE_COUNT"; fi
n=$((n+1))
printf '%s\n' "$n" > "$CLOSE_COUNT"
if [ "$n" = "$CLOSE_BLOCK_STAGE" ]; then
 : > "$CLOSE_STARTED"
 exec sleep 30
fi
if [ "$n" = 1 ]; then printf '%s' '{"state":"opened"}'; else printf '%s' '{"state":"closed"}'; fi
`
				for _, binary := range []string{"gh", "glab"} {
					if err := os.WriteFile(filepath.Join(dir, binary), []byte(script), 0700); err != nil {
						t.Fatal(err)
					}
				}
				t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				result := make(chan error, 1)
				go func() { result <- call(ctx, dir) }()
				deadline := time.Now().Add(3 * time.Second)
				for {
					if _, err := os.Stat(marker); err == nil {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("provider did not reach requested command stage")
					}
					time.Sleep(5 * time.Millisecond)
				}
				cancel()
				select {
				case err := <-result:
					if err == nil || !strings.Contains(err.Error(), "context canceled") {
						t.Fatalf("canceled provider error=%v", err)
					}
				case <-time.After(3 * time.Second):
					t.Fatal("provider command ignored cancellation")
				}
				data, err := os.ReadFile(counter)
				if err != nil || strings.TrimSpace(string(data)) != strconv.Itoa(stage) {
					t.Fatalf("command continued after cancellation: %q %v", data, err)
				}
			})
		}
	}
}
