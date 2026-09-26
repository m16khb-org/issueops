package selfverify

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestValidateFormatUsesTrackedFilesAndFailsOnUnformatted(t *testing.T) {
	called := 0
	step := ValidateFormat("/repo", FormatDeps{
		Now: time.Now,
		ListTrackedGoFiles: func(_ context.Context, root string) ([]string, error) {
			if root != "/repo" {
				t.Fatalf("root=%q", root)
			}
			called++
			return []string{"a.go", "b.go"}, nil
		},
		ListUnformatted: func(_ context.Context, root string, files []string) ([]string, error) {
			if root != "/repo" || len(files) != 2 {
				t.Fatalf("root=%q files=%v", root, files)
			}
			called++
			return []string{"b.go"}, nil
		},
	})
	if step.OK || step.Label != FormatLabel || step.Command != FormatCommand || !strings.Contains(step.Error, "b.go") || called != 2 {
		t.Fatalf("step=%+v calls=%d", step, called)
	}
}

func TestValidateFormatFailsClosedWithoutFilesOrTooling(t *testing.T) {
	for _, tc := range []struct {
		name  string
		files []string
		err   error
		want  string
	}{
		{name: "no tracked files", files: []string{}, want: "no tracked .go files found"},
		{name: "git failed", err: errors.New("git failed"), want: "list tracked .go files: git failed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			step := ValidateFormat("/repo", FormatDeps{
				Now:                time.Now,
				ListTrackedGoFiles: func(context.Context, string) ([]string, error) { return tc.files, tc.err },
				ListUnformatted: func(context.Context, string, []string) ([]string, error) {
					t.Fatal("gofmt should not run")
					return nil, nil
				},
			})
			if step.OK || step.Command != FormatCommand || !strings.Contains(step.Error, tc.want) {
				t.Fatalf("step=%+v", step)
			}
		})
	}
}
