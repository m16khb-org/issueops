package candidatescmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	augmentcontract "issueops/internal/contract/selfaugment"
	verifycontract "issueops/internal/contract/selfverify"
	"issueops/internal/testsupport"
)

func TestRunPrintsTextOutput(t *testing.T) {
	out := captureStdout(t, func() error {
		return Run([]string{}, Deps{
			Export: func() augmentcontract.SelfVerificationCandidateExportResult {
				return selfVerifyCandidatesCLIResultForTest()
			},
		})
	})

	if !strings.Contains(out, "자기 검증 루프 candidates: 2 candidate(s), selected=open-candidate") ||
		!strings.Contains(out, "- open-candidate coverage score=90.0 status=open") ||
		!strings.Contains(out, "- satisfied-candidate reliability score=0.0 status=already_satisfied") {
		t.Fatalf("unexpected candidates text output:\n%s", out)
	}
}

func TestRunSavesAndPrintsJSON(t *testing.T) {
	var savedKey string
	out := captureStdout(t, func() error {
		return Run([]string{"--save-state", "--state-key", "candidate-key", "--json"}, Deps{
			Export: func() augmentcontract.SelfVerificationCandidateExportResult {
				return selfVerifyCandidatesCLIResultForTest()
			},
			Save: func(result *augmentcontract.SelfVerificationCandidateExportResult, key string) error {
				savedKey = key
				result.StateCheckpoint = &augmentcontract.SelfAugmentStateCheckpoint{OK: true, Key: key}
				return nil
			},
			PrintJSON: printJSONForTest,
		})
	})
	if savedKey != "candidate-key" {
		t.Fatalf("expected save key candidate-key, got %q", savedKey)
	}
	var result augmentcontract.SelfVerificationCandidateExportResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode candidates JSON: %v\n%s", err, out)
	}
	if !result.OK || result.StateCheckpoint == nil || result.StateCheckpoint.Key != "candidate-key" {
		t.Fatalf("unexpected candidates JSON result: %+v", result)
	}
}

func TestRunPropagatesSaveError(t *testing.T) {
	saveErr := errors.New("save candidates failed")
	err := Run([]string{"--save-state"}, Deps{
		Export: func() augmentcontract.SelfVerificationCandidateExportResult {
			return selfVerifyCandidatesCLIResultForTest()
		},
		Save: func(*augmentcontract.SelfVerificationCandidateExportResult, string) error {
			return saveErr
		},
	})
	if !errors.Is(err, saveErr) {
		t.Fatalf("expected save error, got %v", err)
	}
}

func selfVerifyCandidatesCLIResultForTest() augmentcontract.SelfVerificationCandidateExportResult {
	selected := verifycontract.SelfVerificationCandidate{Priority: 1, ID: "open-candidate", Category: "coverage", Status: "open", Score: 90}
	satisfied := verifycontract.SelfVerificationCandidate{Priority: 2, ID: "satisfied-candidate", Category: "reliability", Status: "already_satisfied", Score: 0}
	return augmentcontract.SelfVerificationCandidateExportResult{
		OK:                    true,
		Kind:                  augmentcontract.SelfVerificationCandidateExportKind,
		LoopKind:              "self_verification",
		KoreanName:            augmentcontract.SelfVerificationKoreanName,
		CandidateCount:        2,
		OpenCandidateIDs:      []string{selected.ID},
		SatisfiedCandidateIDs: []string{satisfied.ID},
		SelectedCandidate:     &selected,
		Candidates:            []verifycontract.SelfVerificationCandidate{selected, satisfied},
	}
}

func captureStdout(t *testing.T, fn func() error) string {
	t.Helper()
	return testsupport.CaptureStdout(t, fn)
}

func printJSONForTest(value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}
