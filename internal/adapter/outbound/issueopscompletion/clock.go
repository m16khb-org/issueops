package issueopscompletion

import "time"

// UTCClock reports the current time in UTC for completion timestamps.
type UTCClock struct{}

// Now returns the current time converted to UTC.
func (UTCClock) Now() time.Time { return time.Now().UTC() }
