//go:build unix

package hostprobe

import "os"

// syncDirectory makes the entries created in path durable.
func syncDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
