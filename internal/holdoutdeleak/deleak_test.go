// Package holdoutdeleak mechanically guards the A6 reproduction-harness
// invariant: the committed pioneer holdout fixtures under
// testdata/pioneer-holdouts/ contain INPUTS ONLY, never the recorded answer
// (scores, root cause, fix, provenance), and the raw answer tree under
// .issueops/evidence/ stays untracked. See testdata/pioneer-holdouts/README.md.
package holdoutdeleak

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// forbiddenAnswerTokens are field/markers that live ONLY in the gitignored
// result.yaml answer artifacts. None may appear in any committed fixture file —
// a reproduction harness reproduces the symptom, not the diagnosis or its score.
var forbiddenAnswerTokens = []string{
	"case_score",
	"request_fit",
	"method_fidelity",
	"evidence_and_verification",
	"safety_and_portability",
	"evidence_strength",
	"keep_discard_decision",
	"sub_agent_id",
	"calibration_cases_rerun",
	"gate_flags",
	"anti-gaming",
}

// repoRoot resolves the repository root from this test file's own location,
// independent of the test's working directory (internal/holdoutdeleak -> root
// is two directories up).
func repoRoot(t *testing.T) string {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot resolve test file path via runtime.Caller")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(self), "..", ".."))
}

func TestHoldoutFixturesContainInputsOnly(t *testing.T) {
	root := repoRoot(t)
	fixtureDir := filepath.Join(root, "testdata", "pioneer-holdouts")
	if info, err := os.Stat(fixtureDir); err != nil || !info.IsDir() {
		t.Fatalf("fixture dir %s missing: %v", fixtureDir, err)
	}

	var files int
	walkErr := filepath.WalkDir(fixtureDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		files++
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lower := strings.ToLower(string(b))
		for _, tok := range forbiddenAnswerTokens {
			if strings.Contains(lower, strings.ToLower(tok)) {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("fixture %s leaks answer token %q — reproduction harness must hold inputs only, never the recorded score/diagnosis", rel, tok)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walk fixtures: %v", walkErr)
	}
	if files == 0 {
		t.Fatal("no fixture files found under testdata/pioneer-holdouts")
	}
}

func TestEvidenceAnswerTreeStaysUntracked(t *testing.T) {
	// The raw result.yaml answers live under .issueops/evidence. Ask git whether
	// the repository's own ignore rules cover that tree, so the guard follows the
	// rule's effect rather than one spelling of the .gitignore line.
	root := repoRoot(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	answer := ".issueops/evidence/pioneer-skills-quality/reruns/sample/result.yaml"
	cmd := exec.Command(git, "check-ignore", "-q", "--no-index", "--", answer)
	cmd.Dir = root
	// Only the repository's rules count: a developer's global excludesFile
	// must not make an unignored tree look ignored.
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1")
	err = cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return
	case errors.As(err, &exitErr) && exitErr.ExitCode() == 1:
		t.Errorf(".gitignore no longer ignores %s — the holdout answer tree may now be tracked; A6 keeps answers OUT of git", answer)
	default:
		t.Skipf("git check-ignore unavailable (not a work tree?): %v", err)
	}
}

func TestEvidenceAnswerFilesNotForceTracked(t *testing.T) {
	// Stronger than the .gitignore-line check: a `git add -f result.yaml` would
	// bypass the ignore and silently track an answer. Assert the answer tree is
	// genuinely untracked. Skips cleanly when git is unavailable or this is not a
	// git work tree (keeps the unit test environment-tolerant).
	root := repoRoot(t)
	git, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not available")
	}
	cmd := exec.Command(git, "ls-files", "--", ".issueops/evidence")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("git ls-files unavailable (not a work tree?): %v", err)
	}
	if tracked := strings.TrimSpace(string(out)); tracked != "" {
		t.Errorf("answer tree .issueops/evidence is git-tracked (force-added?); holdout answers (result.yaml/baselines) must stay untracked:\n%s", tracked)
	}
}
