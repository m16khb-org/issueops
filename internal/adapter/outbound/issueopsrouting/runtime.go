// Package issueopsrouting is the outbound adapter that backs the issueops routing
// service with the record store and the system clock.
package issueopsrouting

import (
	"time"
)

// SystemClock is the wall-clock time source for the routing service.
type SystemClock struct{}

// Now returns the current local time.
func (SystemClock) Now() time.Time {
	return time.Now()
}
