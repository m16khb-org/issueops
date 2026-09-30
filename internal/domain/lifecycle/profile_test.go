package lifecycle

import (
	"testing"

	lifecyclecontract "issueops/internal/contract/lifecycle"
)

func TestResolveProfileRejectsNamespaceAndPreservesMetadata(t *testing.T) {
	fingerprint := lifecyclecontract.ProjectFingerprint{RepoRoot: "/repo"}
	plan := lifecyclecontract.ProjectLifecycleStatePlan{RepoID: "repo-id", Fingerprint: fingerprint}
	profile := lifecyclecontract.ProjectLifecycleProfile{SchemaVersion: 1, RepoID: "wrong", Fingerprint: fingerprint}
	resolved := ResolveProfile(plan, profile, 1)
	if !resolved.Exists || resolved.NamespaceValid || len(resolved.Warnings) != 1 || resolved.Warnings[0] != "namespace_mismatch" {
		t.Fatalf("unexpected mismatch result: %+v", resolved)
	}
	profile.RepoID = "repo-id"
	resolved = ResolveProfile(plan, profile, 1)
	if !resolved.NamespaceValid || len(resolved.Warnings) != 0 {
		t.Fatalf("valid namespace rejected: %+v", resolved)
	}
}

func TestNewProfileKeepsCreationTimeAndExistingMetadata(t *testing.T) {
	plan := lifecyclecontract.ProjectLifecycleStatePlan{RepoID: "repo-id", Fingerprint: lifecyclecontract.ProjectFingerprint{RepoRoot: "/repo"}, Profile: &lifecyclecontract.ProjectLifecycleProfile{CreatedAt: "created"}}
	profile := NewProfile(plan, lifecyclecontract.ProjectLifecycleProfile{}, "updated", 1)
	if profile.CreatedAt != "created" || profile.UpdatedAt != "updated" || profile.RepoID != "repo-id" {
		t.Fatalf("profile transition: %+v", profile)
	}
}
