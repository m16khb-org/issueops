package issueopsapp

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	core "issueops/internal/adapter/issueops"
	"issueops/internal/adapter/outbound/sqlstore"
	model "issueops/internal/contract/issueops"
)

func TestGraphSyncCompositionPreservesPreviewAndProviderContracts(t *testing.T) {
	for _, mode := range []string{"preview", "missing-issue", "no-links", "github", "gitlab", "provider-error"} {
		t.Run(mode, func(t *testing.T) {
			root, bin := t.TempDir(), t.TempDir()
			bodyFile := filepath.Join(t.TempDir(), "body")
			code := "#!/bin/sh\nprintf '%s' \"$5\" > \"$GRAPH_BODY_FILE\"\nprintf '  https://example.test/comment/1  \\n'\n"
			if mode == "provider-error" {
				code = "#!/bin/sh\nprintf 'unavailable' >&2\nexit 2\n"
			}
			for _, tool := range []string{"gh", "glab"} {
				if err := os.WriteFile(filepath.Join(bin, tool), []byte(code), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("GRAPH_BODY_FILE", bodyFile)
			record, err := startIssueOpsFixture(root, model.IssueOpsStartRequest{Repo: t.TempDir(), Branch: "72-graph"})
			if err != nil {
				t.Fatal(err)
			}
			if mode != "preview" && mode != "missing-issue" {
				record.IssueURL = "https://github.com/acme/repo/issues/72"
				if mode == "gitlab" {
					record.IssueURL = "https://gitlab.example/group/repo/-/issues/72"
				}
			}
			if mode != "preview" && mode != "missing-issue" && mode != "no-links" {
				record.IssueLinks = []model.IssueOpsIssueLink{{Type: "depends-on", URL: "https://github.com/acme/repo/issues/73", Title: "Dependency", CreatedAt: "then"}}
			}
			if record, err = (core.CycleRecordStore{StateRoot: root}).Save(context.Background(), record); err != nil {
				t.Fatal(err)
			}
			db, err := sqlstore.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			before, exists, err := db.Get("issueops_v1", record.ID)
			if err != nil || !exists {
				t.Fatalf("initial row exists=%v err=%v", exists, err)
			}
			result, err := newIssueGraphSyncService(root).Sync(context.Background(), record.ID, mode != "preview")
			if mode == "missing-issue" || mode == "provider-error" {
				want := "no issue_url"
				if mode == "provider-error" {
					want = "gh issue comment failed: unavailable"
				}
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("err=%v want=%s", err, want)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if mode == "preview" {
					if result["dry_run"] != true || result["link_count"] != 0 {
						t.Fatalf("preview=%v", result)
					}
				}
				if mode == "no-links" {
					if !reflect.DeepEqual(result, map[string]any{"ok": true, "synced": false, "message": "no issue graph links to sync"}) {
						t.Fatalf("no links=%v", result)
					}
				}
				if mode == "github" || mode == "gitlab" {
					key := "comment_url"
					if mode == "gitlab" {
						key = "note_url"
					}
					if result["provider"] != mode || result[key] != "https://example.test/comment/1" || result["link_count"] != 1 {
						t.Fatalf("result=%v", result)
					}
					body, e := os.ReadFile(bodyFile)
					if e != nil {
						t.Fatal(e)
					}
					want := "## 관련 이슈\n\n- **선행 이슈**: https://github.com/acme/repo/issues/73 (Dependency)\n"
					if string(body) != want {
						t.Fatalf("body=%q want=%q", body, want)
					}
				}
			}
			if mode == "preview" || mode == "missing-issue" || mode == "no-links" {
				if _, e := os.Stat(bodyFile); !os.IsNotExist(e) {
					t.Fatalf("provider was invoked: %v", e)
				}
			}
			after, exists, err := db.Get("issueops_v1", record.ID)
			if err != nil || !exists || !bytes.Equal(before, after) {
				t.Fatalf("sync changed stored row: exists=%v err=%v", exists, err)
			}

		})
	}
}
