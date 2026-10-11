package installcli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"issueops/internal/port"
)

type installHostPathSnapshot struct {
	path       string
	mode       os.FileMode
	content    []byte
	linkTarget string
	kind       installHostPathKind
	existed    bool
	wasSymlink bool
}

type installHostPathKind uint8

const (
	installHostFile installHostPathKind = iota
	installHostLink
)

type installHostTransaction struct {
	snapshots []installHostPathSnapshot
	parents   []installHostPathSnapshot
}

// collectInstallHostPaths는 plan의 파일/링크 경로를 정리해 종류별로 모으고, 같은 경로가 두 종류로 선언되면 오류를 낸다.
func collectInstallHostPaths(plan port.NativeInstallResult) (map[string]installHostPathKind, error) {
	paths := map[string]installHostPathKind{}
	add := func(raw string, want installHostPathKind) error {
		if raw == "" {
			return nil
		}
		path := filepath.Clean(raw)
		if kind, exists := paths[path]; exists && kind != want {
			return fmt.Errorf("native install path is both file and symlink: %s", path)
		}
		paths[path] = want
		return nil
	}
	for _, file := range plan.Files {
		if err := add(file.Path, installHostFile); err != nil {
			return nil, err
		}
	}
	for _, link := range plan.Links {
		if err := add(link.Path, installHostLink); err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// captureInstallHostParents는 path의 상위 디렉터리 스냅샷을 존재하는 첫 디렉터리까지 기록한다.
func (transaction *installHostTransaction) captureInstallHostParents(path string, parentSnapshots map[string]installHostPathSnapshot) error {
	for parent := filepath.Dir(path); parent != "." && parent != string(filepath.Separator); parent = filepath.Dir(parent) {
		if parentSnapshot, seen := parentSnapshots[parent]; seen {
			if parentSnapshot.existed {
				break
			}
			continue
		}
		parentSnapshot, parentErr := captureInstallHostParent(parent)
		if parentErr != nil {
			return parentErr
		}
		parentSnapshots[parent] = parentSnapshot
		transaction.parents = append(transaction.parents, parentSnapshot)
		if parentSnapshot.existed {
			break
		}
	}
	return nil
}

func prepareInstallHostTransaction(plan port.NativeInstallResult) (*installHostTransaction, error) {
	paths, err := collectInstallHostPaths(plan)
	if err != nil {
		return nil, err
	}
	ordered := make([]string, 0, len(paths))
	for path := range paths {
		ordered = append(ordered, path)
	}
	sort.Strings(ordered)
	transaction := &installHostTransaction{}
	parentSnapshots := map[string]installHostPathSnapshot{}
	for _, path := range ordered {
		snapshot, err := captureInstallHostPath(path, paths[path])
		if err != nil {
			return nil, err
		}
		transaction.snapshots = append(transaction.snapshots, snapshot)
		if err := transaction.captureInstallHostParents(path, parentSnapshots); err != nil {
			return nil, err
		}
	}
	sort.Slice(transaction.parents, func(left, right int) bool {
		return pathDepth(transaction.parents[left].path) > pathDepth(transaction.parents[right].path)
	})
	return transaction, nil
}

func captureInstallHostParent(path string) (installHostPathSnapshot, error) {
	snapshot := installHostPathSnapshot{path: path}
	info, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	if !info.IsDir() {
		return snapshot, fmt.Errorf("native install rollback parent is not a directory: %s", path)
	}
	snapshot.existed = true
	return snapshot, nil
}

func captureInstallHostPath(path string, kind installHostPathKind) (installHostPathSnapshot, error) {
	snapshot := installHostPathSnapshot{path: path, kind: kind}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	snapshot.existed = true
	if kind == installHostLink {
		if info.Mode()&os.ModeSymlink == 0 {
			return snapshot, fmt.Errorf("native install rollback expected symlink path: %s", path)
		}
		snapshot.linkTarget, err = os.Readlink(path)
		return snapshot, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		snapshot.wasSymlink = true
		snapshot.linkTarget, err = os.Readlink(path)
		if err != nil {
			return snapshot, err
		}
		info, err = os.Stat(path)
		if err != nil {
			return snapshot, err
		}
	}
	if !info.Mode().IsRegular() {
		return snapshot, fmt.Errorf("native install rollback cannot snapshot non-file path: %s", path)
	}
	snapshot.mode = info.Mode()
	snapshot.content, err = os.ReadFile(path)
	return snapshot, err
}

func (transaction *installHostTransaction) rollback() error {
	if transaction == nil {
		return nil
	}
	var errs []error
	for _, v := range slices.Backward(transaction.snapshots) {
		if err := restoreInstallHostPath(v); err != nil {
			errs = append(errs, err)
		}
	}
	for _, snapshot := range transaction.parents {
		if snapshot.existed {
			continue
		}
		if err := removeInstallDirectoryIfEmpty(snapshot.path); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (transaction *installHostTransaction) RollbackHosts() error {
	return transaction.rollback()
}

// removeInstallPathIfPresent removes path and treats a missing path as success.
func removeInstallPathIfPresent(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// writeInstallHostSnapshotContent writes the snapshot content and enforces its permission bits.
func writeInstallHostSnapshotContent(snapshot installHostPathSnapshot) error {
	if err := os.WriteFile(snapshot.path, snapshot.content, snapshot.mode.Perm()); err != nil {
		return err
	}
	return os.Chmod(snapshot.path, snapshot.mode.Perm())
}

// removeNonRegularInstallHostPath removes path only when it exists and is not a regular file.
func removeNonRegularInstallHostPath(path string) error {
	info, err := os.Lstat(path)
	if err == nil && !info.Mode().IsRegular() {
		return os.Remove(path)
	}
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func restoreInstallHostPath(snapshot installHostPathSnapshot) error {
	if !snapshot.existed {
		return removeInstallPathIfPresent(snapshot.path)
	}
	if err := os.MkdirAll(filepath.Dir(snapshot.path), 0o755); err != nil {
		return err
	}
	if snapshot.kind == installHostLink {
		if err := removeInstallPathIfPresent(snapshot.path); err != nil {
			return err
		}
		return os.Symlink(snapshot.linkTarget, snapshot.path)
	}
	if snapshot.wasSymlink {
		if err := removeInstallPathIfPresent(snapshot.path); err != nil {
			return err
		}
		if err := os.Symlink(snapshot.linkTarget, snapshot.path); err != nil {
			return err
		}
		return writeInstallHostSnapshotContent(snapshot)
	}
	if err := removeNonRegularInstallHostPath(snapshot.path); err != nil {
		return err
	}
	return writeInstallHostSnapshotContent(snapshot)
}

func pathDepth(path string) int {
	depth := 0
	for path != "." && path != string(filepath.Separator) {
		depth++
		next := filepath.Dir(path)
		if next == path {
			break
		}
		path = next
	}
	return depth
}

func removeInstallDirectoryIfEmpty(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("native install rollback parent changed to non-directory: %s", path)
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return nil
	}
	return os.Remove(path)
}
