package quality

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCollectPioneerCoverageKeepsNamesakeDenominatorSeparate(t *testing.T) {
	root := t.TempDir()
	writeQualityTestFile(t, filepath.Join(root, "testdata", "issueops", "fixtures", "pioneer-verified-execution.json"), "{}")
	writeQualityTestFile(t, filepath.Join(root, "testdata", "issueops", "fixtures", "pioneer-issueops.json"), "{}")
	writeQualityTestFile(t, filepath.Join(root, "testdata", "pioneer-holdouts", "verified-execution", "TASK.md"), "task")

	coverage, err := CollectPioneerCoverage(root)

	if err != nil {
		t.Fatal(err)
	}
	if coverage.Expected != 12 || coverage.BenchmarkObserved != 1 || coverage.ReproductionObserved != 1 {
		t.Fatalf("coverage = %+v", coverage)
	}
	if slices.Contains(coverage.BenchmarkMissing, "verified-execution") || len(coverage.BenchmarkMissing) != 11 {
		t.Fatalf("benchmark missing = %v", coverage.BenchmarkMissing)
	}
}

func TestCollectPioneerCoverageRejectsSymlinkedFixtures(t *testing.T) {
	root := t.TempDir()
	external := filepath.Join(t.TempDir(), "fixture.json")
	writeQualityTestFile(t, external, "{}")
	fixture := filepath.Join(root, "testdata", "issueops", "fixtures", "pioneer-verified-execution.json")
	if err := os.MkdirAll(filepath.Dir(fixture), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, fixture); err != nil {
		t.Fatal(err)
	}

	coverage, err := CollectPioneerCoverage(root)

	if err != nil {
		t.Fatal(err)
	}
	if coverage.BenchmarkObserved != 0 || !slices.Contains(coverage.BenchmarkMissing, "verified-execution") {
		t.Fatalf("symlinked fixture counted as coverage: %+v", coverage)
	}
}

func TestCollectPioneerEvaluationManifestValidatesThreeAxesAndHashes(t *testing.T) {
	root := t.TempDir()
	skill := "sample-skill"
	files := map[string]string{
		"primary":     "TASK.md",
		"boundary":    "BOUNDARY.md",
		"operational": "OPERATIONAL.md",
	}
	cases := make([]map[string]any, 0, len(files))
	runs := make([]map[string]any, 0, len(files))
	evidencePath := filepath.ToSlash(filepath.Join("evidence-records", skill+".json"))
	evidenceBody := []byte(`{"schema_version":1,"skill":"sample-skill"}`)
	writeQualityTestFile(
		t,
		filepath.Join(root, "testdata", "pioneer-holdouts", evidencePath),
		string(evidenceBody),
	)
	evidenceDigest := sha256.Sum256(evidenceBody)
	evidenceSHA256 := fmt.Sprintf("%x", evidenceDigest[:])
	for axis, filename := range files {
		body := []byte("# " + axis + "\n")
		path := filepath.Join(root, "testdata", "pioneer-holdouts", skill, filename)
		writeQualityTestFile(t, path, string(body))
		digest := sha256.Sum256(body)
		taskID := "st-" + axis
		cases = append(cases, map[string]any{
			"skill":                    skill,
			"axis":                     axis,
			"case_path":                filepath.ToSlash(filepath.Join(skill, filename)),
			"case_sha256":              fmt.Sprintf("%x", digest[:]),
			"task_id":                  taskID,
			"verdict":                  "pass",
			"hidden_holdout":           false,
			"evidence_path":            evidencePath,
			"evidence_sha256":          evidenceSHA256,
			"deterministic_assertions": []string{"fixture-hash", "semantic-contract"},
			"semantic_grade":           "meets_case_contract",
			"host_capability":          "available",
		})
		runs = append(runs, map[string]any{
			"task_id": taskID, "axes": []string{axis}, "status": "completed",
			"host": "omo", "model": "test-model",
			"receipt_sha256":   fmt.Sprintf("%x", sha256.Sum256([]byte(taskID))),
			"receipt_bytes":    len(taskID),
			"execution_method": "fresh_context_child_task",
			"artifact_kind":    "bounded_final_response_receipt",
			"evidence_path":    evidencePath,
			"evidence_sha256":  evidenceSHA256,
		})
	}
	manifest, err := json.Marshal(map[string]any{
		"schema_version": 2,
		"provenance": map[string]any{
			"host": "omo", "execution_count": 3, "case_count": 3,
			"receipt_algorithm": "sha256", "receipt_source": "test receipt",
			"answers_committed": false, "hidden_holdouts": false,
			"evidence_record_count": 1, "evidence_record_algorithm": "sha256",
			"semantic_grading": "case-contract assertions",
		},
		"runs":  runs,
		"cases": cases,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeQualityTestFile(
		t,
		filepath.Join(root, "testdata", "pioneer-holdouts", "evaluation-manifest.json"),
		string(manifest),
	)

	counts, err := collectPioneerEvaluationManifest(root, []string{skill})
	if err != nil {
		t.Fatal(err)
	}
	if counts.observed != 3 || counts.passed != 3 || counts.blocked != 0 || counts.hidden != 0 {
		t.Fatalf("counts = %+v", counts)
	}

	cases[0]["case_sha256"] = strings.Repeat("0", 64)
	manifest, err = json.Marshal(map[string]any{
		"schema_version": 2,
		"provenance": map[string]any{
			"host": "omo", "execution_count": 3, "case_count": 3,
			"receipt_algorithm": "sha256", "receipt_source": "test receipt",
			"answers_committed": false, "hidden_holdouts": false,
			"evidence_record_count": 1, "evidence_record_algorithm": "sha256",
			"semantic_grading": "case-contract assertions",
		},
		"runs": runs, "cases": cases,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeQualityTestFile(
		t,
		filepath.Join(root, "testdata", "pioneer-holdouts", "evaluation-manifest.json"),
		string(manifest),
	)
	if _, err := collectPioneerEvaluationManifest(root, []string{skill}); err == nil {
		t.Fatal("hash mismatch must fail closed")
	}
}

func writeQualityTestFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}
