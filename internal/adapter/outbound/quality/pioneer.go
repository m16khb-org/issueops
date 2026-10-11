package quality

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	contract "issueops/internal/contract/quality"
	"issueops/internal/domain/pioneerskill"
)

func CollectPioneerCoverage(root string) (contract.PioneerCoverage, error) {
	names := pioneerskill.Names()
	benchmarkObserved := make([]string, 0, len(names))
	reproductionObserved := make([]string, 0, len(names))
	for _, name := range names {
		benchmarkPath := filepath.Join(root, "testdata", "issueops", "fixtures", "pioneer-"+name+".json")
		exists, err := regularFileExists(benchmarkPath)
		if err != nil {
			return contract.PioneerCoverage{}, err
		}
		if exists {
			benchmarkObserved = append(benchmarkObserved, name)
		}
		reproductionPath := filepath.Join(root, "testdata", "pioneer-holdouts", name, "TASK.md")
		exists, err = regularFileExists(reproductionPath)
		if err != nil {
			return contract.PioneerCoverage{}, err
		}
		if exists {
			reproductionObserved = append(reproductionObserved, name)
		}
	}
	isolated, err := collectPioneerEvaluationManifest(root, names)
	if err != nil {
		return contract.PioneerCoverage{}, err
	}
	return contract.PioneerCoverage{
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
		IsolatedBlockedCases:   append([]contract.PioneerBlockedCase(nil), isolated.blockedCases...),
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
	blockedCases []contract.PioneerBlockedCase
}

type pioneerManifestProvenance struct {
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
}

type pioneerManifestRun struct {
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
}

type pioneerManifestCase struct {
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
}

type pioneerEvaluationManifest struct {
	SchemaVersion int                       `json:"schema_version"`
	Provenance    pioneerManifestProvenance `json:"provenance"`
	Runs          []pioneerManifestRun      `json:"runs"`
	Cases         []pioneerManifestCase     `json:"cases"`
}

type pioneerEvidenceReference struct{ path, digest string }

// pioneerRunIndex records, per validated run task ID, its declared axes and evidence reference.
type pioneerRunIndex struct {
	axes     map[string]map[string]bool
	evidence map[string]pioneerEvidenceReference
}

var pioneerAxisFilenames = map[string]string{
	"primary":     "TASK.md",
	"boundary":    "BOUNDARY.md",
	"operational": "OPERATIONAL.md",
}

func collectPioneerEvaluationManifest(root string, names []string) (pioneerEvaluationCounts, error) {
	path := filepath.Join(root, "testdata", "pioneer-holdouts", "evaluation-manifest.json")
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return pioneerEvaluationCounts{}, nil
	}
	if err != nil {
		return pioneerEvaluationCounts{}, err
	}
	var manifest pioneerEvaluationManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return pioneerEvaluationCounts{}, fmt.Errorf("decode pioneer evaluation manifest: %w", err)
	}
	if manifest.SchemaVersion != 2 {
		return pioneerEvaluationCounts{}, fmt.Errorf("unsupported pioneer evaluation manifest schema %d", manifest.SchemaVersion)
	}
	if !pioneerProvenanceValid(manifest.Provenance, len(manifest.Runs), len(manifest.Cases), len(names)) {
		return pioneerEvaluationCounts{}, fmt.Errorf("invalid pioneer evaluation provenance")
	}
	runs, err := indexPioneerRuns(manifest.Runs)
	if err != nil {
		return pioneerEvaluationCounts{}, err
	}
	counts, referencedRunAxes, err := tallyPioneerCases(root, names, manifest.Cases, runs)
	if err != nil {
		return pioneerEvaluationCounts{}, err
	}
	if err := checkPioneerRunAxesReferenced(runs.axes, referencedRunAxes); err != nil {
		return pioneerEvaluationCounts{}, err
	}
	counts.executions = len(manifest.Runs)
	return counts, nil
}

