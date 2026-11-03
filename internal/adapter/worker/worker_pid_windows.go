//go:build windows

package worker

import "issueops/internal/adapter/outbound/osprocess"

// isPIDAlive returns true if a process with the given pid exists.
func isPIDAlive(pid int) bool { return osprocess.Alive(pid) }
