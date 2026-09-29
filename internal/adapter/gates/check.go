// Package gates owns ledger filesystem observation and persistence.
package gates

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gatesdomain "issueops/internal/domain/gates"
)

type FileStore struct{}

func (FileStore) Discover(cwd string) ([]string, error) { return DiscoverGateFiles(cwd) }
func (FileStore) Read(file string) ([]byte, error)      { return os.ReadFile(file) }
func (FileStore) ExistsFile(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}
func (FileStore) Create(file string, data []byte) error {
	if dir := filepath.Dir(file); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(file, data, 0644)
}
func (FileStore) WritePreservingMode(file string, data []byte) error {
	mode := os.FileMode(0644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(file, data, mode)
}

type Clock struct{}

func (Clock) Now() time.Time { return time.Now() }

// IssueFolderDir은 이슈 번호별 산출물 폴더의 상대 경로다(#480). 게이트
// 원장은 그 안의 gates.md 하나다.
const IssueFolderDir = ".issueops/issues"

// DiscoverGateFiles는 canonical .issueops/issues/<n>/gates.md를 먼저
// 찾고(번호 오름차순, 그다음 비숫자 폴더), 기존 root GATES.md,
// .issueops/gates/*.md, gates/*.md도 읽기 호환 경로로 반환한다.
func DiscoverGateFiles(root string) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	files := appendIssueFolderGateFiles([]string{}, filepath.Join(root, filepath.FromSlash(IssueFolderDir)))
	if info, err := os.Stat(filepath.Join(root, "GATES.md")); err == nil && !info.IsDir() {
		files = append(files, filepath.Join(root, "GATES.md"))
	}
	files = appendMarkdownGateFiles(files, filepath.Join(root, ".issueops", "gates"))
	files = appendMarkdownGateFiles(files, filepath.Join(root, "gates"))
	return files, nil
}

// appendIssueFolderGateFiles는 issues/<name>/gates.md만 후보로 넣는다. 같은
// 폴더의 plan.md/spec.md는 원장이 아니다.
func appendIssueFolderGateFiles(files []string, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, entry.Name(), "gates.md")); err == nil && !info.IsDir() {
			names = append(names, entry.Name())
		}
	}
	gatesdomain.SortIssueFolders(names)
	for _, name := range names {
		files = append(files, filepath.Join(dir, name, "gates.md"))
	}
	return files
}

func appendMarkdownGateFiles(files []string, dir string) []string {
	entries, err := os.ReadDir(dir)
	if err == nil {
		names := []string{}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				names = append(names, entry.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			files = append(files, filepath.Join(dir, name))
		}
	}
	return files
}
