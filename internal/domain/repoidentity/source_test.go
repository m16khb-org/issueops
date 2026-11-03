package repoidentity

import (
	"testing"

	"issueops/internal/testsupport"
)

func TestSourceRootMapsGitCommonDirToPrimaryCheckout(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		commonDir string
		want      string
	}{
		{
			name: "empty path stays empty",
			path: "",
			want: "",
		},
		{
			name: "whitespace path stays empty",
			path: "   ",
			want: "",
		},
		{
			name: "empty common dir returns cleaned caller path",
			path: testsupport.AbsPath("tmp", "workspace", "repo"),
			want: testsupport.AbsPath("tmp", "workspace", "repo"),
		},
		{
			name:      "absolute git common dir maps to checkout dir",
			path:      testsupport.AbsPath("tmp", "worktree"),
			commonDir: testsupport.AbsPath("tmp", "primary", "repo", ".git"),
			want:      testsupport.AbsPath("tmp", "primary", "repo"),
		},
		{
			name:      "common dir without .git base keeps caller path",
			path:      testsupport.AbsPath("var", "folders", "worktree"),
			commonDir: testsupport.AbsPath("private", "var", "folders", "primary"),
			want:      testsupport.AbsPath("var", "folders", "worktree"),
		},
		{
			name:      "dot git suffix directory is not treated as git dir",
			path:      testsupport.AbsPath("tmp", "workspace", "repo"),
			commonDir: testsupport.AbsPath("tmp", "workspace", "repo", "my.git"),
			want:      testsupport.AbsPath("tmp", "workspace", "repo"),
		},
		{
			name:      "relative common dir joins caller path",
			path:      testsupport.AbsPath("var", "folders", "worktree"),
			commonDir: ".git",
			want:      testsupport.AbsPath("var", "folders", "worktree"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SourceRoot(tt.path, tt.commonDir)
			if got != tt.want {
				t.Fatalf("SourceRoot(%q, %q) = %q, want %q", tt.path, tt.commonDir, got, tt.want)
			}
		})
	}
}

func TestSourceRootCleansRelativePathsLexically(t *testing.T) {
	if got := SourceRoot("repo/../repo", ""); got != "repo" {
		t.Fatalf("got %q", got)
	}
}
