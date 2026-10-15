package issueops

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	leasecontract "issueops/internal/contract/issueopslease"
)

func writeSealedArtifact(root, path string, value []byte) error {
	if len(value) == 0 || len(value) > leasecontract.OwnerArtifactMaxBytes {
		return fmt.Errorf("sealed artifact is empty or oversized")
	}
	if err := ensureSealedArtifactDirectory(root, filepath.Dir(path)); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, os.ErrExist) {
		current, readErr := os.ReadFile(path)
		info, statErr := os.Lstat(path)
		if readErr != nil || statErr != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 || !strings.EqualFold(digestBytes(current), digestBytes(value)) {
			return fmt.Errorf("immutable sealed artifact already exists with different identity")
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

func ensureSealedArtifactDirectory(root, target string) error {
	root, target = filepath.Clean(root), filepath.Clean(target)
	rel, err := filepath.Rel(root, target)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("sealed artifact directory must be inside the canonical worktree")
	}
	current := root
	for part := range strings.SplitSeq(rel, string(os.PathSeparator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if errors.Is(statErr, os.ErrNotExist) {
			if mkdirErr := os.Mkdir(current, 0o700); mkdirErr != nil && !errors.Is(mkdirErr, os.ErrExist) {
				return mkdirErr
			}
			info, statErr = os.Lstat(current)
		}
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("sealed artifact path must contain only real directories")
		}
	}
	return ensureSealedArtifactIgnore(target)
}

// ensureSealedArtifactIgnore는 봉인 아티팩트 디렉터리에 자기 무시
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
func ensureSealedArtifactIgnore(dir string) error {
	path := filepath.Join(dir, ".gitignore")
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("sealed artifact ignore rule must be a regular file")
		}
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte("*\n"), 0o600)
}

func quoteShellArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func digestBytes(value []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(value))
}
