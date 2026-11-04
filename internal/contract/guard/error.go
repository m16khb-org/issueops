package guard

// BlockedError 타입은 guard 점검이 차단 finding으로 끝났음을 나타내는 오류다.
// 메시지는 첫 finding의 규칙 이름을 담는다.
type BlockedError struct {
	Findings []Finding
}

func (e BlockedError) Error() string {
	if len(e.Findings) == 0 {
		return "guard check blocked"
	}
	return "guard check blocked: " + e.Findings[0].Rule
}

// IsGuardBlocked 함수는 err가 BlockedError 값 그 자체인지 알려준다. 감싼 오류는
// 찾지 않는다. 종료 코드 3이 직접 차단에만 대응하기 때문이다.
func IsGuardBlocked(err error) bool {
	_, ok := err.(BlockedError) //nolint:errorlint // exit code 3 maps only a direct guard block; TestIsGuardBlockedMatchesDirectErrorOnly pins it
	return ok
}
