package issueopsapp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	qualityoutbound "issueops/internal/adapter/outbound/quality"
	qualitycontract "issueops/internal/contract/quality"
	qualitydomain "issueops/internal/domain/quality"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"issueops/cmd/issueops/mcpcli"
	"issueops/cmd/issueops/qualitycli"
	catalog "issueops/internal/adapter/inbound/catalog/mcp"
	"issueops/internal/adapter/outbound/sqlstore"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
)

func TestSelfPlanningInstancesKeepRootsAndLessonStateSeparate(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	dirs := []string{t.TempDir(), t.TempDir()}
	if err := os.WriteFile(filepath.Join(roots[0], "GENIUS_THINK.md"), []byte("# Local reasoning\n"), 0600); err != nil {
		t.Fatal(err)
	}
	sessions := make([]*mcp.ClientSession, 2)
	for i := range roots {
		sessions[i] = startHistoryMCPTestSession(t, mcpcli.MCPDependencies{Catalog: catalog.Build(), SelfPlanning: newSelfWorkflowPlanning(roots[i], dirs[i], "test"), SelfState: newSelfWorkflowState(dirs[i])})
	}
	var wg sync.WaitGroup
	for i := range roots {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for _, call := range []struct {
				name string
				args map[string]any
			}{
				{"self_augment", map[string]any{"save_state": true, "state_key": "plan"}},
				{"self_verify_candidates", map[string]any{"save_state": true, "state_key": "candidates"}},
				{"self_augment_lesson", map[string]any{"lesson": fmt.Sprintf("lesson-%d", i), "next_action": "verify next run", "state_key": "lesson"}},
			} {
				result, err := sessions[i].CallTool(context.Background(), &mcp.CallToolParams{Name: call.name, Arguments: call.args})
				if err != nil || result.IsError {
					t.Errorf("instance %d %s: %v %+v", i, call.name, err, result)
					return
				}
				var got struct {
					IssueOpsRoot    string `json:"issueops_root"`
					UsesGeniusThink bool   `json:"uses_genius_think"`
				}
				if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &got); err != nil {
					t.Error(err)
					return
				}
				if got.IssueOpsRoot != roots[i] {
					t.Errorf("root leaked: %s != %s", got.IssueOpsRoot, roots[i])
					return
				}
				if call.name == "self_augment" && got.UsesGeniusThink != (i == 0) {
					t.Errorf("source observation mixed between instances: %+v", got)
					return
				}
			}
		}(i)
	}
	wg.Wait()
	for i, dir := range dirs {
		for _, key := range []string{"plan", "candidates", "lesson"} {
			raw, exists, err := sqlstore.GetExisting(dir, "state", key)
			if err != nil || !exists {
				t.Fatalf("%d/%s: %v %v", i, key, exists, err)
			}
			var record statecontract.RecordEnvelope
			if err := json.Unmarshal(raw, &record); err != nil {
				t.Fatal(err)
			}
			var saved struct {
				IssueOpsRoot string `json:"issueops_root"`
				Lesson       string `json:"lesson"`
			}
			if err := json.Unmarshal([]byte(record.Content), &saved); err != nil {
				t.Fatal(err)
			}
			if saved.IssueOpsRoot != roots[i] || (key == "lesson" && saved.Lesson != fmt.Sprintf("lesson-%d", i)) {
				t.Fatalf("mixed durable record: %+v", saved)
			}
		}
	}
	absent := filepath.Join(t.TempDir(), "absent")
	if _, err := newSelfWorkflowPlanning(roots[0], absent, "test").SaveLesson(contract.SelfAugmentLessonRequest{CandidateID: "candidate", Lesson: "", NextAction: "next"}); err == nil {
		t.Fatal("invalid lesson accepted")
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatalf("rejected lesson created storage: %v", err)
	}
}

