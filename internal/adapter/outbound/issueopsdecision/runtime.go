// Package issueopsdecision is the outbound adapter that backs the issueops
// decision service with the record store and the system clock.
package issueopsdecision

import "time"

// SystemClock is the wall-clock time source for the decision service.
type SystemClock struct{}

// Now returns the current local time.
func (SystemClock) Now() time.Time {
	return time.Now()
}
