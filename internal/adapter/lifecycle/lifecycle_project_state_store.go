package lifecycle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"issueops/internal/adapter/lifecycle/fingerprint"
	lifecycleapp "issueops/internal/application/lifecycle"
	lifecyclecontract "issueops/internal/contract/lifecycle"
	lifecycledomain "issueops/internal/domain/lifecycle"
)

func lifecycleService() lifecycleapp.Service {
	return lifecycleapp.Service{Effects: lifecycleEffects{}, SchemaVersion: ProjectLifecycleSchemaVersion}
}

func ResolveProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return lifecycleService().Resolve(repoRoot)
}

func InitProjectLifecycleState(repoRoot string, confirm bool, metadata ...ProjectProfile) (ProjectLifecycleStatePlan, error) {
	return lifecycleService().Init(repoRoot, confirm, metadata...)
}

func ValidateProjectLifecycleState(repoRoot string) (ProjectLifecycleStatePlan, error) {
	return ResolveProjectLifecycleState(repoRoot)
}

type lifecycleEffects struct{}

func (lifecycleEffects) NormalizeRoot(root string) (string, error) { return NormalizeRepoRoot(root) }
func (lifecycleEffects) StateRoot() string                         { return StateDir() }
func (lifecycleEffects) Fingerprint(root string) lifecyclecontract.ProjectFingerprint {
	return fingerprint.ForRoot(root)
}
func (lifecycleEffects) Paths(root string, projectFingerprint lifecyclecontract.ProjectFingerprint) lifecyclecontract.ProjectLifecycleStatePlan {
	repoID := lifecycledomain.RepoID(projectFingerprint)
	stateRoot := StateDir()
	projectDir := filepath.Join(stateRoot, "projects", repoID)
	return ProjectLifecycleStatePlan{
		OK:              true,
		SchemaVersion:   ProjectLifecycleSchemaVersion,
		RepoRoot:        root,
		RepoID:          repoID,
		StateRoot:       stateRoot,
		ProjectStateDir: projectDir,
		ProjectJSONPath: filepath.Join(projectDir, projectLifecycleProfileFile),
		QueuePath:       filepath.Join(projectDir, docUpkeepQueueFile),
		CompactPath:     filepath.Join(projectDir, compactCapsuleFile),
		Fingerprint:     projectFingerprint,
		Warnings:        []string{},
	}
}
func (lifecycleEffects) ReadProfile(path string) (ProjectLifecycleProfile, error) {
	return readProjectLifecycleProfile(path)
}
func (lifecycleEffects) Mkdir(path string) error { return os.MkdirAll(path, 0o700) }
func (lifecycleEffects) Create(path string, profile ProjectLifecycleProfile) error {
	return createJSONAtomic(path, profile, 0o600)
}
func (lifecycleEffects) Write(path string, profile ProjectLifecycleProfile) error {
	return writeJSONAtomic(path, profile, 0o600)
}
func (lifecycleEffects) Now() time.Time { return time.Now() }

func readProjectLifecycleProfile(path string) (ProjectLifecycleProfile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return ProjectLifecycleProfile{}, err
	}
	var profile ProjectLifecycleProfile
	if err := json.Unmarshal(b, &profile); err != nil {
		return ProjectLifecycleProfile{}, err
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
