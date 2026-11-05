// Package doctor 패키지는 harness doctor 점검의 DTO를 소유한다. 요청, 결과,
// 점검 항목, 이슈, 수정 안내가 여기에 있다.
package doctor

import (
	lifecyclecontract "issueops/internal/contract/lifecycle"
	operationalhealthcontract "issueops/internal/contract/operationalhealth"
)

// ProjectLifecycleStatePlan 타입은 lifecycle 계획 타입의 별칭이다. domain/doctor는
// contract/doctor만 import할 수 있어서 여기서 이 이름으로 둔다.
type ProjectLifecycleStatePlan = lifecyclecontract.ProjectLifecycleStatePlan

// HarnessDoctorRequest 타입은 harness doctor 실행 입력이다. json 태그가 없는
// 필드(StaticOnly, OperationalSnapshot, OperationalOptions)는 호출자가 직접 채우며
// 직렬화되지 않는다.
type HarnessDoctorRequest struct {
	RepoRoot            string                              `json:"repo_root,omitempty"`
	IssueOpsRoot        string                              `json:"issueops_root,omitempty"`
	Home                string                              `json:"home,omitempty"`
	Version             string                              `json:"version,omitempty"`
	StaticOnly          bool                                `json:"-"`
	OperationalSnapshot *operationalhealthcontract.Snapshot `json:"-"`
	OperationalOptions  operationalhealthcontract.Options   `json:"-"`
}

// HarnessDoctorResult 타입은 harness doctor 점검 결과다. Healthy는 Issues가 모두
// 해소됐는지를 나타내고, Checks는 항목별 판정을 담는다.
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

// HarnessDoctorCheck 타입은 doctor가 수행한 개별 점검 하나의 이름, 통과 여부, 요약이다.
type HarnessDoctorCheck struct {
	Name    string `json:"name"`
	Healthy bool   `json:"healthy"`
	Summary string `json:"summary"`
}

// HarnessDoctorIssue 타입은 doctor가 찾은 문제 하나다. Code로 종류를, Severity로
// 심각도를 구분하고, 알려진 수정 방법이 있으면 Fix에 담는다.
type HarnessDoctorIssue struct {
	Code     string            `json:"code"`
	Severity string            `json:"severity"`
	Summary  string            `json:"summary"`
	Path     string            `json:"path,omitempty"`
	Fix      *HarnessDoctorFix `json:"fix,omitempty"`
}

// HarnessDoctorFix 타입은 문제를 고치는 방법이다. 실행할 수 있는 명령이 있으면
// Command에, 사람이 읽을 설명은 Description에 담는다.
type HarnessDoctorFix struct {
	Command     string `json:"command,omitempty"`
	Description string `json:"description"`
}
