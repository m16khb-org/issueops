//go:build windows

package hostprobe

import (
	"fmt"
	"io"
	"os"

	"issueops/internal/adapter/outbound/osfile"
)

// readOmoAuthFile reads an owner-only regular auth file and verifies that the
// path still names the opened file, unchanged, after the read.
func readOmoAuthFile(path string, maxBytes int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !omoAuthInfoValid(path, before, maxBytes) {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || !omoAuthInfoValid(path, opened, maxBytes) {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	afterOpened, err := file.Stat()
	if err != nil || !stableOmoAuthInfo(opened, afterOpened) {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	afterPath, err := os.Lstat(path)
	if err != nil || !os.SameFile(afterPath, opened) || afterPath.Mode() != opened.Mode() || afterPath.Size() != opened.Size() {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	return data, nil
}

func stableOmoAuthInfo(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}

func omoAuthInfoValid(path string, info os.FileInfo, maxBytes int64) bool {
	return info.Mode().IsRegular() && osfile.Perm(path, info) == 0o600 && info.Size() <= maxBytes
}
