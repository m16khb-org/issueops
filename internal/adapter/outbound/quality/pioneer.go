package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	contract "issueops/internal/contract/quality"
	"issueops/internal/domain/pioneerskill"
)

type PioneerCoverage = contract.PioneerCoverage
type PioneerBlockedCase = contract.PioneerBlockedCase

func CollectPioneerCoverage(root string) (PioneerCoverage, error) {
	names := pioneerskill.Names()
	benchmarkObserved := make([]string, 0, len(names))
	reproductionObserved := make([]string, 0, len(names))
	for _, name := range names {
		benchmarkPath := filepath.Join(root, "testdata", "issueops", "fixtures", "pioneer-"+name+".json")
		exists, err := regularFileExists(benchmarkPath)
		if err != nil {
			return PioneerCoverage{}, err
		}
		if exists {
			benchmarkObserved = append(benchmarkObserved, name)
		}
		reproductionPath := filepath.Join(root, "testdata", "pioneer-holdouts", name, "TASK.md")
		exists, err = regularFileExists(reproductionPath)
		if err != nil {
			return PioneerCoverage{}, err
		}
		if exists {
			reproductionObserved = append(reproductionObserved, name)
		}
	}
	isolated, err := collectPioneerEvaluationManifest(root, names)
	if err != nil {
		return PioneerCoverage{}, err
	}
	return PioneerCoverage{
		Expected:               len(names),
		BenchmarkObserved:      len(benchmarkObserved),
		BenchmarkMissing:       pioneerskill.Missing(benchmarkObserved),
		ReproductionObserved:   len(reproductionObserved),
		ReproductionMissing:    pioneerskill.Missing(reproductionObserved),
		IsolatedExpected:       len(names) * 3,
		IsolatedObserved:       isolated.observed,
		IsolatedPassed:         isolated.passed,
		IsolatedBlocked:        isolated.blocked,
		IsolatedFailed:         isolated.failed,
		IsolatedExecutionCount: isolated.executions,
		IsolatedBlockedCases:   append([]PioneerBlockedCase(nil), isolated.blockedCases...),
		HiddenHoldoutObserved:  isolated.hidden,
	}, nil
}

type pioneerEvaluationCounts struct {
	observed     int
	passed       int
	blocked      int
	failed       int
	hidden       int
	executions   int
	blockedCases []PioneerBlockedCase
}