func pioneerProvenanceValid(p pioneerManifestProvenance, runCount, caseCount, nameCount int) bool {
	return strings.TrimSpace(p.Host) != "" &&
		p.ExecutionCount == runCount &&
		p.CaseCount == caseCount &&
		p.ReceiptAlgorithm == "sha256" &&
		strings.TrimSpace(p.ReceiptSource) != "" &&
		!p.AnswersCommitted &&
		!p.HiddenHoldouts &&
		p.EvidenceRecordCount == nameCount &&
		p.EvidenceRecordAlgorithm == "sha256" &&
		strings.TrimSpace(p.SemanticGrading) != ""
}

func indexPioneerRuns(runs []pioneerManifestRun) (pioneerRunIndex, error) {
	index := pioneerRunIndex{
		axes:     make(map[string]map[string]bool, len(runs)),
		evidence: make(map[string]pioneerEvidenceReference, len(runs)),
	}
	for _, run := range runs {
		taskID := strings.TrimSpace(run.TaskID)
		if taskID == "" || index.axes[taskID] != nil || !pioneerRunValid(run) {
			return pioneerRunIndex{}, fmt.Errorf("invalid pioneer evaluation run %q", taskID)
		}
		axes, err := pioneerRunAxes(taskID, run.Axes)
		if err != nil {
			return pioneerRunIndex{}, err
		}
		index.axes[taskID] = axes
		index.evidence[taskID] = pioneerEvidenceReference{path: run.EvidencePath, digest: run.EvidenceSHA256}
	}
	return index, nil
}

// pioneerRunValid checks every run field except the task ID and its uniqueness.
func pioneerRunValid(run pioneerManifestRun) bool {
	return run.Status == "completed" &&
		strings.TrimSpace(run.Host) != "" &&
		strings.TrimSpace(run.Model) != "" &&
		isSHA256Hex(run.ReceiptSHA256) &&
		run.ReceiptBytes > 0 &&
		run.ReceiptBytes <= 1<<20 &&
		run.ExecutionMethod == "fresh_context_child_task" &&
		run.ArtifactKind == "bounded_final_response_receipt" &&
		strings.HasPrefix(run.EvidencePath, "evidence-records/") &&
		isSHA256Hex(run.EvidenceSHA256)
}

func isSHA256Hex(value string) bool {
	digest, err := hex.DecodeString(value)
	return err == nil && len(digest) == sha256.Size
}

func pioneerRunAxes(taskID string, declared []string) (map[string]bool, error) {
	axes := make(map[string]bool, len(declared))
	for _, axis := range declared {
		if axis != "primary" && axis != "boundary" && axis != "operational" {
			return nil, fmt.Errorf("invalid pioneer evaluation run axis %q", axis)
		}
		if axes[axis] {
			return nil, fmt.Errorf("duplicate pioneer evaluation run axis %q", axis)
		}
		axes[axis] = true
	}
	if len(axes) == 0 {
		return nil, fmt.Errorf("pioneer evaluation run %q has no axes", taskID)
	}
	return axes, nil
}

func tallyPioneerCases(root string, names []string, cases []pioneerManifestCase, runs pioneerRunIndex) (pioneerEvaluationCounts, map[string]map[string]bool, error) {
	expected := make(map[string]bool, len(names))
	for _, name := range names {
		expected[name] = true
	}
	seen := make(map[string]bool, len(cases))
	referencedRunAxes := make(map[string]map[string]bool, len(runs.axes))
	var counts pioneerEvaluationCounts
	for _, item := range cases {
		key := item.Skill + "/" + item.Axis
		if !pioneerCaseValid(item, expected, seen[key], runs) {
			return pioneerEvaluationCounts{}, nil, fmt.Errorf("invalid pioneer evaluation case %q", key)
		}
		seen[key] = true
		if referencedRunAxes[item.TaskID] == nil {
			referencedRunAxes[item.TaskID] = map[string]bool{}
		}
		referencedRunAxes[item.TaskID][item.Axis] = true
		if err := verifyPioneerCaseFiles(root, item, key); err != nil {
			return pioneerEvaluationCounts{}, nil, err
		}
		if err := counts.record(item); err != nil {
			return pioneerEvaluationCounts{}, nil, err
		}
	}
	return counts, referencedRunAxes, nil
}

