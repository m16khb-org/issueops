package quality

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const pioneerFixtureSkill = "sample-skill"

type pioneerFixture struct {
	root     string
	manifest pioneerEvaluationManifest
}

func pioneerDigest(body []byte) string {
	sum := sha256.Sum256(body)
	return fmt.Sprintf("%x", sum[:])
}

// newPioneerFixture writes a consistent single-skill fixture: one run covering
// the three axes, three passing cases, and the shared evidence record.
func newPioneerFixture(t *testing.T) *pioneerFixture {
	t.Helper()
	root := t.TempDir()
	holdouts := filepath.Join(root, "testdata", "pioneer-holdouts")
	evidenceBody := []byte(`{"schema_version":1}`)
	evidencePath := "evidence-records/" + pioneerFixtureSkill + ".json"
	writeQualityTestFile(t, filepath.Join(holdouts, evidencePath), string(evidenceBody))
	evidenceDigest := pioneerDigest(evidenceBody)
	manifest := pioneerEvaluationManifest{
		SchemaVersion: 2,
		Provenance: pioneerManifestProvenance{
			Host: "omo", ExecutionCount: 1, CaseCount: 3,
			ReceiptAlgorithm: "sha256", ReceiptSource: "receipt",
			EvidenceRecordCount: 1, EvidenceRecordAlgorithm: "sha256",
			SemanticGrading: "contract",
		},
		Runs: []pioneerManifestRun{{
			TaskID: "run-1", Axes: []string{"primary", "boundary", "operational"},
			Status: "completed", Host: "omo", Model: "m",
			ReceiptSHA256: pioneerDigest([]byte("receipt")), ReceiptBytes: 7,
			ExecutionMethod: "fresh_context_child_task",
			ArtifactKind:    "bounded_final_response_receipt",
			EvidencePath:    evidencePath, EvidenceSHA256: evidenceDigest,
		}},
	}
	for _, axis := range []string{"primary", "boundary", "operational"} {
		filename := pioneerAxisFilenames[axis]
		body := []byte("# " + axis + "\n")
		writeQualityTestFile(t, filepath.Join(holdouts, pioneerFixtureSkill, filename), string(body))
		manifest.Cases = append(manifest.Cases, pioneerManifestCase{
			Skill: pioneerFixtureSkill, Axis: axis,
			CasePath: pioneerFixtureSkill + "/" + filename, CaseSHA256: pioneerDigest(body),
			TaskID: "run-1", Verdict: "pass",
			EvidencePath: evidencePath, EvidenceSHA256: evidenceDigest,
			DeterministicAssertions: []string{"a", "b"},
			SemanticGrade:           "ok", HostCapability: "available",
		})
	}
	return &pioneerFixture{root: root, manifest: manifest}
}

