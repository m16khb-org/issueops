//go:build unix

package issueops

import "syscall"

func signalCleanupProcess(pid int, sig syscall.Signal) error { return syscall.Kill(pid, sig) }
