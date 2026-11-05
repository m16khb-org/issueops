package issueopslease

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/outbound/gitexclude"
	"issueops/internal/adapter/outbound/osfile"
	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
)

// ReseedArtifacts 타입은 lease reseed 때 새 세대의 claim token 파일을 만들고 지우는 파일 시스템 어댑터다.
type ReseedArtifacts struct{}

// NewReseedArtifacts 함수는 상태가 없는 ReseedArtifacts를 만든다.
func NewReseedArtifacts() *ReseedArtifacts {
	return &ReseedArtifacts{}
}

// Prepare 메서드는 record의 현재 세대 claim token 파일을 0600 권한으로 새로 만들고 token의 SHA-256과 대상 경로를 담은 영수증을 반환한다.
// 같은 경로의 기존 파일은 먼저 지우며, 쓰기나 검증이 실패하면 만든 파일을 정리한 뒤 오류를 반환한다.
// record에 execution이 없으면 오류를 반환한다.
func (a *ReseedArtifacts) Prepare(_ context.Context, record leasecontract.Record) (leaseapp.ReseedArtifactReceipt, error) {
	if record.Execution == nil {
		return leaseapp.ReseedArtifactReceipt{}, fmt.Errorf("cannot prepare reseed artifacts without an execution")
	}
	paths := reseedArtifactTargetPaths(record)
	path := paths[0]
	if err := cleanupReseedTarget(record); err != nil {
		return leaseapp.ReseedArtifactReceipt{}, err
	}
	if err := reseedMkdirAll(record.Execution.Workspace.Root, filepath.Dir(path)); err != nil {
		return leaseapp.ReseedArtifactReceipt{}, err
	}
	// Best effort: the token must not show up as an untracked change in a
	// target repo; a failure here only leaves git status noisier.
	_ = gitexclude.EnsureWorkspaceState(record.Execution.Workspace.Root)
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return leaseapp.ReseedArtifactReceipt{}, err
	}
	token := fmt.Sprintf("%x", raw)
	file, err := osfile.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return leaseapp.ReseedArtifactReceipt{}, err
	}
	if _, err = file.WriteString(token + "\n"); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return leaseapp.ReseedArtifactReceipt{}, joinReseedArtifactErrors(err, cleanupReseedTarget(record))
	}
	receipt := leasecontract.ReseedReceipt{ClaimTokenPath: path}
	if err := validateReseedReceiptTargets(receipt, paths); err != nil {
		return leaseapp.ReseedArtifactReceipt{}, joinReseedArtifactErrors(err, cleanupReseedTarget(record))
	}
	receipt.Execution = *record.Execution
	return leaseapp.ReseedArtifactReceipt{TokenSHA256: claimTokenSHA256(token), Receipt: receipt, TargetPaths: paths}, nil
}

// Rollback 메서드는 Prepare가 만든 token 파일을 영수증의 대상 경로 안에서만 지운다.
// 영수증 경로가 대상 세대 밖이면 오류를 반환한다.
func (a *ReseedArtifacts) Rollback(_ context.Context, receipt leaseapp.ReseedArtifactReceipt) error {
	return joinReseedArtifactErrors(validateReseedReceiptTargets(receipt.Receipt, receipt.TargetPaths), removeReseedArtifactTargets(receipt.Receipt.Execution.Workspace.Root, receipt.TargetPaths))
}

// CleanupSuperseded 메서드는 대체된 세대의 claim token 파일을 지운다. 파일이 없으면 무시하고 record에 execution이 없으면 오류를 반환한다.
func (a *ReseedArtifacts) CleanupSuperseded(_ context.Context, record leasecontract.Record) error {
	if record.Execution == nil {
		return fmt.Errorf("cannot clean superseded reseed artifacts without an execution")
	}
	return removeReseedArtifactTargets(record.Execution.Workspace.Root, []string{reseedTokenPath(record)})
}

func cleanupReseedTarget(record leasecontract.Record) error {
	if record.Execution == nil {
		return fmt.Errorf("cannot clean reseed artifacts without an execution")
	}
	return removeReseedArtifactTargets(record.Execution.Workspace.Root, reseedArtifactTargetPaths(record))
}

