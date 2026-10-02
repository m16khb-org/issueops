package mcpcli

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

const (
	httpBearerBytes   = 32
	httpBearerFile    = "bearer"
	httpStateDirName  = "mcp-http"
	maxHTTPBearerSize = 256
)

var errHTTPBearerFile = errors.New("mcp http bearer file must be an owner-only regular file")

// EnsureHTTPBearer returns the 256-bit bearer at <state>/mcp-http/bearer,
// creating it owner-only when absent. The value is never printed.
func EnsureHTTPBearer(stateDir string) (string, error) {
	dir, err := httpStateDir(stateDir)
	if err != nil {
		return "", err
	}
	path := filepath.Join(dir, httpBearerFile)
	bearer, err := readHTTPBearer(path)
	if !errors.Is(err, fs.ErrNotExist) {
		return bearer, err
	}
	if err := createHTTPBearer(dir, path); err != nil && !errors.Is(err, fs.ErrExist) {
		return "", err
	}
	return readHTTPBearer(path)
}

func httpStateDir(stateDir string) (string, error) {
	if !filepath.IsAbs(stateDir) {
		return "", fmt.Errorf("mcp http state dir must be absolute")
	}
	dir := filepath.Join(filepath.Clean(stateDir), httpStateDirName)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	if !info.IsDir() || info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("mcp http state dir must be an owner-only directory")
	}
	return dir, nil
}

// createHTTPBearer publishes a fully written temp file with a no-clobber link,
// so concurrent starters never observe a partial bearer.
func createHTTPBearer(dir, path string) error {
	secret := make([]byte, httpBearerBytes)
	if _, err := io.ReadFull(rand.Reader, secret); err != nil {
		return fmt.Errorf("generate mcp http bearer: %w", err)
	}
	temp, err := os.CreateTemp(dir, ".bearer-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.WriteString(base64.RawURLEncoding.EncodeToString(secret)); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Link(temp.Name(), path)
}

func readHTTPBearer(path string) (string, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm()&0o077 != 0 {
		return "", errHTTPBearerFile
	}
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	after, err := file.Stat()
	if err != nil || !os.SameFile(before, after) {
		return "", errHTTPBearerFile
	}
	data, err := io.ReadAll(io.LimitReader(file, maxHTTPBearerSize+1))
	if err != nil {
		return "", err
	}
	bearer := strings.TrimSpace(string(data))
	if err := validateHTTPBearer(bearer); err != nil {
		return "", err
	}
	return bearer, nil
}

func validateHTTPBearer(bearer string) error {
	decoded, err := base64.RawURLEncoding.DecodeString(bearer)
	if err != nil || len(decoded) < httpBearerBytes {
		return fmt.Errorf("mcp http bearer must carry at least 256 bits")
	}
	return nil
}
