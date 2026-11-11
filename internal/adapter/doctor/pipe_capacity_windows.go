//go:build windows

package doctor

import "golang.org/x/sys/windows"

// MeasurePipeCapacity fills a fresh anonymous pipe in PIPE_NOWAIT mode until a
// write accepts no bytes, so the buffered byte count is the pipe capacity
// os/exec children get: CreatePipe with the default buffer size.
func MeasurePipeCapacity() (int, error) {
	var read, write windows.Handle
	if err := windows.CreatePipe(&read, &write, nil, 0); err != nil {
		return 0, err
	}
	defer func() { _ = windows.CloseHandle(read) }()
	defer func() { _ = windows.CloseHandle(write) }()
	mode := uint32(windows.PIPE_NOWAIT)
	if err := windows.SetNamedPipeHandleState(write, &mode, nil, nil); err != nil {
		return 0, err
	}
	chunk := make([]byte, 512)
	total := 0
	for total < 1<<20 {
		var n uint32
		if err := windows.WriteFile(write, chunk, &n, nil); err != nil {
			return total, err
		}
		if n == 0 {
			return total, nil
		}
		total += int(n)
	}
	return total, nil
}
