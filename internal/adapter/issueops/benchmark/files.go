// Package benchmark 패키지는 issueops 벤치마크 CLI가 쓰는 파일·stdin 입력 읽기와 실행 결과 저장을 구현하는 adapter 계층이다.
// fixture, judge 점수, 기록된 결과, 후보 파일을 JSON으로 파싱해 contract 타입으로 돌려주며,
// 채점 규칙과 검증은 domain/issueopsbenchmark에 맡긴다.
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

// Files 타입은 벤치마크 입력 파일을 읽는 adapter다.
// 경로가 비어 있는 호출은 Stdin에서 읽는다.
type Files struct{ Stdin io.Reader }

// ReadSamples 메서드는 path(비어 있으면 Stdin)에서 {"samples": [...]} JSON을 읽어 judge 샘플 목록을 돌려준다.
// 읽기나 파싱에 실패하면 에러를 돌려준다.
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

// ReadOutcomes 메서드는 path(비어 있으면 Stdin)에서 기록된 benchmark outcome JSON을 읽어 돌려준다.
// 읽기나 파싱에 실패하면 에러를 돌려준다.
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

// ReadCandidate 메서드는 autoresearch 후보 JSON 파일을 읽는다.
// path는 필수이며 Stdin으로 대체하지 않는다. 비어 있거나 파싱에 실패하면 에러를 돌려준다.
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

// ReadJudgeMap 메서드는 path(비어 있으면 Stdin)에서 judge map JSON을 읽는다.
// 알 수 없는 필드와 누락된 "scores" 객체는 거부하고, 각 점수는 fixtures 기준으로 domain에서 검증·해석한다.
func (f Files) ReadJudgeMap(path string, fixtures []issueopscontract.Fixture) (issueopscontract.IssueOpsJudgeMap, error) {
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
	scores, err := domain.ResolveJudgeScores(fixtures, payloads, decodeJudgeScore)
	if err != nil {
		return issueopscontract.IssueOpsJudgeMap{}, err
	}
	return issueopscontract.IssueOpsJudgeMap{SourceRunID: wrapper.SourceRunID, Provenance: wrapper.Provenance, Scores: scores}, nil
}
