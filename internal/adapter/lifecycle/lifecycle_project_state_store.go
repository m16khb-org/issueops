// Package lifecycle is the outbound adapter that stores the project lifecycle
// profile as JSON under the per-project state directory, using atomic writes, and
// derives the lifecycle state paths from a project fingerprint.
package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"issueops/internal/adapter/lifecycle/model"
	"issueops/internal/adapter/outbound/osfile"
	lifecyclecontract "issueops/internal/contract/lifecycle"
	lifecycledomain "issueops/internal/domain/lifecycle"
)

// ProfileFiles reads and writes project lifecycle profiles. Normalize and
// ObserveFingerprint are injected by the composition root; StateDir is the root
// that holds the "projects/<repo id>" directories.
type ProfileFiles struct {
	Normalize          func(string) (string, error)
	StateDir           string
	ObserveFingerprint func(string) lifecyclecontract.ProjectFingerprint
}

// NormalizeRoot returns the canonical form of the repository root.
func (files ProfileFiles) NormalizeRoot(root string) (string, error) { return files.Normalize(root) }

// StateRoot returns the state directory that holds all project lifecycle state.
func (files ProfileFiles) StateRoot() string { return files.StateDir }

// Fingerprint observes the identity fingerprint of the project at root.
func (files ProfileFiles) Fingerprint(root string) lifecyclecontract.ProjectFingerprint {
	return files.ObserveFingerprint(root)
}

// Paths returns the lifecycle state plan for root: the project state directory
// named by the fingerprint's repo id and the profile, doc-upkeep queue, and
// compact capsule file paths inside it. It does not touch the file system.
func (files ProfileFiles) Paths(root string, projectFingerprint lifecyclecontract.ProjectFingerprint) lifecyclecontract.ProjectLifecycleStatePlan {
	repoID := lifecycledomain.RepoID(projectFingerprint)
	stateRoot := files.StateDir
	projectDir := filepath.Join(stateRoot, "projects", repoID)
	return lifecyclecontract.ProjectLifecycleStatePlan{
		OK:              true,
		SchemaVersion:   model.ProjectLifecycleSchemaVersion,
		RepoRoot:        root,
		RepoID:          repoID,
		StateRoot:       stateRoot,
		ProjectStateDir: projectDir,
		ProjectJSONPath: filepath.Join(projectDir, model.ProjectLifecycleProfileFile),
		QueuePath:       filepath.Join(projectDir, model.DocUpkeepQueueFile),
		CompactPath:     filepath.Join(projectDir, model.CompactCapsuleFile),
		Fingerprint:     projectFingerprint,
		Warnings:        []string{},
	}
}

// ReadProfile decodes the lifecycle profile stored at path.
func (files ProfileFiles) ReadProfile(path string) (lifecyclecontract.ProjectLifecycleProfile, error) {
	return readProjectLifecycleProfile(path)
}

// Mkdir creates path and any missing parents with owner-only permissions.
func (files ProfileFiles) Mkdir(path string) error { return osfile.MkdirAll(path, 0o700) }

// Create writes profile to path only if no file exists there. The content becomes
// visible fully written, so a concurrent loser never reads a partial file.
func (files ProfileFiles) Create(path string, profile lifecyclecontract.ProjectLifecycleProfile) error {
	return createJSONAtomic(path, profile, 0o600)
}

// Write atomically replaces the profile stored at path with profile.
func (files ProfileFiles) Write(path string, profile lifecyclecontract.ProjectLifecycleProfile) error {
	return writeJSONAtomic(path, profile, 0o600)
}

// Now returns the current time.
func (files ProfileFiles) Now() time.Time { return time.Now() }

func readProjectLifecycleProfile(path string) (lifecyclecontract.ProjectLifecycleProfile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return lifecyclecontract.ProjectLifecycleProfile{}, err
	}
	var profile lifecyclecontract.ProjectLifecycleProfile
	if err := json.Unmarshal(b, &profile); err != nil {
		return lifecyclecontract.ProjectLifecycleProfile{}, err
	}
	return profile, nil
}

func writeJSONAtomic(path string, value any, perm os.FileMode) error {
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	writeErr := func() error {
		if _, err := tmp.Write(append(b, '\n')); err != nil {
			return err
		}
		if err := osfile.FileChmod(tmp, perm); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return writeErr
	}
	if err := osfile.Replace(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func createJSONAtomic(path string, value any, perm os.FileMode) error {
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	writeErr := func() error {
		if _, err := tmp.Write(append(b, '\n')); err != nil {
			return err
		}
		if err := osfile.FileChmod(tmp, perm); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		return writeErr
	}
	// Hard-link the fully written temp file into place. Link fails with
	// EEXIST when the path already exists (os.IsExist unwraps the LinkError),
	// preserving O_EXCL's single-winner semantics while guaranteeing the file
	// is complete the moment it becomes visible — no read-while-write window
	// for the losing session.
	return os.Link(tmpName, path)
}
