package lifecycle

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	lifecyclecontract "issueops/internal/contract/lifecycle"
)

func RepoID(fingerprint lifecyclecontract.ProjectFingerprint) string {
	parts := []string{fingerprint.RepoRoot, fingerprint.GitDir, fingerprint.GitOriginHash}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])[:24]
}

func EqualFingerprint(a, b lifecyclecontract.ProjectFingerprint) bool {
	return a.RepoRoot == b.RepoRoot && a.GitDir == b.GitDir && a.GitOriginHash == b.GitOriginHash
}

func ResolveProfile(plan lifecyclecontract.ProjectLifecycleStatePlan, profile lifecyclecontract.ProjectLifecycleProfile, schemaVersion int) lifecyclecontract.ProjectLifecycleStatePlan {
	plan.Exists = true
	plan.Profile = &profile
	plan.NamespaceValid = EqualFingerprint(profile.Fingerprint, plan.Fingerprint) && profile.RepoID == plan.RepoID && profile.SchemaVersion == schemaVersion
	if !plan.NamespaceValid {
		plan.Warnings = append(plan.Warnings, "namespace_mismatch")
	}
	return plan
}

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
