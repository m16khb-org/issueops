//go:build unix

package sqlstore

// isTransientSidecarError always reports false on Unix: unlinking a file
// another process holds removes the name at once, so a sidecar error there is
// never a pending deletion and the repair fails without waiting.
func isTransientSidecarError(error) bool { return false }
