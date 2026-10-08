// Package agentmodelconfig reads and writes the global and local agent model
// settings files and the Codex role files injected into owner sessions.
package agentmodelconfig

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	contract "issueops/internal/contract/agentmodel"
	"issueops/internal/domain/agentmodel"
	"issueops/internal/domain/repoidentity"
)

const (
	localRelPath = ".issueops/agent-models.local.json"
	// excludeLine keeps the local settings file out of git status.
	excludeLine = "/" + localRelPath
)

// GlobalPath follows XDG: $XDG_CONFIG_HOME/issueops/agent-models.json, or
// ~/.config/issueops/agent-models.json when the variable is empty.
func GlobalPath(getenv func(string) string, home string) string {
	base := strings.TrimSpace(getenv("XDG_CONFIG_HOME"))
	if base == "" {
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "issueops", "agent-models.json")
}

// Local is the main-worktree settings file of a repository. Path is empty
// outside a git repository.
type Local struct {
	Path      string
	CommonDir string
}

// LocalPath finds the local settings file in the main worktree, so every
// linked worktree of the repository shares one file.
func LocalPath(ctx context.Context, repo string) (Local, error) {
	if info, err := os.Stat(repo); err != nil || !info.IsDir() {
		return Local{}, nil
	}
	command := exec.CommandContext(ctx, "git", "-C", repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	command.Env = withoutGitEnvironment(os.Environ())
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return Local{}, ctxErr
		}
		if _, exited := errors.AsType[*exec.ExitError](err); exited && strings.Contains(stderr.String(), "not a git repository") {
			return Local{}, nil
		}
		return Local{}, fmt.Errorf("resolve git common dir for %s: %w: %s", repo, err, strings.TrimSpace(stderr.String()))
	}
	commonDir := strings.TrimSpace(stdout.String())
	main := repoidentity.SourceRoot(repo, commonDir)
	return Local{Path: filepath.Join(main, filepath.FromSlash(localRelPath)), CommonDir: commonDir}, nil
}

// Read decodes a settings file strictly. An absent file is the zero Config.
// Every error names the file.
func Read(path string) (contract.Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return contract.Config{}, nil
	}
	if err != nil {
		return contract.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var cfg contract.Config
	if err := decoder.Decode(&cfg); err != nil {
		return contract.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return contract.Config{}, fmt.Errorf("%s: trailing data after the settings object", path)
	}
	if cfg.Version == 0 {
		return contract.Config{}, fmt.Errorf("%s: missing version (want %d)", path, contract.ConfigVersion)
	}
	if err := agentmodel.ValidateConfig(cfg); err != nil {
		return contract.Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, nil
}

// Write renders cfg deterministically and replaces path atomically.
func Write(path string, cfg contract.Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return writeAtomic(path, append(data, '\n'))
}

// WriteLocal writes the local settings file and registers it in
// <common-dir>/info/exclude. excluded reports whether the line was added now.
func WriteLocal(local Local, cfg contract.Config) (excluded bool, err error) {
	if local.Path == "" {
		return false, fmt.Errorf("local settings need a git repository")
	}
	if excluded, err = EnsureExcluded(local.CommonDir); err != nil {
		return false, err
	}
	return excluded, Write(local.Path, cfg)
}

// EnsureExcluded appends excludeLine to <common-dir>/info/exclude once.
func EnsureExcluded(commonDir string) (bool, error) {
	path := filepath.Join(commonDir, "info", "exclude")
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if strings.TrimSpace(line) == excludeLine {
			return false, nil
		}
	}
	if len(data) > 0 && !bytes.HasSuffix(data, []byte("\n")) {
		data = append(data, '\n')
	}
	data = append(data, excludeLine+"\n"...)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	return true, nil
}

// CodexCatalog lists the model slugs in $CODEX_HOME/models_cache.json
// (default ~/.codex). An absent or unreadable cache is an empty list.
func CodexCatalog(getenv func(string) string, home string) []string {
	dir := strings.TrimSpace(getenv("CODEX_HOME"))
	if dir == "" {
		dir = filepath.Join(home, ".codex")
	}
	data, err := os.ReadFile(filepath.Join(dir, "models_cache.json"))
	if err != nil {
		return nil
	}
	var cache struct {
		Models []struct {
			Slug string `json:"slug"`
		} `json:"models"`
	}
	if json.Unmarshal(data, &cache) != nil {
		return nil
	}
	var slugs []string
	for _, model := range cache.Models {
		if model.Slug != "" {
			slugs = append(slugs, model.Slug)
		}
	}
	return slugs
}

// WriteRoleFile stores a Codex role file under <stateDir>/agent-roles/ by
// content hash. The same content always maps to the same path, so concurrent
// writers never disagree and an existing file is not rewritten.
func WriteRoleFile(stateDir, content string) (string, error) {
	sum := sha256.Sum256([]byte(content))
	path := filepath.Join(stateDir, "agent-roles", hex.EncodeToString(sum[:])+".toml")
	if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
		return path, nil
	}
	return path, writeAtomic(path, []byte(content))
}

func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".*")
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func withoutGitEnvironment(environment []string) []string {
	filtered := make([]string, 0, len(environment))
	for _, entry := range environment {
		if !strings.HasPrefix(entry, "GIT_") {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// Files are the decoded settings files of one repository.
type Files struct {
	Local      contract.Config
	Global     contract.Config
	LocalPath  string
	GlobalPath string
}

// Load reads the global file and, inside a git repository, the
// main-worktree local file.
func Load(ctx context.Context, repo string, getenv func(string) string, home string) (Files, error) {
	files := Files{GlobalPath: GlobalPath(getenv, home)}
	local, err := LocalPath(ctx, repo)
	if err != nil {
		return Files{}, err
	}
	files.LocalPath = local.Path
	if files.Global, err = Read(files.GlobalPath); err != nil {
		return Files{}, err
	}
	if local.Path != "" {
		if files.Local, err = Read(local.Path); err != nil {
			return Files{}, err
		}
	}
	return files, nil
}
