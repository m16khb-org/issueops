package qualitycatalog

import (
	contract "issueops/internal/contract/qualitycatalog"
)

const CandidateStatusOpen = "open"

var resolvedCandidateIDs = map[string]bool{
	"daemon-connection-limit":        true,
	"worker-stuck-running-detection": true,
	"state-write-locking":            true,
	// Resolved wave: signal rules in augmentcatalog confirm satisfaction and the
	// measured coverage backs it (mcpcli/resources 91.7%, domain/judgement 90.6%,
	// linking suites exist under their current package paths).
	"quality-signal-harvester":  true,
	"self-augment-signal-table": true,
	"coverage-mcp-resources":    true,
	"coverage-host-judgement":   true,
	"coverage-issueops-linking": true,
	// Second-wave refill completed: inbound adapter and transport boundary
	// coverage landed (augmentcatalog signal rules confirm with evidence).
	"coverage-issueops-inbound-adapters":     true,
	"coverage-issueops-transport-boundaries": true,
}

type VerificationKind = contract.VerificationKind

const (
	ToolSignalKind  = contract.ToolSignalKind
	DocArtifactKind = contract.DocArtifactKind
)

type CandidateSpec struct {
	ID               string
	Title            string
	Category         string
	VerificationKind VerificationKind
	Impact           float64
	Feasibility      float64
	Novelty          float64
	Risk             float64
	WhyNow           []string
	ExpectedGain     []string
	VerifyWith       []string
	Evidence         []string
}

type Candidate = contract.Candidate

