package issueopslease

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
	leasedomain "issueops/internal/domain/issueopslease"
)

type ReseedInventory struct {
	inspect leaseapp.ProcessInspector
}

func NewReseedInventory(inspect leaseapp.ProcessInspector) *ReseedInventory {
	return &ReseedInventory{inspect: inspect}
}

func (a *ReseedInventory) Observe(ctx context.Context, record leasecontract.Record, requester leasedomain.Actor) (string, error) {
	if a == nil || a.inspect == nil || record.Execution == nil {
		return "", fmt.Errorf("reseed inventory requires an execution and process inspector")
	}
	snapshot, err := reseedWorkspaceSnapshot(record.Execution.Workspace)
	if err != nil {
		return "", err
	}
	processStatus := "none"
	if holder := record.Execution.Lease.Holder; holder != nil && holder.SessionProcess != nil {
		processStatus, _, err = a.inspect(ctx, leasedomain.ProcessReceipt{PID: holder.SessionProcess.PID, StartedAt: holder.SessionProcess.StartedAt, Executable: holder.SessionProcess.Executable})
		if err != nil {
			return "", err
		}
	}
	payload := struct {
		ID         string               `json:"id"`
		Generation uint64               `json:"generation"`
		Status     string               `json:"status"`
		Holder     *leasecontract.Actor `json:"holder,omitempty"`
		Requester  leasecontract.Actor  `json:"requester"`
		Process    string               `json:"process_status"`
		Snapshot   string               `json:"snapshot"`
	}{record.ID, record.Execution.Lease.Generation, record.Execution.Lease.Status, record.Execution.Lease.Holder, reseedInventoryActor(requester), processStatus, snapshot}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func reseedInventoryActor(actor leasedomain.Actor) leasecontract.Actor {
	result := leasecontract.Actor{Host: actor.Host, SessionID: actor.SessionID, AgentID: actor.AgentID}
	if actor.Process != nil {
		result.SessionProcess = &leasecontract.ProcessReceipt{PID: actor.Process.PID, StartedAt: actor.Process.StartedAt, Executable: actor.Process.Executable}
	}
	return result
}

func reseedWorkspaceSnapshot(workspace leasecontract.Workspace) (string, error) {
	if err := reseedRequireCanonicalWorktree(workspace); err != nil {
		return "", err
	}
	head, err := reseedGitOutput(workspace.Root, "rev-parse", "HEAD")
	if err != nil {
		return "", err
	}
	commonDir, err := reseedGitOutput(workspace.Root, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	indexBytes, err := reseedIndexBytes(workspace.Root)
	if err != nil {
		return "", err
	}
	tracked, staged, err := reseedGitDiffs(workspace.Root)
	if err != nil {
		return "", err
	}
	untracked, err := reseedUntrackedFiles(workspace.Root)
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	reseedWriteFingerprintPart(hash, workspace.Root)
	reseedWriteFingerprintPart(hash, commonDir)
	reseedWriteFingerprintPart(hash, workspace.Branch)
	reseedWriteFingerprintPart(hash, head)
	reseedWriteFingerprintBytes(hash, indexBytes)
	reseedWriteFingerprintPart(hash, tracked)
	reseedWriteFingerprintPart(hash, staged)
	for _, relative := range untracked {
		if err := reseedWriteUntrackedFingerprint(hash, workspace.Root, relative); err != nil {
			return "", err
		}
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// reseedRequireCanonicalWorktree는 작업공간 루트가 실제 디렉터리이고 Git top-level 및 현재 브랜치와 일치하는지 확인한다.
func reseedRequireCanonicalWorktree(workspace leasecontract.Workspace) error {
	info, err := os.Lstat(workspace.Root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("canonical worktree must be a real directory")
	}
	top, err := reseedGitOutput(workspace.Root, "rev-parse", "--show-toplevel")
	if err != nil || !(FilesystemPathMatcher{}).Matches(top, workspace.Root) {
		return fmt.Errorf("canonical worktree root does not match Git top-level")
	}
	branch, err := reseedGitOutput(workspace.Root, "branch", "--show-current")
	if err != nil || branch != workspace.Branch {
		return fmt.Errorf("canonical worktree branch mismatch")
	}
	return nil
}

// reseedIndexBytes는 Git index 파일 내용을 읽는다. 파일이 없으면 빈 값을 돌려준다.
func reseedIndexBytes(root string) ([]byte, error) {
	indexPath, err := reseedGitOutput(root, "rev-parse", "--git-path", "index")
	if err != nil {
		return nil, err
	}
	if !filepath.IsAbs(indexPath) {
		indexPath = filepath.Join(root, indexPath)
	}
	indexBytes, err := os.ReadFile(indexPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	return indexBytes, nil
}

// reseedGitDiffs는 추적 파일의 작업 트리 diff와 staged diff를 순서대로 읽는다.
func reseedGitDiffs(root string) (tracked, staged string, err error) {
	code, tracked, stderr := reseedGitRaw(root, "diff", "--binary", "--no-ext-diff", "--")
	if code != 0 {
		return "", "", fmt.Errorf("read tracked diff: %s", strings.TrimSpace(stderr))
	}
	code, staged, stderr = reseedGitRaw(root, "diff", "--cached", "--binary", "--no-ext-diff", "--")
	if code != 0 {
		return "", "", fmt.Errorf("read staged diff: %s", strings.TrimSpace(stderr))
	}
	return tracked, staged, nil
}

// reseedUntrackedFiles는 추적되지 않은 파일의 상대 경로를 정렬해 돌려준다. 없으면 nil이다.
func reseedUntrackedFiles(root string) ([]string, error) {
	code, untrackedRaw, stderr := reseedGitRaw(root, "ls-files", "--others", "--exclude-standard", "-z")
	if code != 0 {
		return nil, fmt.Errorf("list untracked files: %s", strings.TrimSpace(stderr))
	}
	untracked := strings.Split(strings.TrimSuffix(untrackedRaw, "\x00"), "\x00")
	if len(untracked) == 1 && untracked[0] == "" {
		untracked = nil
	}
	sort.Strings(untracked)
	return untracked, nil
}

// reseedWriteUntrackedFingerprint는 추적되지 않은 일반 파일 하나의 경로, 모드, 내용을 지문에 기록한다.
func reseedWriteUntrackedFingerprint(hash io.Writer, root, relative string) error {
	if filepath.IsAbs(relative) || strings.HasPrefix(filepath.Clean(relative), ".."+string(filepath.Separator)) {
		return fmt.Errorf("unsafe untracked path %q", relative)
	}
	path := filepath.Join(root, filepath.FromSlash(relative))
	entry, err := os.Lstat(path)
	if err != nil || entry.Mode()&os.ModeSymlink != 0 || !entry.Mode().IsRegular() {
		return fmt.Errorf("untracked path must be a regular file: %s", relative)
	}
	reseedWriteFingerprintPart(hash, relative)
	reseedWriteFingerprintPart(hash, entry.Mode().String())
	return reseedWriteFingerprintFile(hash, path, entry)
}

func reseedGitOutput(root string, args ...string) (string, error) {
	code, stdout, stderr := reseedGitRaw(root, args...)
	if code != 0 {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), stderr)
	}
	return strings.TrimSpace(stdout), nil
}

func reseedGitRaw(root string, args ...string) (int, string, string) {
	command := exec.Command("git", args...)
	command.Dir = root
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String()
	}
	return 1, stdout.String(), err.Error()
}

func reseedWriteFingerprintPart(hash interface{ Write([]byte) (int, error) }, value string) {
	reseedWriteFingerprintBytes(hash, []byte(value))
}

func reseedWriteFingerprintBytes(hash interface{ Write([]byte) (int, error) }, value []byte) {
	_, _ = hash.Write([]byte(strconv.Itoa(len(value))))
	_, _ = hash.Write([]byte{0})
	_, _ = hash.Write(value)
	_, _ = hash.Write([]byte{0})
}

func reseedWriteFingerprintFile(hash interface{ Write([]byte) (int, error) }, path string, entry os.FileInfo) (err error) {
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
	if !os.SameFile(entry, opened) || !opened.Mode().IsRegular() || opened.Size() != entry.Size() {
		return fmt.Errorf("untracked file changed while snapshotting: %s", path)
	}
	_, _ = hash.Write([]byte(strconv.FormatInt(opened.Size(), 10)))
	_, _ = hash.Write([]byte{0})
	if _, err := io.CopyN(hash, file, opened.Size()); err != nil {
		return err
	}
	var extra [1]byte
	if n, err := file.Read(extra[:]); n != 0 || (err != nil && !errors.Is(err, io.EOF)) {
		return fmt.Errorf("untracked file changed while snapshotting: %s", path)
	}
	_, _ = hash.Write([]byte{0})
	return nil
}
