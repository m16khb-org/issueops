//go:build unix

package doctor

import (
	"errors"
	"syscall"
)

// MeasurePipeCapacity fills a fresh pipe with non-blocking writes until the
// kernel answers EAGAIN, so the buffered byte count is the pipe capacity. It
// returns in well under a millisecond instead of waiting on an idle timer.
func MeasurePipeCapacity() (int, error) {
	var fds [2]int
	if err := syscall.Pipe(fds[:]); err != nil {
		return 0, err
	}
	defer syscall.Close(fds[0])
	defer syscall.Close(fds[1])
	if err := syscall.SetNonblock(fds[1], true); err != nil {
		return 0, err
	}
	chunk := make([]byte, 512)
	total := 0
	for total < 1<<20 {
		n, err := syscall.Write(fds[1], chunk)
		if n > 0 {
			total += n
		}
		if errors.Is(err, syscall.EAGAIN) {
			return total, nil
		}
		if errors.Is(err, syscall.EINTR) {
			continue
		}
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
