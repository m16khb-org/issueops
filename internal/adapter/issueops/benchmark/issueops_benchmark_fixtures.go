package benchmark

import (
	"encoding/json"
	"fmt"
	issueopscontract "issueops/internal/contract/issueopsbenchmark"
	domain "issueops/internal/domain/issueopsbenchmark"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LoadFixtures 메서드는 dir 바로 아래의 *.json 파일을 fixture로 읽어 검증한 뒤 돌려준다.
// 하위 디렉터리는 탐색하지 않으며, dir이 비어 있거나 파싱·검증에 실패한 파일이 있으면 에러를 돌려준다.
func (Files) LoadFixtures(dir string) ([]issueopscontract.Fixture, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("fixtures path is required")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var fixtures []issueopscontract.Fixture
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var fixture issueopscontract.Fixture
		if err := json.Unmarshal(b, &fixture); err != nil {
			return nil, fmt.Errorf("parse fixture %s: %w", path, err)
		}
		if err := domain.ValidateFixture(fixture); err != nil {
			return nil, fmt.Errorf("invalid fixture %s: %w", path, err)
		}
		fixtures = append(fixtures, fixture)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	if len(fixtures) == 0 {
		return nil, fmt.Errorf("no issueops benchmark fixtures in %s", dir)
	}
	return fixtures, nil
}
