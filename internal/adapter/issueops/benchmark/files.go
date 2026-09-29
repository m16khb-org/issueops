package benchmark

import (
	"encoding/json"
	"fmt"
	"io"
	issueopscontract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
	"os"
	"strings"
)

type Files struct{ Stdin io.Reader }

func (f Files) ReadSamples(path string) ([]issueopscontract.JudgeSample, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(path) == "" {
		raw, err = io.ReadAll(f.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, err
	}
	var payload struct {
		Samples []issueopscontract.JudgeSample `json:"samples"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse judge samples: %w", err)
	}
	return payload.Samples, nil
}

func (f Files) ReadOutcomes(path string) (issueopscontract.RecordedOutcomes, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(path) == "" {
		raw, err = io.ReadAll(f.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return issueopscontract.RecordedOutcomes{}, err
	}
	var rec issueopscontract.RecordedOutcomes
	if err := json.Unmarshal(raw, &rec); err != nil {
		return issueopscontract.RecordedOutcomes{}, fmt.Errorf("parse recorded outcomes: %w", err)
	}
	return rec, nil
}

func (f Files) ReadCandidate(path string) (issueopscontract.IssueOpsAutoresearchCandidate, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return issueopscontract.IssueOpsAutoresearchCandidate{}, fmt.Errorf("candidate-file is required")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return issueopscontract.IssueOpsAutoresearchCandidate{}, err
	}
	var candidate issueopscontract.IssueOpsAutoresearchCandidate
	if err := json.Unmarshal(b, &candidate); err != nil {
		return issueopscontract.IssueOpsAutoresearchCandidate{}, fmt.Errorf("parse candidate file %s: %w", path, err)
	}
	return candidate, nil
}

func (f Files) ReadJudgeMap(path string, fixtures []issueopscontract.IssueOpsBenchmarkFixture) (issueopscontract.IssueOpsJudgeMap, error) {
	var raw []byte
	var err error
	if strings.TrimSpace(path) == "" {
		raw, err = io.ReadAll(f.Stdin)
	} else {
		raw, err = os.ReadFile(path)
	}
	if err != nil {
		return issueopscontract.IssueOpsJudgeMap{}, err
	}
	var wrapper struct {
		SourceRunID string                     `json:"source_run_id"`
		Provenance  string                     `json:"provenance"`
		Scores      map[string]json.RawMessage `json:"scores"`
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&wrapper); err != nil {
		return issueopscontract.IssueOpsJudgeMap{}, fmt.Errorf("parse judge map (expect {\"source_run_id\":..,\"provenance\":..,\"scores\":{...}}): %w", err)
	}
	if wrapper.Scores == nil {
		return issueopscontract.IssueOpsJudgeMap{}, fmt.Errorf("judge map missing \"scores\" object")
	}
	payloads := make(map[string][]byte, len(wrapper.Scores))
	for id, raw := range wrapper.Scores {
		payloads[id] = raw
	}
	scores, err := domain.ResolveJudgeScores(fixtures, payloads, DecodeJudgeScore)
	if err != nil {
		return issueopscontract.IssueOpsJudgeMap{}, err
	}
	return issueopscontract.IssueOpsJudgeMap{SourceRunID: wrapper.SourceRunID, Provenance: wrapper.Provenance, Scores: scores}, nil
}
