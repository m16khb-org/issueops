// Package selfaugment is the application layer of the self-augmentation loop: it
// builds improvement plans from repository signals and recorded lessons, saves
// plan, lesson and baseline checkpoints, and reads, compares and prunes the
// self-verification history, all through injected state functions.
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

// Planner builds self-augmentation plans. Its fields are the injected plan
// repository, docs index, skill lister, state accessors and clock.
type Planner struct {
	Repository     port.PlanRepository
	DocsIndex      func(string, string) docscontract.IndexResult
	ListSkillNames func(string) ([]string, error)
	StateList      func() (statecontract.ListResult, error)
	StateRead      func(string) (statecontract.Result, error)
	Now            func() time.Time
}

// Plan builds the self-augmentation plan for the repository at root. Missing
// GENIUS_THINK.md or an unreadable skill list become warnings, not errors. The
// candidates are weighted by the severe lessons still active, and the plan is
// stamped with the root, the genius-think path and the generation time (UTC).
func (planner Planner) Plan(req contract.PlanRequest, root, version string) contract.PlanResult {
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

// SelfVerificationPassed reports whether the latest saved self-verification
// snapshot succeeded, is eligible for termination, and was produced under the
// current verification contract (same name, version and hash).
func (planner Planner) SelfVerificationPassed() bool {
	snapshot, err := (SnapshotStore{ReadState: planner.StateRead}).Read("self-verify-latest")
	current := verifydomain.ContractValue()
	proof := snapshot.Summary.Contract
	return err == nil && snapshot.OK && snapshot.Summary.TerminationEligible && proof.Name == current.Name && proof.Version == current.Version && proof.Hash == current.Hash
}
