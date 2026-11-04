package issueopslease

import (
	"path/filepath"
	"strings"
)

// FilesystemPathMatcher 타입은 실제 파일 시스템 기준으로 두 경로가 같은 위치인지 비교한다.
type FilesystemPathMatcher struct{}

// Matches 메서드는 두 경로를 절대 경로로 바꾸고 심볼릭 링크를 풀어 정리한 뒤 같은 위치이면 true를 반환한다.
// 경로가 없어 링크를 풀 수 없으면 정리된 절대 경로끼리 비교하고, 절대 경로를 만들 수 없으면 false를 반환한다.
func (FilesystemPathMatcher) Matches(left, right string) bool {
	leftPath, err := filepath.Abs(strings.TrimSpace(left))
	if err != nil {
		return false
	}
	rightPath, err := filepath.Abs(strings.TrimSpace(right))
	if err != nil {
		return false
	}
	if resolved, err := filepath.EvalSymlinks(leftPath); err == nil {
		leftPath = resolved
	}
	if resolved, err := filepath.EvalSymlinks(rightPath); err == nil {
		rightPath = resolved
	}
	return filepath.Clean(leftPath) == filepath.Clean(rightPath)
}
