package issueops

import (
	model "issueops/internal/contract/issueops"
	lease "issueops/internal/contract/issueopslease"
	"os"
	"path/filepath"
	"testing"
)

func TestCompletionArtifactsRejectUnsealedFiles(t *testing.T) {
	for _, kind := range []string{"mode", "symlink", "oversize", "sealed"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			record := model.IssueOpsRecord{Repo: root}
			path := sealedArtifactPath(record, root, "plan")
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				t.Fatal(err)
			}
			if kind == "symlink" {
				target := filepath.Join(root, "source")
				if err := os.WriteFile(target, []byte("plan"), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			} else {
				mode := os.FileMode(0600)
				if kind == "mode" {
					mode = 0644
				}
				if err := os.WriteFile(path, []byte("plan"), mode); err != nil {
					t.Fatal(err)
				}
				if kind == "oversize" {
					if err := os.Truncate(path, lease.OwnerArtifactMaxBytes+1); err != nil {
						t.Fatal(err)
					}
				}
			}
			body, ok := (CompletionArtifacts{}).Read(record, root, "plan")
			if kind == "sealed" {
				if !ok || body != "plan" {
					t.Fatalf("sealed plan rejected: %q %t", body, ok)
				}
			} else if ok || body != "" {
				t.Fatalf("%s artifact published", kind)
			}
		})
	}
}
