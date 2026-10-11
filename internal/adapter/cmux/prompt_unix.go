//go:build darwin || linux

package cmux

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"
)

type promptOpenedComponent struct {
	parentFD int
	name     string
	fd       int
	stat     unix.Stat_t
}

func readPromptPlatform(root, path, expectedDigest string, afterOpen func()) ([]byte, error) {
	parts, err := promptRelativeParts(root, path, expectedDigest)
	if err != nil {
		return nil, err
	}

	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open cmux prompt root: %w", err)
	}
	defer unix.Close(rootFD)
	var rootStat unix.Stat_t
	if err := unix.Fstat(rootFD, &rootStat); err != nil {
		return nil, fmt.Errorf("stat cmux prompt root: %w", err)
	}

	components, currentFD, err := openPromptDirectories(rootFD, parts[:len(parts)-1])
	if err != nil {
		return nil, err
	}
	defer closePromptComponents(components)

	walk := promptWalk{root: root, rootFD: rootFD, rootStat: rootStat, components: components, parentFD: currentFD}
	return walk.readLeaf(path, parts[len(parts)-1], expectedDigest, afterOpen)
}

// promptWalk은 루트에서 leaf 부모 디렉터리까지 열어 둔 경로 상태를 담는다.
type promptWalk struct {
	root       string
	rootFD     int
	rootStat   unix.Stat_t
	components []promptOpenedComponent
	parentFD   int
}

// promptRelativeParts는 경로 입력을 검증하고 root 기준 상대 경로 구성요소를 반환한다.
func promptRelativeParts(root, path, expectedDigest string) ([]string, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || !filepath.IsAbs(path) || filepath.Clean(path) != path ||
		strings.ContainsRune(root+path, 0) || !validDigest(expectedDigest) {
		return nil, fmt.Errorf("cmux prompt file must be a clean path inside the canonical worktree")
	}
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return nil, fmt.Errorf("cmux prompt file must be inside the canonical worktree")
	}
	parts := strings.Split(relative, string(filepath.Separator))
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, fmt.Errorf("cmux prompt file path is unsafe")
		}
	}
	return parts, nil
}

// openPromptDirectories는 rootFD 아래 디렉터리를 순서대로 열고 마지막 디렉터리 fd를 반환한다.
// 실패하면 이미 연 구성요소를 닫고 반환한다.
func openPromptDirectories(rootFD int, names []string) ([]promptOpenedComponent, int, error) {
	components := make([]promptOpenedComponent, 0, len(names)+1)
	currentFD := rootFD
	for _, name := range names {
		fd, openErr := unix.Openat(currentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			closePromptComponents(components)
			return nil, 0, fmt.Errorf("open cmux prompt directory: %w", openErr)
		}
		var stat unix.Stat_t
		if statErr := unix.Fstat(fd, &stat); statErr != nil || stat.Mode&unix.S_IFMT != unix.S_IFDIR {
			unix.Close(fd)
			closePromptComponents(components)
			return nil, 0, fmt.Errorf("cmux prompt directory identity is unsafe")
		}
		components = append(components, promptOpenedComponent{parentFD: currentFD, name: name, fd: fd, stat: stat})
		currentFD = fd
	}
	return components, currentFD, nil
}

// readLeaf는 leaf 파일을 열어 읽고, 읽는 동안 파일 신원과 경로 네임스페이스가 바뀌지 않았는지 확인한다.
func (walk *promptWalk) readLeaf(path, leafName, expectedDigest string, afterOpen func()) ([]byte, error) {
	leafFD, err := unix.Openat(walk.parentFD, leafName, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open cmux prompt file: %w", err)
	}
	file := os.NewFile(uintptr(leafFD), path)
	if file == nil {
		unix.Close(leafFD)
		return nil, fmt.Errorf("open cmux prompt file handle")
	}
	defer file.Close()
	var before unix.Stat_t
	if err := unix.Fstat(leafFD, &before); err != nil {
		return nil, fmt.Errorf("stat cmux prompt file: %w", err)
	}
	expectedUID := uint32(os.Geteuid())
	if err := validatePromptLeafStat(before, expectedUID); err != nil {
		return nil, err
	}
	if afterOpen != nil {
		afterOpen()
	}
	value, err := io.ReadAll(io.LimitReader(file, MaximumPromptBytes+1))
	if err != nil {
		return nil, err
	}
	if len(value) > MaximumPromptBytes {
		return nil, fmt.Errorf("cmux prompt file exceeds the safe maximum")
	}
	var after unix.Stat_t
	if err := unix.Fstat(leafFD, &after); err != nil {
		return nil, fmt.Errorf("restat cmux prompt file: %w", err)
	}
	if err := validatePromptLeafStat(after, expectedUID); err != nil || !samePromptStat(before, after) {
		return nil, fmt.Errorf("cmux prompt file identity changed while reading")
	}
	leaf := promptOpenedComponent{parentFD: walk.parentFD, name: leafName, fd: leafFD, stat: before}
	if !promptNamespaceStable(walk.root, walk.rootFD, walk.rootStat, append(append([]promptOpenedComponent(nil), walk.components...), leaf)) {
		return nil, fmt.Errorf("cmux prompt namespace changed while reading")
	}
	if err := validatePromptBytes(value, expectedDigest); err != nil {
		return nil, err
	}
	return value, nil
}

func validatePromptLeafStat(stat unix.Stat_t, expectedUID uint32) error {
	if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Mode&0o777 != 0o600 || stat.Size < 0 {
		return fmt.Errorf("cmux prompt leaf must be a regular mode-0600 file")
	}
	if stat.Uid != expectedUID {
		return fmt.Errorf("cmux prompt leaf owner must match the current effective uid")
	}
	if stat.Size > MaximumPromptBytes {
		return fmt.Errorf("cmux prompt file exceeds the safe maximum")
	}
	return nil
}

func closePromptComponents(components []promptOpenedComponent) {
	for _, component := range slices.Backward(components) {
		_ = unix.Close(component.fd)
	}
}

func promptNamespaceStable(root string, rootFD int, rootStat unix.Stat_t, components []promptOpenedComponent) bool {
	var currentRoot unix.Stat_t
	if err := unix.Lstat(root, &currentRoot); err != nil || !samePromptIdentity(rootStat, currentRoot) {
		return false
	}
	var openedRoot unix.Stat_t
	if err := unix.Fstat(rootFD, &openedRoot); err != nil || !samePromptStat(rootStat, openedRoot) {
		return false
	}
	for _, component := range components {
		var namespace unix.Stat_t
		if err := unix.Fstatat(component.parentFD, component.name, &namespace, unix.AT_SYMLINK_NOFOLLOW); err != nil ||
			!samePromptIdentity(component.stat, namespace) || namespace.Mode&unix.S_IFMT == unix.S_IFLNK {
			return false
		}
	}
	return true
}

func samePromptIdentity(left, right unix.Stat_t) bool {
	return left.Dev == right.Dev && left.Ino == right.Ino && left.Mode&unix.S_IFMT == right.Mode&unix.S_IFMT
}

func samePromptStat(left, right unix.Stat_t) bool {
	return samePromptIdentity(left, right) && left.Mode == right.Mode && left.Size == right.Size && left.Uid == right.Uid && left.Gid == right.Gid
}
