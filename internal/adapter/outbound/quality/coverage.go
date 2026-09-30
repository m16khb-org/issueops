package quality

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	contract "issueops/internal/contract/quality"
	policy "issueops/internal/domain/quality"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

func RunGoTestCoverage(root string, execute func(context.Context, string) (string, error), cacheBase func() (string, error)) (string, error) {
	fingerprint, fingerprintErr := CoverageFingerprint(root)
	if fingerprintErr == nil {
		if output, ok := readCoverageCache(root, fingerprint, cacheBase); ok {
			return output, nil
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), CoverageCommandTimeout)
	defer cancel()
	output, err := execute(ctx, root)
	if err == nil && fingerprintErr == nil {
		writeCoverageCache(root, fingerprint, output, cacheBase)
	}
	return output, err
}

// CoverageCommandTimeout leaves bounded headroom above the measured 270-second
// repository-wide warm-cache run. A two-minute deadline made a healthy first
// collection fail before it could seed the exact-fingerprint cache.
const CoverageCommandTimeout = 10 * time.Minute
const CoverageCommandOutputLimit = 16 * 1024 * 1024
const coverageFingerprintGitOutputLimit = 64 * 1024 * 1024
const coverageFingerprintFileLimit = 64 * 1024 * 1024
const coverageFingerprintTotalFileLimit = 512 * 1024 * 1024
const coverageFingerprintFileCountLimit = 10_000

func ExecuteGoTestCoverage(ctx context.Context, root string) (string, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "-cover", "./...")
	cmd.Dir = root
	out := NewBoundedQualityBuffer(CoverageCommandOutputLimit)
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return out.String(), ctx.Err()
	}
	if out.Truncated() {
		return out.String(), fmt.Errorf("coverage command output exceeds %d bytes", CoverageCommandOutputLimit)
	}
	return out.String(), err
}

const coverageCacheVersion = 2

type coverageCacheEntry struct {
	Version     int                        `json:"version"`
	Fingerprint string                     `json:"fingerprint"`
	Packages    []contract.CoveragePackage `json:"packages"`
}

type BoundedQualityBuffer struct {
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func NewBoundedQualityBuffer(limit int) BoundedQualityBuffer {
	return BoundedQualityBuffer{limit: limit}
}

func (buffer *BoundedQualityBuffer) Write(value []byte) (int, error) {
	size := len(value)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		if len(value) > remaining {
			value = value[:remaining]
		}
		_, _ = buffer.buffer.Write(value)
	}
	if size > remaining {
		buffer.truncated = true
	}
	return size, nil
}

func (buffer *BoundedQualityBuffer) String() string  { return buffer.buffer.String() }
func (buffer *BoundedQualityBuffer) Truncated() bool { return buffer.truncated }

func DefaultCoverageCacheBase() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "issueops", "quality-coverage"), nil
}

