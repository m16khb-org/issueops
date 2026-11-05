// Package workspacefingerprint computes the quiescence fingerprint of a Git
// worktree: its root, identity, index, diffs and the content of every untracked
// file, hashed in one fixed order.
//
// Two adapters seal and compare this value (the lease replacement snapshot and
// the reseed inventory), so they must agree on every byte. The package is the
// single writer of that format. It holds no policy and no state: callers decide
// which worktree to observe, how Git is run, and what a mismatch means.
package workspacefingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"issueops/internal/adapter/outbound/osfile"
)

// gitRaw runs git in dir and returns its exit code with untrimmed stdout and stderr.
type gitRaw func(dir string, args ...string) (code int, stdout, stderr string)

// Inputs is everything one fingerprint covers. Untracked is the sorted list of
// slash-separated paths relative to Root, as ReadDiffs returns it.
type Inputs struct {
	Root      string
	CommonDir string
	Branch    string
	Head      string
	Index     []byte
	Tracked   string
	Staged    string
	Untracked []string
}

// Digest hashes the inputs and streams each untracked regular file into the
// hash. It fails when an untracked path is unsafe, is not a regular file, or
// changes while it is read.
func Digest(in Inputs) (string, error) {
	hash := sha256.New()
	writePart(hash, in.Root)
	writePart(hash, in.CommonDir)
	writePart(hash, in.Branch)
	writePart(hash, in.Head)
	writeBytes(hash, in.Index)
	writePart(hash, in.Tracked)
	writePart(hash, in.Staged)
	for _, relative := range in.Untracked {
		if err := writeUntracked(hash, in.Root, relative); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// Absent fingerprints the observation that the worktree directory does not
// exist. It is bound to the exact root and branch so one lifecycle's absence
// cannot stand in for another's.
func Absent(root, branch string) string {
	hash := sha256.New()
	writePart(hash, "workspace-absent")
	writePart(hash, root)
	writePart(hash, branch)
	return hex.EncodeToString(hash.Sum(nil))
}

// ReadIndex reads the Git index file that `git rev-parse --git-path index`
// named as gitPath (absolute, or relative to root). A missing index reads as
// nil; any other read failure is returned.
func ReadIndex(root, gitPath string) ([]byte, error) {
	if !filepath.IsAbs(gitPath) {
		gitPath = filepath.Join(root, gitPath)
	}
	index, err := os.ReadFile(gitPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return index, nil
}

// ReadDiffs reads the tracked and staged binary diffs and the sorted untracked
// path list. A command that exits zero is accepted even when it wrote warnings
// to stderr.
func ReadDiffs(git gitRaw, root string) (tracked, staged string, untracked []string, err error) {
	code, tracked, stderr := git(root, "diff", "--binary", "--no-ext-diff", "--")
	if code != 0 {
		return "", "", nil, fmt.Errorf("read tracked diff: %s", strings.TrimSpace(stderr))
	}
	code, staged, stderr = git(root, "diff", "--cached", "--binary", "--no-ext-diff", "--")
	if code != 0 {
		return "", "", nil, fmt.Errorf("read staged diff: %s", strings.TrimSpace(stderr))
	}
	code, raw, stderr := git(root, "ls-files", "--others", "--exclude-standard", "-z")
	if code != 0 {
		return "", "", nil, fmt.Errorf("list untracked files: %s", strings.TrimSpace(stderr))
	}
	untracked = strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00")
	if len(untracked) == 1 && untracked[0] == "" {
		untracked = nil
	}
	sort.Strings(untracked)
	return tracked, staged, untracked, nil
}

type hashWriter interface {
	Write([]byte) (int, error)
}

func writePart(hash hashWriter, value string) {
	writeBytes(hash, []byte(value))
}

func writeBytes(hash hashWriter, value []byte) {
	_, _ = hash.Write([]byte(strconv.Itoa(len(value))))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(value)
	_, _ = hash.Write([]byte{0})
}

// writeUntracked records one safe regular file: its path, mode, and content.
func writeUntracked(hash hashWriter, root, relative string) error {
	if filepath.IsAbs(relative) || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe untracked path %q", relative)
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	entry, err := os.Lstat(path)
	if err != nil || entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
		return fmt.Errorf("untracked path must be a regular file: %s", relative)
	}
	listed, listedKnown := osfile.PathIdentity(path, entry)
	writePart(hash, relative)
	writePart(hash, entry.Mode().String())
	return writeFile(hash, path, entry, listed, listedKnown)
}

// listed is the identity of the entry when it was listed. Unix reads it from
// entry; Windows os.FileInfo resolves file identity lazily by path, so only an
// identity captured at listing time can tell a replaced file from the listed one.
func writeFile(hash hashWriter, path string, entry os.FileInfo, listed osfile.Identity, listedKnown bool) (err error) {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
	}()
	opened, err := file.Stat()
	if err != nil {
		return err
	}
	if !sameListedFile(file, opened, entry, listed, listedKnown) {
		return changedWhileSnapshotting(path)
	}
	_, _ = hash.Write([]byte(strconv.FormatInt(opened.Size(), 10)))
	_, _ = hash.Write([]byte{0})
	if _, err := io.CopyN(hash, file, opened.Size()); err != nil {
		return err
	}
	var extra [1]byte
	if n, err := file.Read(extra[:]); n != 0 || (err != nil && !errors.Is(err, io.EOF)) {
		return changedWhileSnapshotting(path)
	}
	_, _ = hash.Write([]byte{0})
	return nil
}

// sameListedFile reports whether the opened file is still the regular file that
// was listed: same file, same size, and the same identity when one was captured.
func sameListedFile(file *os.File, opened, entry os.FileInfo, listed osfile.Identity, listedKnown bool) bool {
	if !os.SameFile(entry, opened) || !opened.Mode().IsRegular() || opened.Size() != entry.Size() {
		return false
	}
	if !listedKnown {
		return true
	}
	current, ok := osfile.FileIdentity(file, opened)
	return ok && current.Device == listed.Device && current.Inode == listed.Inode
}

func changedWhileSnapshotting(path string) error {
	return fmt.Errorf("untracked file changed while snapshotting: %s", path)
}
