package gates

import (
	"strconv"
	"strings"

	model "issueops/internal/contract/gates"
	policy "issueops/internal/contract/policy"
	domain "issueops/internal/domain/gates"
)

// Lint는 게이트를 실행하지 않고 gates init·check가 거부하거나 met으로 만들 수 없는
// 형식을 찾는다. CHECK는 gates check와 같은 정책 요청으로 평가만 한다.
func (service Service) Lint(req model.LintRequest) (model.LintResult, error) {
	if len(req.Gates) == 0 && strings.TrimSpace(req.PlanFile) == "" && len(req.Files) == 0 {
		return model.LintResult{SchemaVersion: model.SchemaVersion}, model.ErrNothingToLint
	}
	check := domain.NormalizeCheck(model.CheckRequest{
		WorkspaceRoot: req.WorkspaceRoot, CWD: req.CWD, TimeoutSeconds: req.TimeoutSeconds,
		EnvAllowlist: req.EnvAllowlist, WriteAllowed: req.WriteAllowed, NetworkAllowed: req.NetworkAllowed,
	})
	var candidates []domain.LintCandidate
	var warnings, unparsedLines []string
	for _, spec := range req.Gates {
		candidates = append(candidates, domain.LintSpecCandidate("spec", spec))
	}
	if plan := strings.TrimSpace(req.PlanFile); plan != "" {
		data, err := service.Store.Read(plan)
		if err != nil {
			return model.LintResult{SchemaVersion: model.SchemaVersion}, err
		}
		found, unparsed := domain.PlanLintCandidates(plan, string(data))
		candidates = append(candidates, found...)
		if len(unparsed) > 0 {
			warnings = append(warnings, domain.UnparsedWarning(plan, unparsed))
			for _, line := range unparsed {
				unparsedLines = append(unparsedLines, plan+":"+strconv.Itoa(line))
			}
		}
		if len(found) == 0 {
			warnings = append(warnings, plan+": no gates found; a plan without linted gates proves nothing")
		}
	}
	for _, file := range req.Files {
		data, err := service.Store.Read(file)
		if err != nil {
			return model.LintResult{SchemaVersion: model.SchemaVersion}, err
		}
		candidates = append(candidates, domain.LedgerLintCandidates(file, string(data))...)
		warnings = append(warnings, domain.LedgerFormatWarnings(file, domain.Parse(string(data)))...)
	}
	results := make([]model.LintGate, 0, len(candidates))
	for _, candidate := range candidates {
		result, argv := domain.LintStatic(candidate)
		if argv != nil && service.Evaluate != nil {
			if evaluation := service.Evaluate(checkPolicyRequest(check.WorkspaceRoot, check.CWD, check, argv)); !evaluation.Allowed {
				result = domain.DeniedLint(result, evaluation.DenyReasons)
			}
		}
		results = append(results, result)
	}
	return domain.SummarizeLint(results, unparsedLines, warnings), nil
}

// PolicyEvaluate는 실행 없이 command policy 판정만 하는 함수다.
type PolicyEvaluate func(policy.CommandPolicyRequest) policy.CommandPolicyEvaluation