func CoverageFingerprint(root string) (string, error) {
	head, err := gitCoverageFingerprintInput(root, "rev-parse", "--verify", "HEAD")
	if err != nil {
		return "", err
	}
	diff, err := gitCoverageFingerprintInput(
		root,
		"diff",
		"--binary",
		"--no-ext-diff",
		"--no-textconv",
		"HEAD",
		"--",
	)
	if err != nil {
		return "", err
	}
	untracked, err := gitCoverageFingerprintInput(root, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return "", err
	}
	hasher := sha256.New()
	_, _ = fmt.Fprintf(
		hasher,
		"coverage-v%d\ngo=%s\ngoos=%s\ngoarch=%s\n",
		coverageCacheVersion,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
	environment := coverageEnvironmentFingerprint()
	sort.Strings(environment)
	for _, value := range environment {
		_, _ = fmt.Fprintf(hasher, "env:%s\n", value)
	}
	_, _ = hasher.Write(head)
	_, _ = hasher.Write(diff)
	paths := bytes.Split(untracked, []byte{0})
	sort.Slice(paths, func(left, right int) bool {
		return bytes.Compare(paths[left], paths[right]) < 0
	})
	fileCount := 0
	var totalFileBytes int64
	for _, rawPath := range paths {
		if len(rawPath) == 0 {
			continue
		}
		relative := filepath.Clean(string(rawPath))
		if relative == "." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("coverage fingerprint path escapes repository: %s", relative)
		}
		path := filepath.Join(root, relative)
		info, err := os.Lstat(path)
		if err != nil {
			return "", err
		}
		_, _ = fmt.Fprintf(hasher, "path:%s\nmode:%s\n", filepath.ToSlash(relative), info.Mode())
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return "", err
			}
			_, _ = hasher.Write([]byte(target))
			continue
		}
		if !info.Mode().IsRegular() {
			continue
		}
		fileCount++
		if fileCount > coverageFingerprintFileCountLimit {
			return "", fmt.Errorf("coverage fingerprint exceeds %d untracked files", coverageFingerprintFileCountLimit)
		}
		if info.Size() > coverageFingerprintFileLimit {
			return "", fmt.Errorf("coverage fingerprint file %s exceeds %d bytes", relative, coverageFingerprintFileLimit)
		}
		totalFileBytes += info.Size()
		if totalFileBytes > coverageFingerprintTotalFileLimit {
			return "", fmt.Errorf("coverage fingerprint exceeds %d untracked bytes", coverageFingerprintTotalFileLimit)
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		_, copyErr := io.Copy(hasher, file)
		closeErr := file.Close()
		if copyErr != nil {
			return "", copyErr
		}
		if closeErr != nil {
			return "", closeErr
		}
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func gitCoverageFingerprintInput(root string, args ...string) ([]byte, error) {
	command := exec.Command("git", args...)
	command.Dir = root
	output := NewBoundedQualityBuffer(coverageFingerprintGitOutputLimit)
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Run(); err != nil {
		return nil, err
	}
	if output.Truncated() {
		return nil, fmt.Errorf("git coverage fingerprint output exceeds %d bytes", coverageFingerprintGitOutputLimit)
	}
	return append([]byte(nil), output.buffer.Bytes()...), nil
}

func coverageEnvironmentFingerprint() []string {
	result := []string{}
	for _, entry := range os.Environ() {
		name, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(name, "GO") ||
			name == "CGO_ENABLED" ||
			name == "CC" ||
			name == "CXX" ||
			name == "PATH" {
			result = append(result, entry)
		}
	}
	return result
}

func CoverageCachePath(root string, cacheBase func() (string, error)) (string, error) {
	base, err := cacheBase()
	if err != nil {
		return "", err
	}
	rootHash := sha256.Sum256([]byte(filepath.Clean(root)))
	return filepath.Join(base, hex.EncodeToString(rootHash[:16])+".json"), nil
}

func readCoverageCache(root, fingerprint string, cacheBase func() (string, error)) (string, bool) {
	path, err := CoverageCachePath(root, cacheBase)
	if err != nil {
		return "", false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var entry coverageCacheEntry
	if json.Unmarshal(data, &entry) != nil ||
		entry.Version != coverageCacheVersion ||
		entry.Fingerprint != fingerprint ||
		len(entry.Packages) == 0 {
		return "", false
	}
	return renderCoveragePackages(entry.Packages), true
}

func writeCoverageCache(root, fingerprint, output string, cacheBase func() (string, error)) {
	path, err := CoverageCachePath(root, cacheBase)
	if err != nil || os.MkdirAll(filepath.Dir(path), 0o700) != nil {
		return
	}
	packages := policy.ParseCoveragePackages(output, 101)
	if len(packages) == 0 {
		return
	}
	data, err := json.Marshal(coverageCacheEntry{
		Version:     coverageCacheVersion,
		Fingerprint: fingerprint,
		Packages:    packages,
	})
	if err != nil {
		return
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".coverage-*.tmp")
	if err != nil {
		return
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return
	}
	if err := temporary.Close(); err != nil {
		_ = temporary.Close()
		return
	}
	_ = os.Rename(temporaryPath, path)
}

func renderCoveragePackages(packages []contract.CoveragePackage) string {
	var output strings.Builder
	for _, item := range packages {
		_, _ = fmt.Fprintf(&output, "%s coverage: %.1f%% of statements\n", item.Package, item.Coverage)
	}
	return output.String()
}
