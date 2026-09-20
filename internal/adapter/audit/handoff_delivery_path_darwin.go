//go:build darwin

package audit

// macOS exposes /var and /tmp as root-owned aliases into /private. Resolve only
// those fixed aliases; every caller-controlled component below them remains
// subject to the no-follow walk.
func handoffDeliveryTrustedBases() []string {
	return []string{"/var", "/tmp"}
}
