package augmentplan

import (
	"path/filepath"
	"testing"

	augmentcontract "issueops/internal/contract/selfaugment"
)

func TestPlanSelfAugmentationUsesGeniusThinkAndScoreGate(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	result := Plan(augmentcontract.SelfAugmentPlanRequest{Cycles: 1, TargetScore: 95}, root, "test")
	if !result.OK || result.LoopKind != "self_augmentation" || result.KoreanName != augmentcontract.SelfAugmentationKoreanName {
		t.Fatalf("unexpected loop identity: %+v", result)
	}
	if !result.UsesGeniusThink || len(result.SelectedFormulas) < 2 {
		t.Fatalf("expected GENIUS_THINK formulas: %+v", result.SelectedFormulas)
	}
	if len(result.Candidates) < 10 {
		t.Fatalf("expected candidate curriculum: %+v", result.Candidates)
	}
	if result.SelectedCandidate != nil && result.SelectedCandidate.Status != augmentcontract.CandidateStatusOpen {
		t.Fatalf("selected candidate must be an open improvement, got %+v", result.SelectedCandidate)
	}
	if candidateByID(result.Candidates, "loop-taxonomy-score-gates").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("completed taxonomy candidate should be kept for audit but skipped for selection: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "durable-augmentation-memory").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("durable memory candidate should be satisfied after state capture support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "reflexion-state-memory").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("reflexion memory candidate should be satisfied after lesson capture support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "adapter-contract-matrix").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("adapter contract matrix candidate should be satisfied after matrix golden support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "qa-race-tier").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("QA race tier candidate should be satisfied after risk-tier QA support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "repo-local-augmentation-sandbox").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("repo-local sandbox candidate should be satisfied after path boundary hardening: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "performance-baseline").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("performance baseline candidate should be satisfied after slow-step compare support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "genius-mermaid-lint").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("Mermaid lint candidate should be satisfied after QA lint support: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "install-dry-run-mode").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("install dry-run candidate should be satisfied after dry-run planning support: %+v", result.Candidates)
	}

	for _, id := range []string{"cli-mcp-adapter-split", "dto-compatibility-contract", "candidate-refill-curriculum", "policy-audit-redaction", "worker-mvp-no-shell"} {
		if candidateByID(result.Candidates, id).Status != augmentcontract.CandidateStatusSatisfied {
			t.Fatalf("%s should be satisfied after recommended implementation work: %+v", id, result.Candidates)
		}
	}
	if candidateByID(result.Candidates, "release-repro-pack").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("release reproducibility should be satisfied after the release reproducibility pack is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "release-user-readme").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("release user README should be satisfied after README install/update/rollback guide is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "cross-platform-build-matrix").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("cross-platform build matrix should be satisfied after release build matrix support is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "distribution-decision-record").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("distribution decision record should be satisfied after ADR and rollback criteria are implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "release-dogfood-notes").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("release dogfood notes should be satisfied after Codex/Claude dogfood transcripts are implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "quality-signal-harvester").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("quality signal harvester should be satisfied after quality inspect signal output is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "self-augment-signal-table").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("self-augment signal table should be satisfied after repo signal collection is table-driven: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "coverage-mcp-resources").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("MCP resource coverage should be satisfied after catalog/read edge coverage is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "coverage-host-judgement").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("external LLM coverage should be satisfied after malformed output, timeout, and command failure coverage is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "coverage-issueops-linking").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("issueops linking coverage should be satisfied after boundary coverage is implemented: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "state-write-locking").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("state write locking should be satisfied after StateWrite uses the application-owned store span: %+v", result.Candidates)
	}
	if candidateByID(result.Candidates, "worker-stuck-running-detection").Status != augmentcontract.CandidateStatusSatisfied {
		t.Fatalf("worker stuck-running detection should be satisfied after cleanup-stuck support is implemented: %+v", result.Candidates)
	}
	// Second-wave refill candidates flip to satisfied through evidence-backed
	// signal rules once their coverage lands; nothing stays open for selection.
	refill := candidateByID(result.Candidates, "coverage-issueops-inbound-adapters")
	if refill.Status != augmentcontract.CandidateStatusSatisfied || len(refill.SatisfactionEvidence) == 0 {
		t.Fatalf("inbound adapter refill should be satisfied by its coverage tests: %+v", refill)
	}
	transport := candidateByID(result.Candidates, "coverage-issueops-transport-boundaries")
	if transport.Status != augmentcontract.CandidateStatusSatisfied || len(transport.SatisfactionEvidence) == 0 {
		t.Fatalf("transport boundary refill should be satisfied by its contract tests: %+v", transport)
	}
	if result.SelectedCandidate != nil {
		t.Fatalf("expected no selected candidate after every catalog entry is satisfied, got %+v", result.SelectedCandidate)
	}
	// Goals are scored from real observables (git diff presence, saved self-verify
	// state, captured lessons), so termination eligibility must always equal the
	// conjunction of the goal results rather than a hardcoded constant.
	allPassed := true
	for _, goal := range result.Goals {
		if !goal.Passed {
			allPassed = false
		}
	}
	if result.TerminationEligible != allPassed {
		t.Fatalf("termination eligibility %v must equal all-goals-passed %v", result.TerminationEligible, allPassed)
	}
}

func candidateByID(candidates []augmentcontract.SelfAugmentCandidate, id string) augmentcontract.SelfAugmentCandidate {
	for _, candidate := range candidates {
		if candidate.ID == id {
			return candidate
		}
	}
	return augmentcontract.SelfAugmentCandidate{}
}
