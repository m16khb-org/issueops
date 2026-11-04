package issueopslease

import "time"

// UTCClock 타입은 현재 시각을 UTC로 돌려주는 시계다.
type UTCClock struct{}

// Now 메서드는 현재 시각을 UTC로 돌려준다.
func (UTCClock) Now() time.Time { return time.Now().UTC() }
