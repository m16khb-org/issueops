//go:build darwin || linux

package cmux

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	if err := requireSupportedPlatform(); err != nil {
		return nil, err
	}
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

	rootFD, err := unix.Open(root, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open cmux prompt root: %w", err)
	}
	defer unix.Close(rootFD)
	var rootStat unix.Stat_t
	if err := unix.Fstat(rootFD, &rootStat); err != nil {
		return nil, fmt.Errorf("stat cmux prompt root: %w", err)
	}

	components := make([]promptOpenedComponent, 0, len(parts))
	currentFD := rootFD
	for _, name := range parts[:len(parts)-1] {
		fd, openErr := unix.Openat(currentFD, name, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
		if openErr != nil {
			closePromptComponents(components)
			return nil, fmt.Errorf("open cmux prompt directory: %w", openErr)
		}
		var stat unix.Stat_t
		if statErr := unix.Fstat(fd, &stat); statErr != nil || stat.Mode&unix.S_IFMT != unix.S_IFDIR {
			unix.Close(fd)
			closePromptComponents(components)
			return nil, fmt.Errorf("cmux prompt directory identity is unsafe")
		}
		components = append(components, promptOpenedComponent{parentFD: currentFD, name: name, fd: fd, stat: stat})
		currentFD = fd
	}
	defer closePromptComponents(components)

	leafName := parts[len(parts)-1]
	leafFD, err := unix.Openat(currentFD, leafName, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0)
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
	if err := unix.Fstat(leafFD, &before); err != nil || before.Mode&unix.S_IFMT != unix.S_IFREG || before.Mode&0o777 != 0o600 || before.Size < 0 || before.Size > maximumPromptBytes {
		return nil, fmt.Errorf("cmux prompt file boundary is unsafe")
	}
	if afterOpen != nil {
		afterOpen()
	}
	value, err := io.ReadAll(io.LimitReader(file, maximumPromptBytes+1))
	if err != nil {
		return nil, err
	}
	if len(value) > maximumPromptBytes {
		return nil, fmt.Errorf("cmux prompt file exceeds the safe maximum")
	}
	var after unix.Stat_t
	if err := unix.Fstat(leafFD, &after); err != nil || !samePromptStat(before, after) {
		return nil, fmt.Errorf("cmux prompt file identity changed while reading")
	}
	leaf := promptOpenedComponent{parentFD: currentFD, name: leafName, fd: leafFD, stat: before}
	if !promptNamespaceStable(root, rootFD, rootStat, append(append([]promptOpenedComponent(nil), components...), leaf)) {
		return nil, fmt.Errorf("cmux prompt namespace changed while reading")
	}
	if err := validatePromptBytes(value, expectedDigest); err != nil {
		return nil, err
	}
	return value, nil
}

func closePromptComponents(components []promptOpenedComponent) {
	for index := len(components) - 1; index >= 0; index-- {
		_ = unix.Close(components[index].fd)
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
