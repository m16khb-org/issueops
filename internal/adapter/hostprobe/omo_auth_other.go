//go:build !unix

package hostprobe

import (
	"fmt"
	"io"
	"os"
)

func readOmoAuthFile(path string, maxBytes int64) ([]byte, error) {
	beforePath, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !beforePath.Mode().IsRegular() || beforePath.Mode()&os.ModeSymlink != 0 || beforePath.Mode().Perm() != 0o600 || beforePath.Size() > maxBytes {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	beforeOpened, err := file.Stat()
	if err != nil || !stableOmoAuthInfo(beforePath, beforeOpened) {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil || int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	afterOpened, err := file.Stat()
	afterPath, pathErr := os.Lstat(path)
	if err != nil || pathErr != nil || !stableOmoAuthInfo(beforeOpened, afterOpened) || !stableOmoAuthInfo(afterOpened, afterPath) {
		return nil, fmt.Errorf("omo_auth_invalid")
	}
	return data, nil
}

func stableOmoAuthInfo(before, after os.FileInfo) bool {
	return os.SameFile(before, after) && before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime().Equal(after.ModTime())
}
