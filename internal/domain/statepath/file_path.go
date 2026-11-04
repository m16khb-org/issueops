// Package statepath 패키지는 상태 키를 검증·정규화하고 상태 파일 경로로 조합하는 순수 규칙을 소유한다.
package statepath

import "path/filepath"

// Path 함수는 상태 키를 파일 경로로 조합한다. 순수 계산이므로 domain 계층이 소유한다.
func Path(dir, key string) string {
	return filepath.Join(dir, key+".json")
}
