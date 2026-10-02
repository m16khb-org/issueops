//go:build !darwin && !linux

package mcpservice

import (
	"errors"
	"os"
)

var errLockHeld = errors.New("lock held")

var errLockUnsupported = errors.New("mcp http instance lock is unsupported on this platform")

func tryLock(*os.File) error { return errLockUnsupported }

func LockHolder(string) (int, error) { return 0, errLockUnsupported }
