// Package installutil is the shared adapter toolkit of the host installers
// (Claude, Codex and others): planning and applying file, JSON and symlink
// writes with dry-run support, skill link planning and pruning, hook config
// validation and drift/generation checks, activation evidence capture, and the
// transaction that adopts a regular command file as a managed symlink.
package installutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/adapter/outbound/osfile"
	"issueops/internal/port"
)

// PlanHostSkillLinks plans the symlinks that expose the shared skills of root
// in destRoot for host. It returns the enabled skill names, the planned
// links (including stale-link prunes), human-readable messages, and
// errors. With dryRun nothing on disk changes.
func PlanHostSkillLinks(root, destRoot string, skillNames []string, host string, dryRun bool) ([]string, []port.InstallLink, []string, []error) {
	enabledSkills, skippedSkills := SkillNamesForHost(root, skillNames, host)
	messages := make([]string, 0, len(skippedSkills))
	for _, skillName := range skippedSkills {
		messages = append(messages, "skip skill for "+host+": "+skillName)
	}
	links, errs := planSkillLinks(root, destRoot, enabledSkills, dryRun)
	pruned, pruneErrs := pruneStaleSkillLinks(root, destRoot, dryRun)
	for _, link := range pruned {
		verb := "prune"
		if link.WouldRemove {
			verb = "would prune"
		}
		messages = append(messages, verb+" stale skill link for "+host+": "+filepath.Base(link.Path)+" (target missing: "+link.Target+")")
	}
	links = append(links, pruned...)
	errs = append(errs, pruneErrs...)
	return enabledSkills, links, messages, errs
}

// pruneStaleSkillLinks removes symlinks in destRoot that point into this
// checkout's skills/ directory but whose target no longer exists, which is what
// a removed or renamed shared skill leaves behind in every host skill
// directory. Only harness-owned links are touched: links that point elsewhere,
// links whose target still exists, and non-symlink entries are left alone.
// dryRun reports WouldRemove without deleting. A missing destRoot is not an
// error because a fresh install has nothing to prune.
func pruneStaleSkillLinks(root, destRoot string, dryRun bool) ([]port.InstallLink, []error) {
	entries, err := os.ReadDir(destRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, []error{err}
	}
	skillsRoot := filepath.Clean(filepath.Join(root, "skills")) + string(filepath.Separator)
	var links []port.InstallLink
	var errs []error
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}
		path := filepath.Join(destRoot, entry.Name())
		target, err := os.Readlink(path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		resolved := target
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(destRoot, resolved)
		}
		if !strings.HasPrefix(filepath.Clean(resolved), skillsRoot) {
			continue
		}
		if _, err := os.Stat(resolved); err == nil {
			continue
		} else if !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
			continue
		}
		link := port.InstallLink{Path: path, Target: target}
		if dryRun {
			link.WouldRemove = true
		} else if err := os.Remove(path); err != nil {
			errs = append(errs, err)
			continue
		} else {
			link.Removed = true
		}
		links = append(links, link)
	}
	return links, errs
}

func planSkillLinks(root, destRoot string, skillNames []string, dryRun bool) ([]port.InstallLink, []error) {
	links := make([]port.InstallLink, 0, len(skillNames))
	var errs []error
	for _, skillName := range skillNames {
		link, err := EnsureSymlinkPlan(filepath.Join(root, "skills", skillName), filepath.Join(destRoot, skillName), dryRun)
		links = append(links, link)
		if err != nil {
			errs = append(errs, err)
		}
	}
	return links, errs
}

// WriteTextPlan writes content to path when it differs from the current
// file, creating parent directories and applying perm. With dryRun it only
// marks the result WouldWrite. An unchanged file reports neither flag.
func WriteTextPlan(path, kind, content string, perm os.FileMode, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	if existing, err := os.ReadFile(path); err == nil && bytes.Equal(existing, []byte(content)) {
		return file, nil
	} else if err != nil && !errors.Is(err, fs.ErrNotExist) && !dryRun {
		return file, err
	}
	if dryRun {
		file.WouldWrite = true
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	if err := osfile.WriteFile(path, []byte(content), perm); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}

// WriteJSONPlan writes value to path as indented JSON with a trailing newline
// through WriteTextPlan.
func WriteJSONPlan(path, kind string, value any, perm os.FileMode, dryRun bool) (port.InstallFile, error) {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return port.InstallFile{Path: path, Kind: kind}, err
	}
	return WriteTextPlan(path, kind, string(append(b, '\n')), perm, dryRun)
}

// EnsureSymlinkPlan ensures path is a symlink to target, creating parent
// directories and replacing a symlink that points elsewhere. It refuses to
// replace a non-symlink path. With dryRun it only marks the result
// WouldCreate.
func EnsureSymlinkPlan(target, path string, dryRun bool) (port.InstallLink, error) {
	link := port.InstallLink{Path: path, Target: target}
	info, err := os.Lstat(path)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return link, fmt.Errorf("refusing to replace non-symlink path: %s", path)
		}
		current, readErr := os.Readlink(path)
		if readErr == nil && current == target {
			return link, nil
		}
		if dryRun {
			link.WouldCreate = true
			return link, nil
		}
		if err := os.Remove(path); err != nil {
			return link, err
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return link, err
	}
	if dryRun {
		link.WouldCreate = true
		return link, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return link, err
	}
	if err := os.Symlink(target, path); err != nil {
		return link, err
	}
	link.Created = true
	return link, nil
}

// TOMLString returns value as a double-quoted string using JSON escaping,
// which the installers embed in generated TOML; an encoding failure yields
// the empty string literal.
func TOMLString(value string) string {
	b, err := json.Marshal(value)
	if err != nil {
		return `""`
	}
	return string(b)
}
