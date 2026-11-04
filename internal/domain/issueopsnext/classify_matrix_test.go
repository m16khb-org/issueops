package issueopsnext

// 이 파일은 Classify의 상태 조합 행렬 테스트다. classify_test.go의 표는 "이미 본
// 상태"를 고정하지만, 이 분류기는 놓친 조합이 나올 때마다 규칙을 덧대 왔다. 여기서는
// 라우팅을 가르는 차원(phase, execution/lease/holder/pending/completion, worktree,
// git 작업, cleanup 시도, 이슈·브랜치·리뷰 산출물, readiness)을 열거해 부분 곱과
// pairwise 집합을 만들고, 모든 조합에 같은 불변식을 건다.
//
// 불변식은 결과(Decision)만 본다: 패닉 없음, 알려진 stage key, 실제 존재하는
// issueops 하위 명령, 카탈로그에 있는 flag, 문서화된 자리표시자만 남은 next_command,
// 읽기 전용 stage의 비변경 명령, 그리고 lease 규칙·phase 진행에 대한 몇 가지 속성이다.

import (
	"strings"
	"testing"
	"time"

	issueopsnextcontract "issueops/internal/contract/issueopsnext"
)

// ---------------------------------------------------------------- 테스트

func newMatrixChecker() *matrixChecker {
	return &matrixChecker{
		m:          &matrix{dims: buildMatrixDims()},
		usage:      parseUsageCatalog(),
		actorFlags: actorLegendFlags(),
		violations: newMatrixViolations(),
		stages:     map[string]int{},
	}
}

// matrixSets visits every case of the layered matrix: full products over the
// dimension groups that interact, plus a pairwise cover of all dimensions
// together. It returns the number of cases per set.
func (k *matrixChecker) matrixSets(visit func(sel []int)) map[string]int {
	m := k.m
	counts := map[string]int{}
	validExec := m.indices(dExec, func(v matrixValue) bool { return !v.malformed })
	selfExec := []int{m.indexOf(dExec, "exec=active/self/unobserved/completion0")}

	// A: every phase x every valid lease state x the readiness/artifact tail.
	counts["lease x phase x readiness"] = m.product(map[int][]int{
		dPhase: allIndices(len(matrixPhases)), dExec: validExec,
		dLocal: {0, 1, 2, 3, 4, 5}, dArtifact: {0, 1},
	}, visit)
	// B: preparation without an execution: issue, branch, reviews, staging, intent.
	counts["preparation"] = m.product(map[int][]int{
		dPhase: allIndices(len(matrixPhases)), dIssueURL: {0, 1}, dBranchPrepare: {0, 1, 2, 3},
		dArchReview: {0, 1, 2, 3}, dStagedArch: {0, 1}, dStagedPlan: {0, 1},
		dDevilsAdvocate: allIndices(len(m.dims[dDevilsAdvocate].values)), dIntent: {0, 1, 6},
	}, visit)
	// B2: every create-issue intent status, a claimed canonical root and a started
	// cleanup against the early states.
	counts["root conflict and cleanup"] = m.product(map[int][]int{
		dPhase: allIndices(len(matrixPhases)), dRootConflict: {0, 1}, dIssueURL: {0, 1}, dBranchPrepare: {0, 1, 2, 3},
		dIntent: allIndices(len(m.dims[dIntent].values)), dCleanup: allIndices(len(m.dims[dCleanup].values)),
	}, visit)
	// C: the live holder entering and doing implementation.
	counts["holder implement"] = m.product(map[int][]int{
		dPhase: {2, 3, 4, 5}, dExec: selfExec, dBranchPrepare: {0, 1, 2, 3}, dPlanPath: {0, 1}, dCompat: {0, 1, 2},
		dDevilsAdvocate: allIndices(len(m.dims[dDevilsAdvocate].values)), dDevilsAdvocateStale: {0, 1},
		dProfile: allIndices(len(m.dims[dProfile].values)), dChildGate: {0, 1},
	}, visit)
	// D: the live holder after implement: seals, local gates, publication.
	counts["holder publication"] = m.product(map[int][]int{
		dPhase: {5, 6, 7}, dExec: selfExec, dProfile: allIndices(len(m.dims[dProfile].values)),
		dLocal: allIndices(len(m.dims[dLocal].values)), dArtifact: {0, 1}, dPRHead: {0, 1, 2}, dWorktreeHead: {0, 1, 2},
	}, visit)
	// E: pairwise over every dimension at once.
	sizes := make([]int, dCount)
	for dim := range sizes {
		sizes[dim] = len(m.dims[dim].values)
	}
	rows := pairwiseRows(sizes)
	for _, row := range rows {
		visit(row)
	}
	counts["pairwise"] = len(rows)
	return counts
}

