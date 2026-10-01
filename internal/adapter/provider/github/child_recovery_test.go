package github

import (
	"errors"
	"issueops/internal/port"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChildCreateNeverRetriesAmbiguousPreferredCreate(t *testing.T) {
	for _, output := range []string{"https://github.com/acme/repo/issues/34", ""} {
		t.Run(output, func(t *testing.T) {
			bin, repo := t.TempDir(), t.TempDir()
			writeFakeGh(t, bin, `#!/bin/sh
if [ "$3" = "--help" ]; then echo '  --parent string'; exit 0; fi
if [ "$1 $2" = "issue create" ]; then
 echo create >> calls
 echo '`+output+`'
 echo 'connection lost' >&2
 exit 1
fi
exit 2
`)
			t.Setenv("PATH", bin)
			got, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{Repo: repo, ParentIssueURL: "https://github.com/acme/repo/issues/12", Title: "child", Confirm: true})
			if err == nil {
				t.Fatal("expected ambiguous create error")
			}
			calls, _ := os.ReadFile(filepath.Join(repo, "calls"))
			if strings.Count(string(calls), "create") != 1 {
				t.Fatalf("duplicate create: %s", calls)
			}
			if got.ChildURL != output {
				t.Fatalf("lost URL: %+v", got)
			}
		})
	}
}

func TestChildCapabilityFailureNeverCreates(t *testing.T) {
	for _, help := range []string{"echo unreadable; exit 0", "echo capability-unavailable >&2; exit 1"} {
		t.Run(help, func(t *testing.T) {
			bin, repo := t.TempDir(), t.TempDir()
			writeFakeGh(t, bin, "#!/bin/sh\nif [ \"$3\" = \"--help\" ]; then "+help+"; fi\necho create >> calls\nexit 2\n")
			t.Setenv("PATH", bin)
			_, err := NewProvider().CreateChild(port.IssueProviderCreateChildRequest{Repo: repo, ParentIssueURL: "https://github.com/acme/repo/issues/12", Title: "child", Confirm: true})
			typed, ok := errors.AsType[*port.IssueProviderCreateError](err)
			if !ok || typed.Invoked {
				t.Fatalf("not-invoked proof missing: %v", err)
			}
			if _, err := os.Stat(filepath.Join(repo, "calls")); !os.IsNotExist(err) {
				t.Fatal("create invoked during failed capability check")
			}
		})
	}
}
