// Package policy 패키지는 명령 정책의 애플리케이션 계층이다. 요청 관찰, 재정의 읽기, 도메인 규칙 평가를 이어 붙이고 허용된 명령만 실행기에 넘긴다. 규칙 자체는 domain/policy가 갖고 파일 시스템과 프로세스 실행은 포트 뒤에 있다.
package policy

import (
	policycontract "issueops/internal/contract/policy"
	policydomain "issueops/internal/domain/policy"
)

// Observation 타입은 Observer가 요청 한 건에서 관찰한 결과다. 정규화된 작업 루트와 CWD, 감사 로그 ID, 생성 시각, 정책 판단에 쓰는 명령 사실을 담는다.
type Observation struct {
	Root        string
	CWD         string
	AuditLogID  string
	GeneratedAt string
	Facts       policydomain.CommandFacts
}

// Observer 인터페이스는 파일 시스템과 git을 직접 만지는 쪽이 구현하며, 정책 평가에 필요한 관찰값을 만든다.
type Observer interface {
	Observe(policycontract.CommandPolicyRequest) Observation
}

// OverrideSnapshot 타입은 저장소에서 읽은 정책 재정의 값이다. 읽기에 문제가 있으면 Warning에 사유가 담기고 Values는 nil일 수 있다.
type OverrideSnapshot struct {
	Values  *policycontract.Overrides
	Warning string
}

type overrideLoader interface {
	Load(root string) OverrideSnapshot
}

// Service 타입은 명령 정책 평가와 실행을 맡는다. 관찰, 재정의 읽기, 실행, 시계는 주입받은 구현에 맡기고 판단 규칙은 도메인에 둔다.
type Service struct {
	PreparedBaseBranch PreparedBaseBranchLookup
	Observer           Observer
	Overrides          overrideLoader
	Executor           executor
	Clock              clock
}

// Evaluate 메서드는 요청을 관찰하고 내장 규칙 목록에 재정의를 적용해 허용 여부를 판단한다. 명령 실행이나 파일 쓰기는 하지 않으며, 인자와 셸 사유는 가려서 결과에 담는다.
func (service Service) Evaluate(request policycontract.CommandPolicyRequest) policycontract.CommandPolicyEvaluation {
	observation := service.Observer.Observe(request)
	facts := observation.Facts
	facts.Timeout, facts.TimeoutValid = policydomain.CommandTimeout(request.Timeout)
	facts.PRTargetDeny, facts.PRTargetExpected = pullRequestTargetDeny(observation.Root, observation.CWD, request.Argv, service.PreparedBaseBranch)
	catalog := policydomain.BuiltinCatalog()
	if service.Overrides != nil {
		overrides := service.Overrides.Load(observation.Root)
		if overrides.Values != nil {
			catalog.Apply(*overrides.Values)
		}
		if overrides.Warning != "" {
			facts.Warnings = append(facts.Warnings, overrides.Warning)
		}
	}
	if len(request.Argv) > 0 {
		classification := catalog.Classify(request.Argv)
		facts.ShellCommand = classification.ShellCommand
		facts.UsesNetwork = classification.UsesNetwork
		facts.Writes = classification.Writes
		facts.ReadOnlyAllowed = classification.ReadOnlyAllowed
	}
	decision := policydomain.EvaluateCommandDecision(request, facts)
	return policycontract.CommandPolicyEvaluation{
		OK: true, Allowed: decision.Allowed, AuditLogID: observation.AuditLogID,
		WorkspaceRoot: observation.Root, CWD: observation.CWD,
		Argv: policydomain.RedactArgv(request.Argv), Timeout: facts.Timeout.String(),
		EnvAllowlist:   policydomain.CleanEnvAllowlist(request.EnvAllowlist),
		NetworkAllowed: request.NetworkAllowed, WriteAllowed: request.WriteAllowed,
		ShellAllowed: request.ShellAllowed, ShellReason: policydomain.RedactFreeform(request.ShellReason),
		Tier: policydomain.ResolveTier(policycontract.Request{
			WriteAllowed: request.WriteAllowed, NetworkAllowed: request.NetworkAllowed, ShellAllowed: request.ShellAllowed,
		}),
		DenyReasons: decision.DenyReasons, Warnings: decision.Warnings, GeneratedAt: observation.GeneratedAt,
	}
}
