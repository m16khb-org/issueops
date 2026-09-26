package quality

import (
	"testing"
	"time"
)

func TestBoundedQualityBufferCapsCapturedBytes(t *testing.T) {
	buffer := NewBoundedQualityBuffer(4)
	if written, err := buffer.Write([]byte("abcdef")); err != nil || written != 6 {
		t.Fatalf("write = %d, %v", written, err)
	}
	if got := buffer.String(); got != "abcd" || !buffer.Truncated() {
		t.Fatalf("bounded output = %q truncated=%v", got, buffer.Truncated())
	}
}
func TestCoverageCommandTimeoutIncludesColdCacheHeadroom(t *testing.T) {
	if CoverageCommandTimeout < 10*time.Minute {
		t.Fatalf("coverage command timeout = %s, want at least 10m", CoverageCommandTimeout)
	}
}