func collectPioneerEvaluationManifest(root string, names []string) (pioneerEvaluationCounts, error) {
	path := filepath.Join(root, "testdata", "pioneer-holdouts", "evaluation-manifest.json")
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return pioneerEvaluationCounts{}, nil
	}
	if err != nil {
		return pioneerEvaluationCounts{}, err
	}
	var manifest struct {
		SchemaVersion int `json:"schema_version"`
		Provenance    struct {
			Host                    string `json:"host"`
			ExecutionCount          int    `json:"execution_count"`
			CaseCount               int    `json:"case_count"`
			ReceiptAlgorithm        string `json:"receipt_algorithm"`
			ReceiptSource           string `json:"receipt_source"`
			AnswersCommitted        bool   `json:"answers_committed"`
			HiddenHoldouts          bool   `json:"hidden_holdouts"`
			EvidenceRecordCount     int    `json:"evidence_record_count"`
			EvidenceRecordAlgorithm string `json:"evidence_record_algorithm"`
			SemanticGrading         string `json:"semantic_grading"`
		} `json:"provenance"`
		Runs []struct {
			TaskID          string   `json:"task_id"`
			Axes            []string `json:"axes"`
			Status          string   `json:"status"`
			Host            string   `json:"host"`
			Model           string   `json:"model"`
			ReceiptSHA256   string   `json:"receipt_sha256"`
			ReceiptBytes    int      `json:"receipt_bytes"`
			ExecutionMethod string   `json:"execution_method"`
			ArtifactKind    string   `json:"artifact_kind"`
			EvidencePath    string   `json:"evidence_path"`
			EvidenceSHA256  string   `json:"evidence_sha256"`
		} `json:"runs"`
		Cases []struct {
			Skill                   string   `json:"skill"`
			Axis                    string   `json:"axis"`
			CasePath                string   `json:"case_path"`
			CaseSHA256              string   `json:"case_sha256"`
			TaskID                  string   `json:"task_id"`
			Verdict                 string   `json:"verdict"`
			BlockedReason           string   `json:"blocked_reason"`
			HiddenHoldout           bool     `json:"hidden_holdout"`
			EvidencePath            string   `json:"evidence_path"`
			EvidenceSHA256          string   `json:"evidence_sha256"`
			DeterministicAssertions []string `json:"deterministic_assertions"`
			SemanticGrade           string   `json:"semantic_grade"`
			HostCapability          string   `json:"host_capability"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return pioneerEvaluationCounts{}, fmt.Errorf("decode pioneer evaluation manifest: %w", err)
	}
	if manifest.SchemaVersion != 2 {
		return pioneerEvaluationCounts{}, fmt.Errorf("unsupported pioneer evaluation manifest schema %d", manifest.SchemaVersion)
	}
	if strings.TrimSpace(manifest.Provenance.Host) == "" ||
		manifest.Provenance.ExecutionCount != len(manifest.Runs) ||
		manifest.Provenance.CaseCount != len(manifest.Cases) ||
		manifest.Provenance.ReceiptAlgorithm != "sha256" ||
		strings.TrimSpace(manifest.Provenance.ReceiptSource) == "" ||
		manifest.Provenance.AnswersCommitted ||
		manifest.Provenance.HiddenHoldouts ||
		manifest.Provenance.EvidenceRecordCount != len(names) ||
		manifest.Provenance.EvidenceRecordAlgorithm != "sha256" ||
		strings.TrimSpace(manifest.Provenance.SemanticGrading) == "" {
		return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation provenance")
	}
	runAxes := make(map[string]map[string]bool, len(manifest.Runs))
	type evidenceReference struct{ path, digest string }
	runEvidence := make(map[string]evidenceReference, len(manifest.Runs))
	for _, run := range manifest.Runs {
		taskID := strings.TrimSpace(run.TaskID)
		digest, digestErr := hex.DecodeString(run.ReceiptSHA256)
		evidenceDigest, evidenceDigestErr := hex.DecodeString(run.EvidenceSHA256)
		if taskID == "" ||
			runAxes[taskID] != nil ||
			run.Status != "completed" ||
			strings.TrimSpace(run.Host) == "" ||
			strings.TrimSpace(run.Model) == "" ||
			digestErr != nil ||
			len(digest) != sha256.Size ||
			run.ReceiptBytes <= 0 ||
			run.ReceiptBytes > 1<<20 ||
			run.ExecutionMethod != "fresh_context_child_task" ||
			run.ArtifactKind != "bounded_final_response_receipt" ||
			!strings.HasPrefix(run.EvidencePath, "evidence-records/") ||
			evidenceDigestErr != nil ||
			len(evidenceDigest) != sha256.Size {
			return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation run %q", taskID)
		}
		axes := make(map[string]bool, len(run.Axes))
		for _, axis := range run.Axes {
			if axis != "primary" && axis != "boundary" && axis != "operational" {
				return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation run axis %q", axis)
			}
			if axes[axis] {
				return pioneerEvaluationCounts{}, fmt.Errorf("duplicate pioneer evaluation run axis %q", axis)
			}
			axes[axis] = true
		}
		if len(axes) == 0 {
			return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evaluation run %q has no axes", taskID)
		}
		runAxes[taskID] = axes
		runEvidence[taskID] = evidenceReference{path: run.EvidencePath, digest: run.EvidenceSHA256}
	}
	expected := make(map[string]bool, len(names))
	for _, name := range names {
		expected[name] = true
	}
	seen := make(map[string]bool, len(manifest.Cases))
	referencedRunAxes := make(map[string]map[string]bool, len(manifest.Runs))
	var counts pioneerEvaluationCounts
	for _, item := range manifest.Cases {
		expectedFilename := map[string]string{
			"primary":     "TASK.md",
			"boundary":    "BOUNDARY.md",
			"operational": "OPERATIONAL.md",
		}[item.Axis]
		expectedPath := filepath.ToSlash(filepath.Join(item.Skill, expectedFilename))
		expectedEvidencePath := filepath.ToSlash(filepath.Join("evidence-records", item.Skill+".json"))
		key := item.Skill + "/" + item.Axis
		if !expected[item.Skill] ||
			expectedFilename == "" ||
			item.CasePath != expectedPath ||
			seen[key] ||
			strings.TrimSpace(item.TaskID) == "" ||
			!runAxes[item.TaskID][item.Axis] ||
			item.EvidencePath != expectedEvidencePath ||
			runEvidence[item.TaskID] != (evidenceReference{path: item.EvidencePath, digest: item.EvidenceSHA256}) ||
			len(item.DeterministicAssertions) < 2 ||
			strings.TrimSpace(item.SemanticGrade) == "" ||
			strings.TrimSpace(item.HostCapability) == "" {
			return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation case %q", key)
		}
		seen[key] = true
		if referencedRunAxes[item.TaskID] == nil {
			referencedRunAxes[item.TaskID] = map[string]bool{}
		}
		referencedRunAxes[item.TaskID][item.Axis] = true
		task, err := os.ReadFile(filepath.Join(root, "testdata", "pioneer-holdouts", item.CasePath))
		if err != nil {
			return pioneerEvaluationCounts{}, err
		}
		digest := sha256.Sum256(task)
		if fmt.Sprintf("%x", digest[:]) != item.CaseSHA256 {
			return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evaluation case hash mismatch for %s", key)
		}
		evidencePath := filepath.Join(root, "testdata", "pioneer-holdouts", item.EvidencePath)
		info, err := os.Lstat(evidencePath)
		if err != nil || !info.Mode().IsRegular() {
			return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evidence record for %s", key)
		}
		evidenceRaw, err := os.ReadFile(evidencePath)
		if err != nil {
			return pioneerEvaluationCounts{}, err
		}
		evidenceDigest := sha256.Sum256(evidenceRaw)
		if fmt.Sprintf("%x", evidenceDigest[:]) != item.EvidenceSHA256 {
			return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evidence record hash mismatch for %s", key)
		}
		counts.observed++
		if item.HiddenHoldout {
			counts.hidden++
		}
		switch item.Verdict {
		case "pass":
			counts.passed++
		case "blocked":
			if strings.TrimSpace(item.BlockedReason) == "" {
				return pioneerEvaluationCounts{}, fmt.Errorf("blocked pioneer evaluation %s requires blocked_reason", item.Skill)
			}
			counts.blocked++
			counts.blockedCases = append(counts.blockedCases, PioneerBlockedCase{
				Skill: item.Skill, Axis: item.Axis, Reason: item.BlockedReason,
			})
		case "fail":
			counts.failed++
		default:
			return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation verdict %q", item.Verdict)
		}
	}
	for taskID, axes := range runAxes {
		if len(referencedRunAxes[taskID]) != len(axes) {
			return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evaluation run %q axis receipt mismatch", taskID)
		}
		for axis := range axes {
			if !referencedRunAxes[taskID][axis] {
				return pioneerEvaluationCounts{}, fmt.Errorf("pioneer evaluation run %q missing case axis %q", taskID, axis)
			}
		}
	}
	counts.executions = len(manifest.Runs)
	return counts, nil
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err == nil {
		return info.Mode().IsRegular(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
