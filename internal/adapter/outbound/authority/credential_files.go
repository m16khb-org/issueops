package authority

import (
	"errors"
	"path/filepath"

	domain "issueops/internal/domain/authority"
	authorityport "issueops/internal/port/authority"
)

const maxCredentialBytes = 256

// CredentialFiles manages immutable owner-only credential files under
// <state>/mcp-http/grants/<key>/<sha256(token)>. Any process running as the same
// OS user that knows a path can exercise its capability; that is the trust boundary.
type CredentialFiles struct{ StateDir string }

var _ authorityport.CredentialFiles = CredentialFiles{}

var errCredentialPath = errors.New("authority credential path is not a managed credential file")

func (f CredentialFiles) grantsDir() string {
	return filepath.Join(filepath.Clean(f.StateDir), "mcp-http", "grants")
}

func (f CredentialFiles) managedComponents(path string) (key, digest string, err error) {
	if !filepath.IsAbs(f.StateDir) || !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "", "", errCredentialPath
	}
	keyDir := filepath.Dir(path)
	if filepath.Dir(keyDir) != f.grantsDir() {
		return "", "", errCredentialPath
	}
	key, digest = filepath.Base(keyDir), filepath.Base(path)
	if !domain.ValidKey(key) || !domain.ValidKey(digest) {
		return "", "", errCredentialPath
	}
	return key, digest, nil
}

func validateCredential(key, digest, token string) error {
	tokenKey, ok := domain.TokenKey(token)
	if !ok || tokenKey != key || domain.TokenDigest(token) != digest {
		return errCredentialPath
	}
	return nil
}