func reseedArtifactTargetPaths(record leasecontract.Record) []string {
	return []string{reseedTokenPath(record)}
}

func removeReseedArtifactTargets(root string, paths []string) error {
	var problems []error
	for _, path := range paths {
		if err := removeReseedRuntimeFile(root, path); err != nil {
			problems = append(problems, err)
		}
	}
	return errors.Join(problems...)
}

func validateReseedReceiptTargets(receipt leasecontract.ReseedReceipt, targets []string) error {
	if len(targets) == 0 {
		return fmt.Errorf("reseed receipt has no target generation paths")
	}
	allowed := make(map[string]struct{}, len(targets))
	for _, path := range targets {
		allowed[path] = struct{}{}
	}
	for _, path := range []string{receipt.ClaimTokenPath} {
		if path == "" {
			continue
		}
		if _, ok := allowed[path]; !ok {
			return fmt.Errorf("reseed receipt path is outside the target generation")
		}
	}
	return nil
}

func joinReseedArtifactErrors(primary, cleanup error) error {
	if cleanup == nil {
		return primary
	}
	if primary == nil {
		return cleanup
	}
	return fmt.Errorf("%w; target artifact cleanup failed: %w", primary, cleanup)
}

func reseedTokenPath(record leasecontract.Record) string {
	key := claimTokenSHA256(record.ID)[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".issueops", "state", "issueops-v1", key, fmt.Sprintf("lease-%d.token", record.Execution.Lease.Generation))
}

func removeReseedRuntimeFile(root, path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if strings.TrimSpace(root) == "" {
		return fmt.Errorf("reseed artifact root is required")
	}
	canonicalRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(canonicalRoot, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("reseed artifact path escapes the canonical worktree")
	}
	if err := requireReseedRealRoot(canonicalRoot); err != nil {
		return err
	}
	parentsExist, err := reseedParentsAreRealDirs(canonicalRoot, rel)
	if err != nil || !parentsExist {
		return err
	}
	return removeReseedRegularFile(target)
}

// requireReseedRealRoot는 root가 symlink가 아닌 실제 디렉터리인지 확인한다.
func requireReseedRealRoot(canonicalRoot string) error {
	rootInfo, err := os.Lstat(canonicalRoot)
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("reseed artifact directory must contain only real directories")
	}
	return nil
}

// reseedParentsAreRealDirs는 rel의 마지막 요소를 제외한 모든 상위 경로가 실제
// 디렉터리인지 확인한다. 상위 경로가 없으면 (false, nil)을 돌려준다.
func reseedParentsAreRealDirs(canonicalRoot, rel string) (bool, error) {
	current := canonicalRoot
	parts := strings.Split(rel, string(filepath.Separator))
	for index, part := range parts {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		if index == len(parts)-1 {
			break
		}
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return false, fmt.Errorf("reseed artifact directory must contain only real directories")
		}
	}
	return true, nil
}

// removeReseedRegularFile은 target이 일반 파일일 때만 지우고, 없으면 무시한다.
func removeReseedRegularFile(target string) error {
	info, err := os.Lstat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("reseed runtime artifact must be a regular file")
	}
	return os.Remove(target)
}

func reseedMkdirAll(root, target string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	rootInfo, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 || !rootInfo.IsDir() {
		return fmt.Errorf("reseed artifact directory must contain only real directories")
	}
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("reseed artifact directory escapes the canonical worktree")
	}
	return reseedMkdirWalk(root, rel)
}

// reseedMkdirWalk는 root에서 rel의 각 구성요소를 따라 내려가며 없는 디렉터리만 0700으로 만들고,
// 이미 있는 구성요소는 symlink가 아닌 실제 디렉터리인지 확인한다.
func reseedMkdirWalk(root, rel string) error {
	current := root
	for part := range strings.SplitSeq(rel, string(filepath.Separator)) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, fs.ErrNotExist) {
			if err := osfile.Mkdir(current, 0o700); err != nil {
				return err
			}
			continue
		}
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("reseed artifact directory must contain only real directories")
		}
	}
	return nil
}
