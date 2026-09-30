package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"issueops/internal/adapter/lifecycle/model"
	lifecyclecontract "issueops/internal/contract/lifecycle"
	lifecycledomain "issueops/internal/domain/lifecycle"
)

type ProfileFiles struct {
	Normalize          func(string) (string, error)
	StateDir           string
	ObserveFingerprint func(string) lifecyclecontract.ProjectFingerprint
}

func (files ProfileFiles) NormalizeRoot(root string) (string, error) { return files.Normalize(root) }
func (files ProfileFiles) StateRoot() string                         { return files.StateDir }
func (files ProfileFiles) Fingerprint(root string) lifecyclecontract.ProjectFingerprint {
	return files.ObserveFingerprint(root)
}
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
func (files ProfileFiles) ReadProfile(path string) (lifecyclecontract.ProjectLifecycleProfile, error) {
	return readProjectLifecycleProfile(path)
}
func (files ProfileFiles) Mkdir(path string) error { return os.MkdirAll(path, 0o700) }
func (files ProfileFiles) Create(path string, profile lifecyclecontract.ProjectLifecycleProfile) error {
	return createJSONAtomic(path, profile, 0o600)
}
func (files ProfileFiles) Write(path string, profile lifecyclecontract.ProjectLifecycleProfile) error {
	return writeJSONAtomic(path, profile, 0o600)
}
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
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
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
		if err := tmp.Chmod(perm); err != nil {
			return err
		}
		return tmp.Close()
	}()
	if writeErr != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return writeErr
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func createJSONAtomic(path string, value any, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
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
		if err := tmp.Chmod(perm); err != nil {
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