func CandidateSpecs() []CandidateSpec {
	specs := []CandidateSpec{
		{
			ID: "quality-signal-harvester", Title: "Add quality inspect signal harvesting CLI", Category: "quality",
			Impact: 94, Feasibility: 92, Novelty: 78, Risk: 14,
			WhyNow:       []string{"테스트 통과 여부보다 다음 결함 후보를 좁히는 측정 표면이 필요하다"},
			ExpectedGain: []string{"coverage, branch complexity, audit risk를 한 JSON 계약으로 수집", "self-augment 후보 refill의 입력을 재현 가능하게 만든다"},
			VerifyWith:   []string{"go test ./cmd/issueops/qualitycli -count=1", "issueops quality inspect --json"},
			Evidence:     []string{"quality inspect CLI", "coverage/complexity/audit signal output"},
		},
		{
			ID: "self-augment-signal-table", Title: "Table-drive self-augment repository signal collection", Category: "refactor",
			Impact: 88, Feasibility: 90, Novelty: 64, Risk: 12,
			WhyNow:       []string{"CollectSelfAugmentRepoSignals branch count is high for a read-only detector"},
			ExpectedGain: []string{"signal additions become table rows", "branch count drops below the planned threshold"},
			VerifyWith:   []string{"go test ./cmd/issueops/selfworkflow/augmentcatalog -count=1"},
			Evidence:     []string{"CollectSelfAugmentRepoSignals table-driven refactor"},
		},
		{
			ID: "coverage-mcp-resources", Title: "Cover MCP resource catalog edge cases", Category: "coverage",
			Impact: 80, Feasibility: 84, Novelty: 50, Risk: 12,
			WhyNow:       []string{"MCP resource drift affects both Codex and Claude hosts"},
			ExpectedGain: []string{"resource schema and lookup regressions fail in package tests"},
			VerifyWith:   []string{"go test ./cmd/issueops/mcpcli/resources -count=1", "go test -cover ./cmd/issueops/mcpcli/resources"},
			Evidence:     []string{"go test -cover low package signal"},
		},
		{
			ID: "coverage-host-judgement", Title: "Cover host-agent judgement malformed output paths", Category: "coverage",
			Impact: 82, Feasibility: 78, Novelty: 56, Risk: 18,
			WhyNow:       []string{"IssueOps and quality gates depend on strict host-agent result file decoding"},
			ExpectedGain: []string{"malformed JSON and bounded error output paths stay deterministic"},
			VerifyWith:   []string{"go test ./internal/domain/judgement -count=1", "go test -cover ./internal/domain/judgement"},
			Evidence:     []string{"go test -cover low package signal"},
		},
		{
			ID: "coverage-issueops-linking", Title: "Cover IssueOps link issue and link plan boundaries", Category: "coverage",
			Impact: 84, Feasibility: 82, Novelty: 54, Risk: 16,
			WhyNow:       []string{"IssueOps durable state gates are easy to regress with boundary paths"},
			ExpectedGain: []string{"invalid URLs, missing files, and path boundaries are pinned"},
			VerifyWith:   []string{"go test ./internal/application/issueopslease ./internal/application/issueopspreparation -count=1", "go test -cover ./internal/application/issueopsbranch"},
			Evidence:     []string{"go test -cover low package signal"},
		},
		{
			ID: "coverage-issueops-inbound-adapters", Title: "Cover IssueOps inbound adapter request mapping", Category: "coverage",
			Impact: 86, Feasibility: 88, Novelty: 48, Risk: 14,
			WhyNow:       []string{"issueopsdecision, issueopsinventory, issueopsretention, issueopsrouting, issueopsstatus inbound adapters measure 0% coverage while CLI responses depend on their mapping"},
			ExpectedGain: []string{"request-to-usecase delegation regressions surface in package tests"},
			VerifyWith:   []string{"go test ./internal/adapter/inbound/issueopsdecision ./internal/adapter/inbound/issueopsinventory ./internal/adapter/inbound/issueopsretention ./internal/adapter/inbound/issueopsrouting ./internal/adapter/inbound/issueopsstatus -count=1", "go test -cover ./internal/adapter/inbound/..."},
			Evidence:     []string{"quality inspect low-coverage evidence"},
		},
		{
			ID: "coverage-issueops-transport-boundaries", Title: "Cover tool conformance and IssueOps transport contracts", Category: "coverage",
			Impact: 82, Feasibility: 86, Novelty: 46, Risk: 12,
			WhyNow:       []string{"contract/toolconformance measures 0% and contract/issueops sits near the threshold while both pin response fields shared by CLI and MCP"},
			ExpectedGain: []string{"response field drift fails at contract tests before golden updates"},
			VerifyWith:   []string{"go test ./internal/contract/toolconformance ./internal/contract/issueops -count=1", "go test -cover ./internal/contract/toolconformance ./internal/contract/issueops"},
			Evidence:     []string{"quality inspect low-coverage evidence"},
		},
		{
			ID: "daemon-connection-limit", Title: "Add daemon connection limit protection", Category: "audit-risk",
			Impact: 90, Feasibility: 72, Novelty: 58, Risk: 26,
			WhyNow:       []string{".issueops/PROJECT_AUDIT.md flags D1 P1 no connection limit"},
			ExpectedGain: []string{"daemon resource exhaustion has an explicit guard and test"},
			VerifyWith:   []string{"go test ./cmd/issueops/daemoncli ./internal/adapter/worker -count=1"},
			Evidence:     []string{"PROJECT_AUDIT D1 P1"},
		},
		{
			ID: "worker-stuck-running-detection", Title: "Detect worker jobs stuck running after process crash", Category: "audit-risk",
			Impact: 88, Feasibility: 74, Novelty: 58, Risk: 24,
			WhyNow:       []string{".issueops/PROJECT_AUDIT.md flags W1 P1 stuck running jobs"},
			ExpectedGain: []string{"worker status can classify stale running records"},
			VerifyWith:   []string{"go test ./internal/adapter/worker ./cmd/issueops/workercli -count=1"},
			Evidence:     []string{"PROJECT_AUDIT W1 P1"},
		},
		{
			ID: "state-write-locking", Title: "Add write locking around state file updates", Category: "audit-risk",
			Impact: 91, Feasibility: 76, Novelty: 56, Risk: 28,
			WhyNow:       []string{".issueops/PROJECT_AUDIT.md flags S1 P1 no write locking"},
			ExpectedGain: []string{"concurrent state writes stop risking lost updates"},
			VerifyWith:   []string{"go test ./internal/application/state ./internal/adapter/outbound/state ./internal/adapter/outbound/sqlstore -count=1", "go test -race ./internal/application/state ./internal/adapter/outbound/state ./internal/adapter/outbound/sqlstore -count=1"},
			Evidence:     []string{"PROJECT_AUDIT S1 P1"},
		},
	}
	// Quality specs are all code/correctness candidates; default them to
	// ToolSignal unless a future spec explicitly classifies itself otherwise.
	for i := range specs {
		if specs[i].VerificationKind == "" {
			specs[i].VerificationKind = ToolSignalKind
		}
	}
	return specs
}

func Candidates() []Candidate {
	specs := CandidateSpecs()
	out := make([]Candidate, 0, len(specs))
	for _, spec := range specs {
		if resolvedCandidateIDs[spec.ID] {
			continue
		}
		out = append(out, Candidate{
			ID:          spec.ID,
			Title:       spec.Title,
			Category:    spec.Category,
			Status:      CandidateStatusOpen,
			Score:       Score(spec.Impact, spec.Feasibility, spec.Novelty, spec.Risk),
			Impact:      spec.Impact,
			Feasibility: spec.Feasibility,
			Risk:        spec.Risk,
			VerifyWith:  append([]string{}, spec.VerifyWith...),
			Evidence:    append([]string{}, spec.Evidence...),
		})
	}
	return out
}

func Score(impact, feasibility, novelty, risk float64) float64 {
	score := impact*0.38 + feasibility*0.30 + novelty*0.20 + (100-risk)*0.12
	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}