func (f *pioneerFixture) write(t *testing.T) {
	t.Helper()
	data, err := json.Marshal(f.manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeQualityTestFile(t, filepath.Join(f.root, "testdata", "pioneer-holdouts", "evaluation-manifest.json"), string(data))
}

func (f *pioneerFixture) collect(t *testing.T) (pioneerEvaluationCounts, error) {
	t.Helper()
	f.write(t)
	return collectPioneerEvaluationManifest(f.root, []string{pioneerFixtureSkill})
}

func TestPioneerManifestCountsVerdictsHiddenAndBlockedReasons(t *testing.T) {
	f := newPioneerFixture(t)
	f.manifest.Cases[1].Verdict = "blocked"
	f.manifest.Cases[1].BlockedReason = "host cannot spawn"
	f.manifest.Cases[2].Verdict = "fail"
	f.manifest.Cases[2].HiddenHoldout = true

	counts, err := f.collect(t)
	if err != nil {
		t.Fatal(err)
	}
	if counts.observed != 3 || counts.passed != 1 || counts.blocked != 1 || counts.failed != 1 || counts.hidden != 1 || counts.executions != 1 {
		t.Fatalf("counts = %+v", counts)
	}
	if len(counts.blockedCases) != 1 {
		t.Fatalf("blocked cases = %+v", counts.blockedCases)
	}
	got := counts.blockedCases[0]
	if got.Skill != pioneerFixtureSkill || got.Axis != "boundary" || got.Reason != "host cannot spawn" {
		t.Fatalf("blocked case = %+v", got)
	}
}

func TestPioneerManifestRejectsInvalidManifests(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, f *pioneerFixture)
	}{
		{"unsupported schema", func(_ *testing.T, f *pioneerFixture) { f.manifest.SchemaVersion = 1 }},
		{"blank host", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.Host = " " }},
		{"execution count mismatch", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.ExecutionCount = 2 }},
		{"case count mismatch", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.CaseCount = 2 }},
		{"non sha256 receipt algorithm", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.ReceiptAlgorithm = "md5" }},
		{"blank receipt source", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.ReceiptSource = "" }},
		{"answers committed", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.AnswersCommitted = true }},
		{"hidden holdouts declared", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.HiddenHoldouts = true }},
		{"evidence record count mismatch", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.EvidenceRecordCount = 2 }},
		{"non sha256 evidence algorithm", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.EvidenceRecordAlgorithm = "sha1" }},
		{"blank semantic grading", func(_ *testing.T, f *pioneerFixture) { f.manifest.Provenance.SemanticGrading = "" }},
		{"blank run task id", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].TaskID = " " }},
		{"incomplete run", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].Status = "running" }},
		{"blank run model", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].Model = "" }},
		{"receipt digest not hex", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].ReceiptSHA256 = strings.Repeat("z", 64) }},
		{"receipt digest wrong length", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].ReceiptSHA256 = "abcd" }},
		{"empty receipt", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].ReceiptBytes = 0 }},
		{"oversized receipt", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].ReceiptBytes = 1<<20 + 1 }},
		{"unexpected execution method", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].ExecutionMethod = "inline" }},
		{"unexpected artifact kind", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].ArtifactKind = "full" }},
		{"run evidence outside records dir", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].EvidencePath = "other/x.json" }},
		{"run evidence digest invalid", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].EvidenceSHA256 = "nope" }},
		{"unknown run axis", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].Axes = []string{"primary", "extra"} }},
		{"duplicate run axis", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].Axes = []string{"primary", "primary"} }},
		{"run without axes", func(_ *testing.T, f *pioneerFixture) { f.manifest.Runs[0].Axes = nil }},
		{"duplicate run task id", func(_ *testing.T, f *pioneerFixture) {
			f.manifest.Runs = append(f.manifest.Runs, f.manifest.Runs[0])
			f.manifest.Provenance.ExecutionCount = 2
		}},
		{"case for unexpected skill", func(_ *testing.T, f *pioneerFixture) { f.manifest.Cases[0].Skill = "other-skill" }},
		{"case with unknown axis", func(_ *testing.T, f *pioneerFixture) { f.manifest.Cases[0].Axis = "weird" }},
		{"case path not canonical for axis", func(_ *testing.T, f *pioneerFixture) {
			f.manifest.Cases[0].CasePath = pioneerFixtureSkill + "/BOUNDARY.md"
		}},
		{"duplicate skill axis case", func(_ *testing.T, f *pioneerFixture) {
			f.manifest.Cases[1] = f.manifest.Cases[0]
		}},
		{"blank case task id", func(_ *testing.T, f *pioneerFixture) { f.manifest.Cases[0].TaskID = "" }},
		{"case references unknown run", func(_ *testing.T, f *pioneerFixture) { f.manifest.Cases[0].TaskID = "ghost" }},
		{"case axis not declared by run", func(_ *testing.T, f *pioneerFixture) {
			f.manifest.Runs[0].Axes = []string{"primary", "boundary"}
		}},
		{"case evidence path not canonical", func(_ *testing.T, f *pioneerFixture) {
			f.manifest.Cases[0].EvidencePath = "evidence-records/other.json"
		}},
		{"case evidence digest differs from run", func(_ *testing.T, f *pioneerFixture) {
			f.manifest.Cases[0].EvidenceSHA256 = strings.Repeat("0", 64)
		}},
		{"too few deterministic assertions", func(_ *testing.T, f *pioneerFixture) {
			f.manifest.Cases[0].DeterministicAssertions = []string{"only"}
		}},
		{"blank semantic grade", func(_ *testing.T, f *pioneerFixture) { f.manifest.Cases[0].SemanticGrade = " " }},
		{"blank host capability", func(_ *testing.T, f *pioneerFixture) { f.manifest.Cases[0].HostCapability = "" }},
		{"blocked without reason", func(_ *testing.T, f *pioneerFixture) {
			f.manifest.Cases[0].Verdict = "blocked"
			f.manifest.Cases[0].BlockedReason = "  "
		}},
		{"unknown verdict", func(_ *testing.T, f *pioneerFixture) { f.manifest.Cases[0].Verdict = "maybe" }},
		{"case file missing", func(t *testing.T, f *pioneerFixture) {
			if err := os.Remove(filepath.Join(f.root, "testdata", "pioneer-holdouts", pioneerFixtureSkill, "TASK.md")); err != nil {
				t.Fatal(err)
			}
		}},
		{"case file content changed", func(t *testing.T, f *pioneerFixture) {
			writeQualityTestFile(t, filepath.Join(f.root, "testdata", "pioneer-holdouts", pioneerFixtureSkill, "TASK.md"), "tampered")
		}},
		{"evidence record missing", func(t *testing.T, f *pioneerFixture) {
			if err := os.Remove(filepath.Join(f.root, "testdata", "pioneer-holdouts", "evidence-records", pioneerFixtureSkill+".json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"evidence record is a symlink", func(t *testing.T, f *pioneerFixture) {
			path := filepath.Join(f.root, "testdata", "pioneer-holdouts", "evidence-records", pioneerFixtureSkill+".json")
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			external := filepath.Join(t.TempDir(), "evidence.json")
			writeQualityTestFile(t, external, string(content))
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(external, path); err != nil {
				t.Fatal(err)
			}
		}},
		{"evidence record content changed", func(t *testing.T, f *pioneerFixture) {
			writeQualityTestFile(t, filepath.Join(f.root, "testdata", "pioneer-holdouts", "evidence-records", pioneerFixtureSkill+".json"), `{"tampered":true}`)
		}},
		{"run axes not all referenced by cases", func(_ *testing.T, f *pioneerFixture) {
			f.manifest.Cases = f.manifest.Cases[:2]
			f.manifest.Provenance.CaseCount = 2
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			f := newPioneerFixture(t)
			if _, err := f.collect(t); err != nil {
				t.Fatalf("baseline fixture must be valid: %v", err)
			}
			test.mutate(t, f)
			if _, err := f.collect(t); err == nil {
				t.Fatal("invalid manifest was accepted")
			}
		})
	}
}

