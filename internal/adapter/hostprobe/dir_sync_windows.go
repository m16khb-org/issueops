//go:build windows

package hostprobe

import "os"

// syncDirectory only checks that path is openable: Windows cannot fsync a
// directory handle (FlushFileBuffers needs a writable file), and NTFS journals
// directory entries itself.
func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	return directory.Close()
}
