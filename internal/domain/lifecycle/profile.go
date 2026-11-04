// Package lifecycle holds the pure rules for the project lifecycle profile:
// repository identity and namespace validation. It is a domain package.
package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	lifecyclecontract "issueops/internal/contract/lifecycle"
)

// RepoID derives the stable 24-hex-character repository ID from the
// fingerprint's repo root, git dir, and origin hash.
func RepoID(fingerprint lifecyclecontract.ProjectFingerprint) string {
	parts := []string{fingerprint.RepoRoot, fingerprint.GitDir, fingerprint.GitOriginHash}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:24]
}

// EqualFingerprint reports whether a and b identify the same repository: same
// repo root, git dir, and origin hash.
func EqualFingerprint(a, b lifecyclecontract.ProjectFingerprint) bool {
	return a.RepoRoot == b.RepoRoot && a.GitDir == b.GitDir && a.GitOriginHash == b.GitOriginHash
}

// ResolveProfile marks plan as having a stored profile and sets NamespaceValid
// when the profile's fingerprint, repo ID, and schemaVersion all match the
// plan. A mismatch adds the namespace_mismatch warning.
func ResolveProfile(plan lifecyclecontract.ProjectLifecycleStatePlan, profile lifecyclecontract.ProjectLifecycleProfile, schemaVersion int) lifecyclecontract.ProjectLifecycleStatePlan {
	plan.Exists = true
	plan.Profile = &profile
	plan.NamespaceValid = EqualFingerprint(profile.Fingerprint, plan.Fingerprint) && profile.RepoID == plan.RepoID && profile.SchemaVersion == schemaVersion
	if !plan.NamespaceValid {
		plan.Warnings = append(plan.Warnings, "namespace_mismatch")
	}
	return plan
}

// NewProfile builds the profile to persist for plan at time now. It keeps the
// existing profile's CreatedAt, and its Metadata when candidate has none.
func NewProfile(plan lifecyclecontract.ProjectLifecycleStatePlan, candidate lifecyclecontract.ProjectLifecycleProfile, now string, schemaVersion int) lifecyclecontract.ProjectLifecycleProfile {
	createdAt := now
	if plan.Profile != nil && plan.Profile.CreatedAt != "" {
		createdAt = plan.Profile.CreatedAt
	}
	if candidate.Metadata == nil && plan.Profile != nil {
		candidate.Metadata = plan.Profile.Metadata
	}
	return lifecyclecontract.ProjectLifecycleProfile{
		SchemaVersion: schemaVersion,
		RepoID:        plan.RepoID,
		Fingerprint:   plan.Fingerprint,
		Metadata:      candidate.Metadata,
		CreatedAt:     createdAt,
		UpdatedAt:     now,
	}
}
