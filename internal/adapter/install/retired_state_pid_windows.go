//go:build windows

package install

import "issueops/internal/adapter/outbound/osprocess"

// legacyPIDAlive reports whether a process with pid exists and has not exited.
func legacyPIDAlive(pid int) bool { return osprocess.Alive(pid) }
