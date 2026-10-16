package issueopsapp

import (
	"os"
	"path/filepath"
	"testing"

	agentmodelcontract "issueops/internal/contract/agentmodel"
)

func TestMemoizedReviewModelLoadsSettingsOncePerRepository(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	settingsPath := filepath.Join(configHome, "issueops", "agent-models.json")
	if err := os.MkdirAll(filepath.Dir(settingsPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(settingsPath, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	repoA, repoB := t.TempDir(), t.TempDir()
	resolve := memoizedReviewModel()

	if _, _, err := resolve("claude", agentmodelcontract.RoleDiffReview, "", repoA); err == nil {
		t.Fatal("malformed settings must fail the resolution")
	}
	if err := os.Remove(settingsPath); err != nil {
		t.Fatal(err)
	}
	if _, _, err := resolve("claude", agentmodelcontract.RoleDiffReview, "", repoA); err == nil {
		t.Fatal("settings for a repository must be loaded once per next call, so the first load result is reused")
	}
	model, _, err := resolve("claude", agentmodelcontract.RoleDiffReview, "", repoB)
	if err != nil || model == "" {
		t.Fatalf("a different repository must load its own settings: model=%q err=%v", model, err)
	}
}
