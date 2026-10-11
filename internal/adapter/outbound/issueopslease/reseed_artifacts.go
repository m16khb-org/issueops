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

	leaseapp "issueops/internal/application/issueopslease"
	leasecontract "issueops/internal/contract/issueopslease"
)

type ReseedOwnerArtifacts func(context.Context, leasecontract.Record) (leasecontract.ReseedReceipt, error)

type ReseedArtifacts struct{ owner ReseedOwnerArtifacts }

func NewReseedArtifacts(owner ReseedOwnerArtifacts) *ReseedArtifacts {
	return &ReseedArtifacts{owner: owner}
}

func (a *ReseedArtifacts) Prepare(ctx context.Context, record leasecontract.Record) (leaseapp.ReseedArtifactReceipt, error) {
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
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return leaseapp.ReseedArtifactReceipt{}, err
	}
	token := fmt.Sprintf("%x", raw)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
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
	if record.Execution.Mode == "orca" {
		if a.owner == nil {
			return leaseapp.ReseedArtifactReceipt{}, joinReseedArtifactErrors(fmt.Errorf("Orca reseed owner artifact writer is required"), cleanupReseedTarget(record))
		}
		receipt, err = a.owner(ctx, record)
		if err != nil {
			return leaseapp.ReseedArtifactReceipt{}, joinReseedArtifactErrors(err, cleanupReseedTarget(record))
		}
		receipt.ClaimTokenPath = path
	}
	if err := validateReseedReceiptTargets(receipt, paths); err != nil {
		return leaseapp.ReseedArtifactReceipt{}, joinReseedArtifactErrors(err, cleanupReseedTarget(record))
	}
	receipt.Execution = *record.Execution
	return leaseapp.ReseedArtifactReceipt{TokenSHA256: claimTokenSHA256(token), Receipt: receipt, TargetPaths: paths}, nil
}

func (a *ReseedArtifacts) Rollback(_ context.Context, receipt leaseapp.ReseedArtifactReceipt) error {
	return joinReseedArtifactErrors(validateReseedReceiptTargets(receipt.Receipt, receipt.TargetPaths), removeReseedArtifactTargets(receipt.Receipt.Execution.Workspace.Root, receipt.TargetPaths))
}

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
	paths := []string{reseedTokenPath(record)}
	if record.Execution != nil && record.Execution.Mode == "orca" {
		paths = append(paths, reseedOwnerArtifactPaths(record)...)
	}
	return paths
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
	for _, path := range []string{receipt.ClaimTokenPath, receipt.ContextPacketPath, receipt.OwnerPromptPath} {
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

func reseedOwnerArtifactPaths(record leasecontract.Record) []string {
	key := claimTokenSHA256(record.ID)[:16]
	base := filepath.Join(record.Execution.Workspace.Root, ".issueops", "state", "issueops-v1", key, "generation-"+fmt.Sprintf("%d", record.Execution.Lease.Generation))
	return []string{filepath.Join(base, "context.json"), filepath.Join(base, "owner-prompt.txt")}
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
			if err := os.Mkdir(current, 0o700); err != nil {
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