func TestPioneerManifestDecodeFailureIsReported(t *testing.T) {
	f := newPioneerFixture(t)
	writeQualityTestFile(t, filepath.Join(f.root, "testdata", "pioneer-holdouts", "evaluation-manifest.json"), "{not json")
	_, err := collectPioneerEvaluationManifest(f.root, []string{pioneerFixtureSkill})
	if err == nil || !strings.Contains(err.Error(), "decode pioneer evaluation manifest") {
		t.Fatalf("err = %v", err)
	}
}

func TestPioneerManifestMissingFileYieldsEmptyCountsButUnreadableFails(t *testing.T) {
	root := t.TempDir()
	counts, err := collectPioneerEvaluationManifest(root, []string{pioneerFixtureSkill})
	if err != nil || counts.observed != 0 || counts.executions != 0 {
		t.Fatalf("missing manifest: counts=%+v err=%v", counts, err)
	}
	if err := os.MkdirAll(filepath.Join(root, "testdata", "pioneer-holdouts", "evaluation-manifest.json"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := collectPioneerEvaluationManifest(root, []string{pioneerFixtureSkill}); err == nil {
		t.Fatal("unreadable manifest must be an error, not an empty result")
	}
}

func TestCollectPioneerCoveragePropagatesStatErrors(t *testing.T) {
	root := t.TempDir()
	// A regular file where a directory is expected makes Lstat fail with
	// something other than "not exist".
	writeQualityTestFile(t, filepath.Join(root, "testdata"), "not a directory")
	if _, err := CollectPioneerCoverage(root); err == nil {
		t.Fatal("stat failure was swallowed as missing coverage")
	}
}

func TestRegularFileExistsDistinguishesFileDirectoryAndMissing(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f")
	writeQualityTestFile(t, file, "x")
	cases := map[string]bool{file: true, dir: false, filepath.Join(dir, "absent"): false}
	for path, want := range cases {
		got, err := regularFileExists(path)
		if err != nil || got != want {
			t.Errorf("regularFileExists(%s) = %v, %v; want %v", path, got, err, want)
		}
	}
	if _, err := regularFileExists(filepath.Join(file, "child")); err == nil {
		t.Error("ENOTDIR must surface as an error")
	}
}