func TestClassifyMatrixInvariants(t *testing.T) {
	start := time.Now()
	k := newMatrixChecker()
	counts := k.matrixSets(k.run)
	elapsed := time.Since(start)

	total := 0
	for name, count := range counts {
		t.Logf("%-28s %6d cases", name, count)
		total += count
	}
	t.Logf("total %d cases in %s; stages reached: %v", total, elapsed.Round(time.Millisecond), k.stages)
	if k.cases != total {
		t.Fatalf("visited %d cases, generated %d", k.cases, total)
	}
	// The floor keeps a future dimension cleanup from silently shrinking the
	// matrix; the ceiling keeps the run bounded.
	if total < 50_000 || total > 150_000 {
		t.Errorf("matrix size %d is outside the intended 50k..150k band", total)
	}
	for _, stage := range producibleStages {
		// none is covered by TestClassifyMatrixEmptyRecord. A stage the matrix
		// never reaches is a stage it proves nothing about.
		if stage != issueopsnextcontract.StageNone && k.stages[stage] == 0 {
			t.Errorf("no matrix case reaches stage %q", stage)
		}
	}
	k.violations.report(t)
}

func TestClassifyMatrixEmptyRecord(t *testing.T) {
	for _, id := range []string{"", " ", "\t"} {
		for _, phase := range matrixPhases {
			for _, source := range []string{"/repo", ""} {
				got := Classify(Input{Record: issueopsnextcontract.Record{ID: id, Phase: phase}, SourceRoot: source})
				if got.Stage.Key != issueopsnextcontract.StageNone || got.Stage.Index != 0 {
					t.Fatalf("id %q phase %s: stage %+v", id, phase, got.Stage)
				}
				if !strings.HasPrefix(got.NextCommand, "issueops start --repo ") || got.Exits.AbandonCommand != "" || got.Exits.PauseCommand != "" {
					t.Fatalf("id %q: next %q exits %+v", id, got.NextCommand, got.Exits)
				}
				if strings.Contains(got.NextCommand, "<") != (source == "") {
					t.Fatalf("source %q: placeholder state of %q", source, got.NextCommand)
				}
			}
		}
	}
}

// pairwiseRows must leave no pair of dimension values uncovered, otherwise the
// matrix silently misses interactions.
func TestClassifyMatrixPairwiseCoversEveryPair(t *testing.T) {
	m := &matrix{dims: buildMatrixDims()}
	sizes := make([]int, dCount)
	for dim := range sizes {
		sizes[dim] = len(m.dims[dim].values)
	}
	rows := pairwiseRows(sizes)
	seen := map[[4]int]bool{}
	for _, row := range rows {
		for i := range row {
			for j := i + 1; j < len(row); j++ {
				seen[[4]int{i, row[i], j, row[j]}] = true
			}
		}
	}
	for i := range sizes {
		for j := i + 1; j < len(sizes); j++ {
			for a := range sizes[i] {
				for b := range sizes[j] {
					if !seen[[4]int{i, a, j, b}] {
						t.Fatalf("pair %s=%s / %s=%s is not covered", m.dims[i].name, m.dims[i].values[a].label, m.dims[j].name, m.dims[j].values[b].label)
					}
				}
			}
		}
	}
}