func TestQualityInstancesKeepBaselineStateSeparate(t *testing.T) {
	root := t.TempDir()
	dirs := []string{t.TempDir(), t.TempDir()}
	deps := []qualitycli.Deps{newQualityDependencies(root, dirs[0]), newQualityDependencies(root, dirs[1])}
	var wg sync.WaitGroup
	for i := range deps {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := deps[i].SaveSNRBaseline(root, float64(i+1)/4); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	for i := range deps {
		value, exists, err := deps[i].ReadSNRBaseline(root)
		if err != nil || !exists || value != float64(i+1)/4 {
			t.Fatalf("baseline %d leaked: %v %v %v", i, value, exists, err)
		}
	}
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	if value, exists, err := deps[0].ReadSNRBaseline(alias); err != nil || !exists || value != 0.25 {
		t.Fatalf("symlink identity changed: %v %v %v", value, exists, err)
	}
	absent := filepath.Join(t.TempDir(), "absent")
	invalid := newQualityDependencies(root, absent)
	if err := invalid.SaveSNRBaseline(root, math.NaN()); err == nil {
		t.Fatal("invalid ratio accepted")
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatalf("invalid ratio wrote state: %v", err)
	}
}

func TestQualityCLIInstancesCollectAndSaveTheirOwnRepository(t *testing.T) {
	roots := []string{t.TempDir(), t.TempDir()}
	dirs := []string{t.TempDir(), t.TempDir()}
	var wg sync.WaitGroup
	for i := range roots {
		files := map[string]string{"go.mod": "module example.test/local\n\ngo 1.26\n", "main.go": "package local\n// Return the fixture value.\nfunc Value()int{return 1}\n", "main_test.go": "package local\nimport \"testing\"\nfunc TestValue(t *testing.T){if Value()!=1{t.Fatal(\"value\")}}\n"}
		if i == 1 {
			files["main.go"] += "// Extra source comment.\n// Separate noise count.\n"
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(roots[i], name), []byte(content), 0600); err != nil {
				t.Fatal(err)
			}
		}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			deps := newQualityDependencies(roots[i], dirs[i])
			var result qualitycontract.InspectResult
			deps.PrintJSON = func(value any) error { result = value.(qualitycontract.InspectResult); return nil }
			if err := deps.SaveSNRBaseline(roots[i], 1); err != nil {
				t.Error(err)
				return
			}
			err := qualitycli.Run([]string{"inspect", "--json", "--trend", "--save-baseline"}, deps)
			if err != nil && !errors.Is(err, qualitycli.ErrQualityGateBlocked) {
				t.Error(err)
				return
			}
			regression := false
			for _, finding := range result.Findings {
				regression = regression || finding.ID == "code-snr-regression"
			}
			if !regression || !errors.Is(err, qualitycli.ErrQualityGateBlocked) {
				t.Errorf("saving the current baseline hid regression against the previous baseline: %v %+v", err, result.Findings)
				return
			}
			abs, _ := filepath.Abs(roots[i])
			if result.IssueOpsRoot != abs {
				t.Errorf("CLI used wrong root: %s", result.IssueOpsRoot)
				return
			}
			ratio, present, err := deps.ReadSNRBaseline(roots[i])
			if err != nil || !present {
				t.Errorf("baseline not saved: %v %v", present, err)
				return
			}
			want, err := qualityoutbound.ComputeCodeSNR(roots[i])
			if err != nil || ratio != want.Ratio {
				t.Errorf("wrong source ratio: %v %+v %v", ratio, want, err)
			}
		}(i)
	}
	wg.Wait()
}

func TestQualityBaselineRefusesUnknownSchemaWithoutMutation(t *testing.T) {
	root := t.TempDir()
	dir := t.TempDir()
	deps := newQualityDependencies(root, dir)
	if err := deps.SaveSNRBaseline(root, 0.8); err != nil {
		t.Fatal(err)
	}
	canonical, err := qualityoutbound.CanonicalRepository(root)
	if err != nil {
		t.Fatal(err)
	}
	key := qualitydomain.SNRBaselineKey(canonical)
	content, err := json.Marshal(qualitycontract.SNRBaselineRecord{SchemaVersion: 2, Repository: canonical, Ratio: 0.8})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newSelfWorkflowStateService(dir).Write(key, string(content)); err != nil {
		t.Fatal(err)
	}
	before, _, err := sqlstore.GetExisting(dir, "state", key)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists, err := deps.ReadSNRBaseline(root); exists || err == nil {
		t.Fatalf("future schema accepted: %v %v", exists, err)
	}
	after, _, err := sqlstore.GetExisting(dir, "state", key)
	if err != nil || string(before) != string(after) {
		t.Fatalf("read refusal modified state: %v", err)
	}
	absent := filepath.Join(t.TempDir(), "absent")
	missing := newQualityDependencies(root, absent)
	if _, exists, err := missing.ReadSNRBaseline(root); exists || err != nil {
		t.Fatalf("missing baseline: %v %v", exists, err)
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatalf("baseline read created storage: %v", err)
	}
}
