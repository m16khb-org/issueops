package benchmark

import (
	"encoding/json"
	"fmt"
	contract "issueops/internal/contract/issueopsbenchmark"
	"os"
	"path/filepath"
	"strings"
)

// Store 타입은 Directory 아래 issueops-benchmarks/<id>.json에 벤치마크 실행 결과를 보관하는 파일 저장소다.
type Store struct{ Directory string }

// Read 메서드는 id로 저장된 벤치마크 실행 결과를 읽는다.
// id가 비어 있거나 파일이 없거나 파싱에 실패하면 에러를 돌려준다.
func (s Store) Read(id string) (contract.RunResult, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return contract.RunResult{}, fmt.Errorf("benchmark id is required")
	}
	b, err := os.ReadFile(filepath.Join(s.Directory, "issueops-benchmarks", id+".json"))
	if err != nil {
		return contract.RunResult{}, err
	}
	var result contract.RunResult
	if err := json.Unmarshal(b, &result); err != nil {
		return contract.RunResult{}, err
	}
	return result, nil
}

// Save 메서드는 result를 result.ID 이름의 들여쓰기된 JSON 파일로 저장한다.
// 저장 디렉터리가 없으면 만들고, 같은 ID의 기존 파일은 덮어쓴다.
func (s Store) Save(result contract.RunResult) error {
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
