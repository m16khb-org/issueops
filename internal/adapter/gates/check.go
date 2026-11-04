// Package gates owns ledger filesystem observation and persistence.
package gates

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	gatesdomain "issueops/internal/domain/gates"
)

// FileStore reads and writes gate ledgers directly on the local filesystem
// without workspace confinement; WorkspaceFileStore is the confined variant.
type FileStore struct{}

// Discover lists the gate ledger files under cwd; see DiscoverGateFiles for
// ordering.
func (FileStore) Discover(cwd string) ([]string, error) { return DiscoverGateFiles(cwd) }
func (FileStore) Read(file string) ([]byte, error)      { return os.ReadFile(file) }

// ExistsFile reports whether file exists and is not a directory.
func (FileStore) ExistsFile(file string) bool {
	info, err := os.Stat(file)
	return err == nil && !info.IsDir()
}

// Create writes data to file with mode 0644, creating missing parent
// directories (0755) first.
func (FileStore) Create(file string, data []byte) error {
	if dir := filepath.Dir(file); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(file, data, 0644)
}

// WritePreservingMode overwrites file with data and keeps its existing
// permission bits, defaulting to 0644 when the file does not exist yet.
func (FileStore) WritePreservingMode(file string, data []byte) error {
	mode := os.FileMode(0644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	return os.WriteFile(file, data, mode)
}

// Clock reports the wall-clock time for ledger timestamps.
type Clock struct{}

// Now returns the current local time.
func (Clock) Now() time.Time { return time.Now() }

// IssueFolderDir은 이슈 번호별 산출물 폴더의 상대 경로다(#480). 게이트
// 원장은 그 안의 gates.md 하나다.
const issueFolderDir = ".issueops/issues"

// DiscoverGateFiles 함수는 canonical .issueops/issues/<n>/gates.md를 먼저
// 찾고(번호 오름차순, 그다음 비숫자 폴더), 이슈가 없는 scope 원장인
// .issueops/gates/*.md를 그 뒤에 반환한다.
func DiscoverGateFiles(root string) ([]string, error) {
	if strings.TrimSpace(root) == "" {
		return nil, nil
	}
	return discoverGateFilesIn(os.DirFS(root), ".", root), nil
}

// discoverGateFilesIn walks dir inside fsys and reports each ledger as prefix
// joined with its dir-relative path.
func discoverGateFilesIn(fsys fs.FS, dir, prefix string) []string {
	scan := gateFileScan{fsys: fsys, dir: dir, prefix: prefix}
	files := scan.appendIssueFolderGateFiles([]string{}, issueFolderDir)
	return scan.appendMarkdownGateFiles(files, ".issueops/gates")
}

type gateFileScan struct {
	fsys        fs.FS
	dir, prefix string
}

func (s gateFileScan) output(name string) string {
	return filepath.Join(s.prefix, filepath.FromSlash(name))
}

// appendIssueFolderGateFiles는 issues/<name>/gates.md만 후보로 넣는다. 같은
// 폴더의 plan.md/spec.md는 원장이 아니다.
func (s gateFileScan) appendIssueFolderGateFiles(files []string, dir string) []string {
	entries, err := fs.ReadDir(s.fsys, path.Join(s.dir, dir))
	if err != nil {
		return files
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if info, err := fs.Stat(s.fsys, path.Join(s.dir, dir, entry.Name(), "gates.md")); err == nil && !info.IsDir() {
			names = append(names, entry.Name())
		}
	}
	gatesdomain.SortIssueFolders(names)
	for _, name := range names {
		files = append(files, s.output(path.Join(dir, name, "gates.md")))
	}
	return files
}

func (s gateFileScan) appendMarkdownGateFiles(files []string, dir string) []string {
	entries, err := fs.ReadDir(s.fsys, path.Join(s.dir, dir))
	if err == nil {
		names := []string{}
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
				names = append(names, entry.Name())
			}
		}
		sort.Strings(names)
		for _, name := range names {
			files = append(files, s.output(path.Join(dir, name)))
		}
	}
	return files
}
