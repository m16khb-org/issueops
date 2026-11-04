// Package lifecycle implements project lifecycle state resolution and
// initialization through injected effects, delegating profile rules to the
// lifecycle domain. It sits in the application layer.
package lifecycle

import (
	"errors"
	"io/fs"
	"time"

	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectdoccontract "issueops/internal/contract/projectdoc"
	lifecycledomain "issueops/internal/domain/lifecycle"
)

type effects interface {
	NormalizeRoot(string) (string, error)
	StateRoot() string
	Fingerprint(string) lifecyclecontract.ProjectFingerprint
	Paths(string, lifecyclecontract.ProjectFingerprint) lifecyclecontract.ProjectLifecycleStatePlan
	ReadProfile(string) (lifecyclecontract.ProjectLifecycleProfile, error)
	Mkdir(string) error
	Create(string, lifecyclecontract.ProjectLifecycleProfile) error
	Write(string, lifecyclecontract.ProjectLifecycleProfile) error
	Now() time.Time
}

// Service resolves and initializes the per-project lifecycle state at the
// SchemaVersion it carries.
type Service struct {
	Effects       effects
	SchemaVersion int
}

// Resolve computes the lifecycle state plan for repoRoot without writing. A
// missing project.json yields the plan as is, and an unreadable one adds the
// project_json_read_error warning; only root normalization failure returns an error.
func (service Service) Resolve(repoRoot string) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
	root, err := service.Effects.NormalizeRoot(repoRoot)
	if err != nil {
		return lifecyclecontract.ProjectLifecycleStatePlan{OK: false, StateRoot: service.Effects.StateRoot(), SchemaVersion: service.SchemaVersion}, err
	}
	fingerprint := service.Effects.Fingerprint(root)
	plan := service.Effects.Paths(root, fingerprint)
	profile, err := service.Effects.ReadProfile(plan.ProjectJSONPath)
	if errors.Is(err, fs.ErrNotExist) {
		return plan, nil
	}
	if err != nil {
		plan.Warnings = append(plan.Warnings, "project_json_read_error")
		return plan, nil
	}
	return lifecycledomain.ResolveProfile(plan, profile, service.SchemaVersion), nil
}

// Init resolves the plan and, when confirm is true, creates or rewrites
// project.json with the optional first metadata profile. It does nothing
// without confirm or when an existing profile has an invalid namespace. If a
// concurrent session created the profile first, that profile stays authoritative.
func (service Service) Init(repoRoot string, confirm bool, metadata ...projectdoccontract.ProjectProfile) (lifecyclecontract.ProjectLifecycleStatePlan, error) {
	plan, err := service.Resolve(repoRoot)
	if err != nil || !confirm || (plan.Exists && !plan.NamespaceValid) {
		return plan, err
	}
	if err := service.Effects.Mkdir(plan.ProjectStateDir); err != nil {
		plan.OK = false
		return plan, err
	}
	now := service.Effects.Now().UTC().Format(time.RFC3339Nano)
	var meta *projectdoccontract.ProjectProfile
	if len(metadata) > 0 {
		m := metadata[0]
		meta = &m
	}
	profile := lifecycledomain.NewProfile(plan, lifecyclecontract.ProjectLifecycleProfile{Metadata: meta}, now, service.SchemaVersion)
	if !plan.Exists {
		if err := service.Effects.Create(plan.ProjectJSONPath, profile); err == nil {
			plan.Exists = true
			plan.NamespaceValid = true
			plan.Profile = &profile
			return plan, nil
		} else if !errors.Is(err, fs.ErrExist) {
			plan.OK = false
			return plan, err
		}
		// Another session created the profile. Its namespace remains authoritative.
		existing, err := service.Effects.ReadProfile(plan.ProjectJSONPath)
		if err != nil {
			plan.OK = false
			return plan, err
		}
		return lifecycledomain.ResolveProfile(plan, existing, service.SchemaVersion), nil
	}
	if err := service.Effects.Write(plan.ProjectJSONPath, profile); err != nil {
		plan.OK = false
		return plan, err
	}
	plan.Exists = true
	plan.NamespaceValid = true
	plan.Profile = &profile
	return plan, nil
}
