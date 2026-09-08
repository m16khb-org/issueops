package state

import (
	"os"
	"path/filepath"
)

// 상태 디렉터리 결정은 환경변수와 홈 디렉터리를 읽는 I/O다. 주입 지점은
// 없다 — 이 규칙이 유일한 규칙이며, 테스트는 ISSUEOPS_STATE_DIR로 격리한다.
func stateDir() string {
	if env := os.Getenv("ISSUEOPS_STATE_DIR"); env != "" {
		if abs, err := filepath.Abs(env); err == nil {
			return abs
		}
		return env
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(os.TempDir(), "issueops-state")
	}
	return filepath.Join(home, ".local", "state", "issueops")
}
