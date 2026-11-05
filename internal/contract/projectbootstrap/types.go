// Package projectbootstrap holds the request and result contract DTOs of the
// project docs bootstrap.
package projectbootstrap

import (
	lifecyclecontract "issueops/internal/contract/lifecycle"
	projectdoccontract "issueops/internal/contract/projectdoc"
)

// ProjectDocsBootstrapRequest selects the repository to bootstrap. Write
// applies the plan to disk (otherwise it is a dry run) and Sync refreshes
// existing documents.
type ProjectDocsBootstrapRequest struct {
	RepoRoot string `json:"repo_root"`
	Write    bool   `json:"write"`
	Sync     bool   `json:"sync"`
}

// ProjectDocsBootstrapResult reports the planned project docs files, detected
// project signals and lifecycle state plan of one bootstrap run. DryRun is true
// when nothing was written, and Warnings lists non-fatal problems.
type ProjectDocsBootstrapResult struct {
	OK             bool                                        `json:"ok"`
	Kind           string                                      `json:"kind"`
	RepoRoot       string                                      `json:"repo_root"`
	DocsDir        string                                      `json:"docs_dir"`
	Write          bool                                        `json:"write"`
	Sync           bool                                        `json:"sync"`
	DryRun         bool                                        `json:"dry_run"`
	GeneratedAt    string                                      `json:"generated_at"`
	Signals        projectdoccontract.ProjectSignals           `json:"signals"`
	Files          []projectdoccontract.ProjectDocsPlannedFile `json:"files"`
	LifecycleState lifecyclecontract.ProjectLifecycleStatePlan `json:"lifecycle_state"`
	Warnings       []string                                    `json:"warnings,omitempty"`
}
