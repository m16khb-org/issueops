package stateio

import (
	"encoding/json"
	docsapp "issueops/internal/application/docs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"issueops/internal/adapter/augmentation"
	"issueops/internal/adapter/docs"
	"issueops/internal/adapter/install"
	statestore "issueops/internal/adapter/outbound/state"
	app "issueops/internal/application/selfaugment"
	augmentcontract "issueops/internal/contract/selfaugment"
)

func TestSaveSelfAugmentPlan(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISSUEOPS_STATE_DIR", dir)
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Clean(filepath.Join(cwd, "..", "..", "..", ".."))
	result := planForStateTest(augmentcontract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}, root, "test")
	if err := SaveSelfAugmentPlan(&result, "self-augment-plan-test"); err != nil {
		t.Fatalf("SaveSelfAugmentPlan: %v", err)
	}
	if result.StateCheckpoint == nil || !result.StateCheckpoint.OK {
		t.Fatalf("missing plan checkpoint: %+v", result.StateCheckpoint)
	}
	if result.StateCheckpoint.Key != "self-augment-plan-test" || result.StateCheckpoint.Path != filepath.Join(dir, "self-augment-plan-test.json") {
		t.Fatalf("unexpected plan checkpoint metadata: %+v", result.StateCheckpoint)
	}
	state, err := statestore.StateRead("self-augment-plan-test")
	if err != nil {
		t.Fatalf("StateRead: %v", err)
	}
	var snapshot augmentcontract.SelfAugmentPlanStateSnapshot
	if err := json.Unmarshal([]byte(state.Record.Content), &snapshot); err != nil {
		t.Fatalf("unmarshal saved plan snapshot: %v", err)
	}
	if snapshot.Kind != augmentcontract.SelfAugmentationPlanKind || snapshot.LoopKind != "self_augmentation" {
		t.Fatalf("unexpected saved plan snapshot: %+v", snapshot)
	}
	if snapshot.CandidateCount < 10 || len(snapshot.SatisfiedCandidateIDs) == 0 {
		t.Fatalf("saved plan did not preserve candidate memory: %+v", snapshot)
	}
}

func TestSaveSelfAugmentPlanRejectsInvalidStateKey(t *testing.T) {
	t.Setenv("ISSUEOPS_STATE_DIR", t.TempDir())
	result := planForStateTest(augmentcontract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 99}, ".", "test")
	if err := SaveSelfAugmentPlan(&result, "!bad-key"); err == nil {
		t.Fatal("expected self-augment plan save to reject invalid state key")
	}
	if result.StateCheckpoint == nil || result.StateCheckpoint.OK || result.StateCheckpoint.Error == "" {
		t.Fatalf("unexpected plan checkpoint after invalid save: %#v", result.StateCheckpoint)
	}
}

func planForStateTest(req augmentcontract.SelfAugmentPlanRequest, root, version string) augmentcontract.SelfAugmentPlanResult {
	return (app.Planner{Repository: augmentation.Repository{ListDocs: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).List}, DocsIndex: (docsapp.Service{Observer: docs.Observer{}, Now: time.Now}).Index, ListSkillNames: install.ListSkillNames, StateList: statestore.StateList, StateRead: statestore.StateRead, Now: time.Now}).Plan(req, root, version)
}
