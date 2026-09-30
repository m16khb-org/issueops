package selfaugment

import (
	"time"

	docscontract "issueops/internal/contract/docs"
	contract "issueops/internal/contract/selfaugment"
	statecontract "issueops/internal/contract/state"
	domain "issueops/internal/domain/selfaugment"
	verifydomain "issueops/internal/domain/selfverify"
	port "issueops/internal/port/selfaugment"
)

type Planner struct {
	Repository     port.PlanRepository
	DocsIndex      func(string, string) docscontract.DocsIndexResult
	ListSkillNames func(string) ([]string, error)
	StateList      func() (statecontract.StateListResult, error)
	StateRead      func(string) (statecontract.StateResult, error)
	Now            func() time.Time
}

func (planner Planner) Plan(req contract.SelfAugmentPlanRequest, root, version string) contract.SelfAugmentPlanResult {
	geniusPath, geniusText, err := planner.Repository.ReadGeniusThink(root)
	warnings := []string{}
	if err != nil {
		geniusText = ""
		warnings = append(warnings, "GENIUS_THINK.md not found; augmentation can still run but loses the local genius-thinking heuristic")
	}
	docs := planner.DocsIndex(root, version)
	skills, err := planner.ListSkillNames(root)
	if err != nil {
		warnings = append(warnings, "list skills: "+err.Error())
	}
	signals := planner.Repository.CollectSignals(root, len(docs.Docs), skills, geniusText)
	facts := domain.PlanFacts{GeniusText: geniusText, Signals: signals, DocsOK: docs.OK,
		ImplementationDelta: planner.Repository.HasImplementationDelta(root),
		VerificationPassed:  planner.SelfVerificationPassed(), LessonsCaptured: planner.LessonsCaptured()}
	candidates := Candidates(signals)
	lessonCounts, lessonWarnings := planner.SevereLessonCountsAt(planner.Now().UTC())
	warnings = append(warnings, lessonWarnings...)
	result := domain.NewPlan(req, facts, candidates, lessonCounts)
	result.Warnings = append(warnings, result.Warnings...)
	result.IssueOpsRoot = root
	result.GeniusThinkPath = geniusPath
	result.GeneratedAt = planner.Now().UTC().Format(time.RFC3339Nano)
	return result
}

func (planner Planner) SelfVerificationPassed() bool {
	snapshot, err := (SnapshotStore{ReadState: planner.StateRead}).Read("self-verify-latest")
	current := verifydomain.ContractValue()
	proof := snapshot.Summary.Contract
	return err == nil && snapshot.OK && snapshot.Summary.TerminationEligible && proof.Name == current.Name && proof.Version == current.Version && proof.Hash == current.Hash
}
