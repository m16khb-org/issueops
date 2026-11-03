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
	var collected lintCollection
	for _, spec := range req.Gates {
		collected.candidates = append(collected.candidates, domain.LintSpecCandidate("spec", spec))
	}
	if plan := strings.TrimSpace(req.PlanFile); plan != "" {
		data, err := service.Store.Read(plan)
		if err != nil {
			return model.LintResult{SchemaVersion: model.SchemaVersion}, err
		}
		collected.addPlan(plan, string(data))
	}
	for _, file := range req.Files {
		data, err := service.Store.Read(file)
		if err != nil {
			return model.LintResult{SchemaVersion: model.SchemaVersion}, err
		}
		collected.addLedger(file, string(data))
	}
	results := service.lintCandidates(check, collected.candidates)
	return domain.SummarizeLint(results, collected.unparsedLines, collected.warnings), nil
}

// lintCollection는 Lint가 읽은 입력에서 모은 후보와 경고를 입력 순서대로 쌓는다.
type lintCollection struct {
	candidates    []domain.LintCandidate
	warnings      []string
	unparsedLines []string
}

func (c *lintCollection) addPlan(plan, data string) {
	found, unparsed := domain.PlanLintCandidates(plan, data)
	c.candidates = append(c.candidates, found...)
	if len(unparsed) > 0 {
		c.warnings = append(c.warnings, domain.UnparsedWarning(plan, unparsed))
		c.addUnparsedLines(plan, unparsed)
	}
	if len(found) == 0 {
		c.warnings = append(c.warnings, plan+": no gates found; a plan without linted gates proves nothing")
	}
}

func (c *lintCollection) addLedger(file, data string) {
	found := domain.LedgerLintCandidates(file, data)
	c.candidates = append(c.candidates, found...)
	c.warnings = append(c.warnings, domain.LedgerFormatWarnings(file, domain.Parse(data))...)
	if len(found) == 0 {
		// QA: a hand-written ledger without the checkbox form linted clean
		// with zero gates, and check would then have nothing to run.
		c.warnings = append(c.warnings, file+": no ledger gates found; a ledger gate is \"- [ ] G1: outcome\" with indented CHECK:/EXPECT: lines (gates init writes this form)")
		_, unparsed := domain.PlanLintCandidates(file, data)
		c.addUnparsedLines(file, unparsed)
	}
}

func (c *lintCollection) addUnparsedLines(source string, lines []int) {
	for _, line := range lines {
		c.unparsedLines = append(c.unparsedLines, source+":"+strconv.Itoa(line))
	}
}

func (service Service) lintCandidates(check model.CheckRequest, candidates []domain.LintCandidate) []model.LintGate {
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
	return results
}

// PolicyEvaluate는 실행 없이 command policy 판정만 하는 함수다.
type PolicyEvaluate func(policy.CommandPolicyRequest) policy.CommandPolicyEvaluation
