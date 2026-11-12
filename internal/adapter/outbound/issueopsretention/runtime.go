// Package issueopsretention is the outbound adapter for the IssueOps retention policy: it
// exposes the record store to the retention use case (list, read, and a conditional delete
// that retains the outcome row) and supplies the system clock.
package issueopsretention

import "time"

// SystemClock is the wall-clock time source for retention decisions.
type SystemClock struct{}

// Now returns the current local time.
func (SystemClock) Now() time.Time {
	return time.Now()
}