func pioneerCaseValid(item pioneerManifestCase, expected map[string]bool, alreadySeen bool, runs pioneerRunIndex) bool {
	expectedFilename := pioneerAxisFilenames[item.Axis]
	expectedPath := filepath.ToSlash(filepath.Join(item.Skill, expectedFilename))
	expectedEvidencePath := filepath.ToSlash(filepath.Join("evidence-records", item.Skill+".json"))
	return expected[item.Skill] &&
		expectedFilename != "" &&
		item.CasePath == expectedPath &&
		!alreadySeen &&
		strings.TrimSpace(item.TaskID) != "" &&
		runs.axes[item.TaskID][item.Axis] &&
		item.EvidencePath == expectedEvidencePath &&
		runs.evidence[item.TaskID] == pioneerEvidenceReference{path: item.EvidencePath, digest: item.EvidenceSHA256} &&
		len(item.DeterministicAssertions) >= 2 &&
		strings.TrimSpace(item.SemanticGrade) != "" &&
		strings.TrimSpace(item.HostCapability) != ""
}

// verifyPioneerCaseFiles checks the case file hash and the evidence record's type and hash.
func verifyPioneerCaseFiles(root string, item pioneerManifestCase, key string) error {
	task, err := os.ReadFile(filepath.Join(root, "testdata", "pioneer-holdouts", item.CasePath))
	if err != nil {
		return err
	}
	digest := sha256.Sum256(task)
	if fmt.Sprintf("%x", digest[:]) != item.CaseSHA256 {
		return fmt.Errorf("pioneer evaluation case hash mismatch for %s", key)
	}
	evidencePath := filepath.Join(root, "testdata", "pioneer-holdouts", item.EvidencePath)
	info, err := os.Lstat(evidencePath)
	if err != nil || !info.Mode().IsRegular() {
		return fmt.Errorf("invalid pioneer evidence record for %s", key)
	}
	evidenceRaw, err := os.ReadFile(evidencePath)
	if err != nil {
		return err
	}
	evidenceDigest := sha256.Sum256(evidenceRaw)
	if fmt.Sprintf("%x", evidenceDigest[:]) != item.EvidenceSHA256 {
		return fmt.Errorf("pioneer evidence record hash mismatch for %s", key)
	}
	return nil
}

func (c *pioneerEvaluationCounts) record(item pioneerManifestCase) error {
	c.observed++
	if item.HiddenHoldout {
		c.hidden++
	}
	switch item.Verdict {
	case "pass":
		c.passed++
	case "blocked":
		if strings.TrimSpace(item.BlockedReason) == "" {
			return fmt.Errorf("blocked pioneer evaluation %s requires blocked_reason", item.Skill)
		}
		c.blocked++
		c.blockedCases = append(c.blockedCases, contract.PioneerBlockedCase{
			Skill: item.Skill, Axis: item.Axis, Reason: item.BlockedReason,
		})
	case "fail":
		c.failed++
	default:
		return fmt.Errorf("invalid pioneer evaluation verdict %q", item.Verdict)
	}
	return nil
}

func checkPioneerRunAxesReferenced(runAxes, referencedRunAxes map[string]map[string]bool) error {
	for taskID, axes := range runAxes {
		if len(referencedRunAxes[taskID]) != len(axes) {
			return fmt.Errorf("pioneer evaluation run %q axis receipt mismatch", taskID)
		}
		for axis := range axes {
			if !referencedRunAxes[taskID][axis] {
				return fmt.Errorf("pioneer evaluation run %q missing case axis %q", taskID, axis)
			}
		}
	}
	return nil
}

func regularFileExists(path string) (bool, error) {
	info, err := os.Lstat(path)
	if err == nil {
		return info.Mode().IsRegular(), nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return false, err
}
