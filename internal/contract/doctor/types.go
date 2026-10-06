package doctor

import (
	lifecyclecontract "issueops/internal/contract/lifecycle"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
)

// domain/doctor는 contract/doctor만 import할 수 있어서 lifecycle 계획 타입을 여기서
// 이 이름으로 둔다.
type ProjectLifecycleStatePlan = lifecyclecontract.ProjectLifecycleStatePlan

type HarnessDoctorRequest struct {
	RepoRoot            string                              `json:"repo_root,omitempty"`
	IssueOpsRoot        string                              `json:"issueops_root,omitempty"`
	Home                string                              `json:"home,omitempty"`
	Version             string                              `json:"version,omitempty"`
	StaticOnly          bool                                `json:"-"`
	OperationalSnapshot *operationalhealthcontract.Snapshot `json:"-"`
	OperationalOptions  operationalhealthcontract.Options   `json:"-"`
}

type HarnessDoctorResult struct {
	OK                bool                      `json:"ok"`
	Healthy           bool                      `json:"healthy"`
	Kind              string                    `json:"kind"`
	Version           string                    `json:"version,omitempty"`
	IssueOpsRoot      string                    `json:"issueops_root,omitempty"`
	RepoRoot          string                    `json:"repo_root"`
	StateDir          string                    `json:"state_dir"`
	LifecycleState    ProjectLifecycleStatePlan `json:"lifecycle_state"`
	PipeCapacityBytes int                       `json:"pipe_capacity_bytes"`
	Checks            []HarnessDoctorCheck      `json:"checks"`
	Issues            []HarnessDoctorIssue      `json:"issues"`
	GeneratedAt       string                    `json:"generated_at"`
}

type HarnessDoctorCheck struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Summary string `json:"summary"`
}

type HarnessDoctorIssue struct {
	Code     string            `json:"code"`
	Severity string            `json:"severity"`
	Summary  string            `json:"summary"`
	Path     string            `json:"path,omitempty"`
	Fix      *HarnessDoctorFix `json:"fix,omitempty"`
}

type HarnessDoctorFix struct {
	Command     string `json:"command,omitempty"`
	Destructive bool   `json:"destructive"`
	Description string `json:"description"`
}
