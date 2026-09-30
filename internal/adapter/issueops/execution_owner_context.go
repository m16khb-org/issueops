package issueops

import (
	"crypto/sha256"
	_ "embed"
	"errors"
	"fmt"
	"issueops/internal/contract/issueops"
	leasecontract "issueops/internal/contract/issueopslease"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

//go:embed testdata/execution_owner_prompt.txt
var executionOwnerPromptTemplate string

func ExecutionOwnerPromptTemplate() string { return executionOwnerPromptTemplate }
func readExecutionOwnerArtifact(root, path string) ([]byte, error) {
	root, path = filepath.Clean(root), filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("owner artifact must be inside the canonical worktree")
	}
	current := root
	parts := strings.Split(rel, string(os.PathSeparator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("owner artifact path contains a missing entry or symlink")
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("owner artifact ancestor is not a directory")
		}
		if index == len(parts)-1 && (!info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || info.Size() > leasecontract.OwnerArtifactMaxBytes) {
			return nil, fmt.Errorf("owner artifact must be a private bounded regular file")
		}
	}
	return os.ReadFile(path)
}

// SealedOwnerContextPacketPath는 현재 세대 봉인 packet의 경로를 돌려준다.
// 훅 가드가 봉인 실존을 확인하는 유일한 경로다 — 경로 규칙을 다른 계층에
// 복제하지 않기 위해 노출한다. execution이 없으면 빈 문자열이다.
func SealedOwnerContextPacketPath(record issueops.IssueOpsRecord) string {
	if record.Execution == nil {
		return ""
	}
	packetPath, _ := executionOwnerArtifactPaths(record)
	return packetPath
}

func executionOwnerArtifactPaths(record issueops.IssueOpsRecord) (string, string) {
	key := digestExecutionOwnerBytes([]byte(record.ID))[:16]
	base := filepath.Join(record.Execution.Workspace.Root, ".issueops", "state", "issueops-v1", key, "generation-"+strconv.FormatUint(record.Execution.Lease.Generation, 10))
	return filepath.Join(base, "context.json"), filepath.Join(base, "owner-prompt.txt")
}

func executionOwnerVerificationReportPath(record issueops.IssueOpsRecord) string {
	key := digestExecutionOwnerBytes([]byte(record.ID))[:16]
	return filepath.Join(record.Execution.Workspace.Root, ".issueops", "verified-execution", "issueops-v1-"+key+".json")
}

func writeExecutionOwnerArtifact(root, path string, value []byte) error {
	if len(value) == 0 || len(value) > leasecontract.OwnerArtifactMaxBytes {
		return fmt.Errorf("owner artifact is empty or oversized")
	}
	if err := ensureExecutionOwnerArtifactDirectory(root, filepath.Dir(path)); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		current, readErr := os.ReadFile(path)
		info, statErr := os.Lstat(path)
		if readErr != nil || statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || !strings.EqualFold(digestExecutionOwnerBytes(current), digestExecutionOwnerBytes(value)) {
			return fmt.Errorf("immutable owner artifact already exists with different identity")
		}
		return nil
	}
	if err != nil {
		return err
	}
	if _, err = file.Write(value); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func ensureExecutionOwnerArtifactDirectory(root, target string) error {
	root, target = filepath.Clean(root), filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("owner artifact directory must be inside the canonical worktree")
	}
	current := root
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			if mkdirErr := os.Mkdir(current, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
				return mkdirErr
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("owner artifact path must contain only real directories")
		}
	}
	return ensureExecutionOwnerArtifactIgnore(target)
}

// ensureExecutionOwnerArtifactIgnore는 봉인 아티팩트 디렉터리에 자기 무시
// `.gitignore`를 둔다.
//
// 아티팩트는 하네스가 대상 저장소의 워크트리 안에 쓰지만 저장소의 산출물이
// 아니다. 무시 규칙이 없으면 `git status`가 `?? .issueops/`를 보고하고,
// 그 dirt가 strict PR readiness의 `worktree_clean`과 `cleanup finish`를 막는다.
// 하네스가 만든 흔적이 하네스 자신의 게이트를 막는 셈이고, 구현을 모두 마친
// PR 게이트에서야 `worktree_clean` 한 단어로 드러나 원인을 찾기 어렵다.
// `ChangeFingerprint`가 미추적 경로를 포함하므로 뒤늦게 손으로 ignore하면
// 이번에는 `ai_slop_clean_stale`이 뒤따른다.
//
// 규칙은 아티팩트 디렉터리 안에만 둔다. 대상 저장소의 추적 파일(.gitignore)을
// 하네스가 대신 고치지 않으므로 사용자의 다른 미추적 변경은 그대로 보인다.
// 패턴 `*`는 이 파일 자신도 포함하므로 디렉터리 전체가 조용해진다.
// 이미 규칙 파일이 있으면 손대지 않는다: 운영자가 고쳤을 수 있고, 이 함수는
// 아티팩트를 쓸 때마다 호출되므로 멱등해야 한다.
func ensureExecutionOwnerArtifactIgnore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("owner artifact ignore rule must be a regular file")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte("*\n"), 0o600)
}

func executionOwnerRegularFiles(root string, candidates []string) []string {
	out := make([]string, 0, len(candidates))
	for _, rel := range candidates {
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
		if err == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			out = append(out, rel)
		}
	}
	return out
}

func quoteExecutionOwnerArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func digestExecutionOwnerBytes(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}
