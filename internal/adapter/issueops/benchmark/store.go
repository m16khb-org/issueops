package benchmark

import (
	"encoding/json"
	"fmt"
	contract "issueops/internal/contract/issueopsbenchmark"
	"os"
	"path/filepath"
	"strings"
)

type Store struct{ Directory string }

func (s Store) Read(id string) (contract.IssueOpsBenchmarkRunResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return contract.IssueOpsBenchmarkRunResult{}, fmt.Errorf("benchmark id is required")
	}
	b, err := os.ReadFile(filepath.Join(s.Directory, "issueops-benchmarks", id+".json"))
	if err != nil {
		return contract.IssueOpsBenchmarkRunResult{}, err
	}
	var result contract.IssueOpsBenchmarkRunResult
	if err := json.Unmarshal(b, &result); err != nil {
		return contract.IssueOpsBenchmarkRunResult{}, err
	}
	return result, nil
}

func (s Store) Save(result contract.IssueOpsBenchmarkRunResult) error {
	dir := filepath.Join(s.Directory, "issueops-benchmarks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, result.ID+".json"), b, 0o644)
}
