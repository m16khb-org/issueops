# #561 remote score 라벨 점수 수정 계획

- lifecycle ID: `io-b5a7cbecca2a`
- 이슈: https://github.com/m16khb-org/issueops/issues/561
- 브랜치: `561-korean-label-score` (base `main`, 봉인 SHA는 record의 `branch_prepare.base_sha`)
- 요청 범위: 결정론적 라벨 점수 수정, 회귀 측정 수단, 테스트. 승인된 종료점은 draft PR 발행과 `execution complete`다.
- 세션: 워크트리 준비 뒤 환경별 자동 세션 인계를 적용한다. 인계는 요청 범위를 넓히지 않는다.

## 문제

`scoreIssueOpsRemoteLabel`(internal/domain/issueopsremote/issueops_remote_score.go:120-146)은
`0.65*overlap + 0.35*heuristic`으로 점수를 낸다(같은 파일 126-128행). `issueOpsRemoteLabelHeuristic`
(issueops_remote_score_tokens.go:50-62)은 키워드가 하나라도 토큰과 정확히 같으면 1(documentation은 0.75)을
돌려준다. 결과:

- 키워드만 맞으면 최대 0.35라 기본 threshold 0.70(issueops_remote.go:3)을 넘지 못한다.
- `issueOpsRemoteTokens`(issueops_remote_score_tokens.go:8-20)는 공백·구두점으로만 자르므로 "오류가"는 "오류"와 다르다.
- 실측: 단일 라벨 이슈 229개 중 224개가 라벨 0개, 정답 선택 4개.

## 설계

키워드 규칙이 있는 라벨(bug, enhancement, documentation)의 점수를 "이슈 토큰 중 유형 근거 토큰에서 이 라벨이
차지하는 비율"로 바꾼다.

1. `issueOpsRemoteLabelKeywords`를 `map[string][]string`으로 바꾸고 키워드를 보강한다. 한글 키워드는 토큰의
   접두와 일치하면 맞는다(조사·어미 흡수). 영어 키워드는 토큰 전체가 같아야 맞는다(`add`가 `address`에 맞지 않게).
2. 이슈 토큰 집합에서 규칙별 근거 토큰 수를 한 번 센다(`issueOpsRemoteLabelEvidence`). 한 토큰은 규칙마다
   최대 한 번 센다.
3. 규칙이 있는 라벨의 점수 = 그 라벨 근거 수 / 전체 근거 수. 전체 근거가 2개 미만이면 0(기권). 근거 1개로
   고른 경우의 정답률이 개발 집합(번호 232 미만)에서 6/9로, 근거 2개 이상(36/43)보다 낮았다.
4. 규칙이 없는 라벨은 점수(`0.65*overlap`, heuristic 항이 항상 0이던 값)와 evidence 두 줄(`token_overlap=%.2f`,
   `type_heuristic=0.00`)을 지금과 바이트 단위로 같게 유지한다. 최대 0.65라 기본 threshold에서 선택되지 않는 것도 같다.
   명시 `score` 입력은 지금처럼 그 값을 쓴다.
5. 규칙 라벨의 evidence는 지금처럼 두 줄이다: `type_evidence=<라벨 근거>/<전체 근거>`와
   `matched_tokens=<이 라벨에 맞은 토큰, 정렬, 쉼표 구분>`.
6. 규칙 라벨은 `description`·`aliases` 겹침을 더 이상 쓰지 않는다. 규칙 세 라벨의 점수 합은 1(근거가 있을 때)이라
   기본 threshold 0.70에서는 많아야 한 라벨만 선택된다. threshold를 0.5 이하로 낮추면 동률 두 라벨이 함께 선택될 수 있다.

키워드 출처와 검증 집합 노출(사실 그대로):

- 키워드는 개발 집합(번호 232 미만) 제목·본문과 개발 이슈에서 흔한 한국어·영어 결함/기능/문서 어휘에서 골랐다. 일부
  (`broken`, `feature`, `support`, `readme`, `오동작` 등)는 개발 집합에 나오지 않는 일반 어휘다.
- 최소 근거 2개 규칙은 개발 집합 수치(근거 1개 선택 6/9, 2개 이상 36/43)로 정했다.
- 검증 집합(232-560)은 규칙 확정 전에 한 번 측정했다(선택 56, 정답 54). 그 뒤 키워드와 최소 근거 규칙을 바꾸지 않는다.
  검증 집합은 완전히 독립된 평가가 아니며, 개발 집합에서는 기준에 못 미친다(선택 43, 정답률 83.7%).
  완료 보고에 두 구간 결과를 함께 적는다.

키워드 목록:

- bug: bug, defect, failure, fail, failed, fails, error, crash, regression, broken, 오류, 결함, 버그, 실패, 회귀,
  깨진, 깨지, 잘못, 누락, 오동작, 재현, 레이스, 교착, 손상, 유실, 막힌, 막혀, 거부되, 고친다, 수정한다
- enhancement: feature, request, support, add, enhancement, 개선, 추가, 지원, 도입, 확장, 새로운, 신규, 기능, 강화
- documentation: docs, documentation, readme, 문서

## 재사용하는 기존 구현

- `issueOpsRemoteTokens`(issueops_remote_score_tokens.go:8): 이슈 토큰 집합을 그대로 쓴다. 바꾸지 않으므로 관련 이슈
  점수(`scoreIssueOpsRemoteIssue`)는 영향이 없다.
- `issueOpsRemoteOverlap`, `clampScore`, `scorePtr`, apply hint·정렬 helper(issueops_remote_score_helpers.go): 그대로 쓴다.
- 품질 표 테스트 형식은 `internal/adapter/projectdocs/projectdocs_test.go:296`의
  `TestRouteProjectDocsQualityTableHasNoRequiredOmissions`와 `testdata/route_quality_cases.json` 선례를 따른다.
- 측정 스크립트는 `scripts/autoresearch/`의 eval 스크립트 선례(doc_injection_eval.py 등, stdlib만 사용)와
  `internal/architecture/ddd_responsibility_test.go:746`의 등록 방식을 따른다.
- 새로 만드는 것: `issueOpsRemoteLabelEvidence`(근거 수 집계)와 `issueOpsRemoteKeywordMatches`(접두/정확 일치) 두 함수.
  기존 heuristic은 "한 키워드라도 맞으면 고정 점수"라 개수를 세지 못하므로 대체한다.

## 성능 영향

이슈를 만들 때 한 번 도는 CLI 경로이고 hot path가 아니다. 근거 집계는 토큰 수 T × 키워드 수 K(약 50)의
문자열 비교이며, 라벨 후보마다가 아니라 요청마다 한 번 계산한다. 기존은 라벨마다 K번 map 조회였다.
본문 1만 토큰에서도 50만 번의 짧은 비교라 체감 차이가 없다. 측정 계획: 없음(측정된 병목이 아니다).

## 하위 호환성과 side effect

- CLI `remote score` JSON, MCP schema, `IssueOpsRemoteScoredItem` 필드는 그대로다. golden은 바뀌지 않는다.
- 바뀌는 것은 규칙 라벨 세 개의 `score` 값과 evidence 문자열 내용이다. evidence 줄 수는 그대로 두 줄이다.
  `internal/domain/issueopsremote/remote_gaps_test.go:526`의 `len(res.SelectedLabels[0].Evidence) != 2` 단언이 이 줄 수를
  고정한다(입력 토큰 `bug`, `failure`는 새 규칙에서도 bug 2/2로 선택된다). evidence 문자열 내용을 검사하는 테스트나 소비자는
  없다(`rg 'type_heuristic|token_overlap'` 결과 issueops_remote_score.go 한 곳).
- 기존 테스트 `TestIssueOpsRemoteLabelHeuristicMatchesKnownLabels`는 삭제되는 함수를 고정하므로 새 동작을 검사하는
  테스트로 바꾼다.
- 저장 데이터·record schema·provider body 계약 변화는 없다. 롤백은 커밋 revert 하나다.
- DDD inventory(`internal/architecture/testdata/ddd_responsibility_inventory.json`)의 심볼 목록을 바꾼다.
- 측정 스크립트는 실행 시 `gh issue list --state all --limit 1000`과 `gh label list`로 이 저장소를 읽는다(읽기 전용,
  네트워크). 요청 파일은 임시 디렉터리에만 쓰고 지운다. 저장소·원격에는 쓰지 않는다. CI·self-verify에 연결하지 않는다.

## 적용되는 결정과 주의사항

- .issueops/CONSTITUTION.md 제2장 문제 해결 원칙: "재현 근거, 호출 경로, 상태 전이, 정책 경계, 문서 계약을 따라가며 문제가 발생한 근본 원인을 확인한 뒤 그 원인을 제거한다." -> threshold를 낮추거나 출력만 고치지 않고 점수 규칙을 고친다.
- .issueops/CONSTITUTION.md 제2장: "수정 뒤에는 같은 실패가 재발하지 않음을 보여주는 targeted test, command smoke, grep evidence, 로그/상태 확인 중 작업 성격에 맞는 검증을 남긴다." -> 품질 표 테스트와 검증 집합 측정을 게이트로 둔다.
- .issueops/ARCHITECTURE.md 의존 방향 불변식: "`internal/domain`은 contract와 순수 domain helper에만 의존하고 concrete adapter나 `cmd/...`를 import하지 않는다." -> 변경은 domain 순수 함수 안에 머물고 새 import는 표준 라이브러리뿐이다.
- .issueops/CONVENTIONS.md 이슈 산출물 레이아웃: "이 표가 유일한 규정이다." -> 측정 데이터와 스크립트를 `.issueops/issues/561/`에 두지 않고 `scripts/autoresearch/`에 둔다.
- .issueops/CAUTIONS.md Universal summary: "`.issueops/*.md` 편집은 response-contract golden을 드리프트시킨다." -> 이 계획은 `.issueops/*.md`를 고치지 않는다. 문서 단계에서 고치게 되면 golden을 다시 만든다.
- .issueops/ADR.md -> .issueops/adr/2026-07-07-standalone-harness-policy.md: "issueops consumes them only through ordinary inspectable boundaries such as local files, command output, or already-configured MCP data." -> 외부 모델을 core에 넣지 않는다. 측정 스크립트도 core 경로가 아니다.
- .issueops/TESTING.md 최소 완료 기준: "Go 코드 변경의 기본 검증(`gofmt -l`, `go test -race ./...`, `go vet ./...`, architecture ratchet, operational-health 위임, golden 갱신 조건)은 [unit-and-contract.md](testing/unit-and-contract.md)가 소유하고" -> 검증 단계에서 gofmt·vet·race·architecture를 실행한다.
- .issueops/testing/unit-and-contract.md: "실패해도 원인을 알 수 없는 거대한 fixture나 broad snapshot만 둔다." (피해야 할 예) -> 품질 표는 번호 232 미만 이슈에서만 뽑은 짧은 발췌 10건 안팎으로 두고, 229개 원문(1.4MB)은 커밋하지 않는다.
- .issueops/TECH_STACK.md: "Python 검증은 stdlib suite runner와 스킬에 이미 선언된 Pydantic/Typer를 사용한다." -> 측정 스크립트는 stdlib만 쓴다.
- .issueops/AGENT_WORKFLOW.md Work 절: 대조했으나 이 변경의 범위를 정하는 항목 없음.
- .issueops/cautions/issueops-lifecycle.md: 대조했으나 없음(readiness 게이트·golden 항목은 이 변경과 무관).
- .issueops/adr/2026-09-24-issue-and-pr-bodies-are-human-documents.md: "Hashes, commit SHAs, plan and spec texts, label scores, and cleanup audits leave the bodies" -> PR 본문에 라벨 점수 표를 넣지 않고 측정 결과 요약만 쓴다.
- .issueops/adr/2026-08-22-task-gate-ledger.md: "CHECK 명령은 raw shell이 아니라 command policy 경로로 실행한다." -> 게이트 CHECK는 argv 한 줄로 쓴다.
- .issueops/adr/2026-10-06-remove-the-last-legacy-compatibility-paths-and-non-release-p.md: "remote score 입력의 related_issues/labels alias" 제거 -> 입력 키는 현재 `label_candidates`만 쓴다.
- .issueops/adr/2026-10-06-delete-re-export-shims-and-stop-naming-live-code-legacy.md: 대조했으나 없음(re-export·legacy 이름을 만들지 않는다).
- .issueops/cautions/2026-08-27-skill-without-openai-yaml-self-verify-qa-gate.md: 열지 않음: 스킬 파일을 바꾸지 않는다.

## 구현 순서

1. RED: `internal/domain/issueopsremote/testdata/label_quality_cases.json`과 `TestScoreIssueOpsRemoteLabelQualityTable` 추가.
   번호 232 미만 이슈의 발췌로만 선택·기권을 고정한다(조사 붙은 한국어 bug, enhancement, documentation, 근거 엇갈림 기권,
   근거 1개 기권). 규칙 없는 라벨의 점수·evidence 불변은 같은 테스트 파일의 별도 경계 테스트로, 명시 score는 기존
   `issueops_remote_test.go`로 고정한다. 현재 코드에서 실패하는 것을 확인한다.
2. GREEN: 설계 1-6을 구현한다. `TestIssueOpsRemoteLabelHeuristicMatchesKnownLabels`를 새 함수의 경계 테스트(접두/정확 일치,
   근거 2개 미만 기권)로 바꾼다. `remote_gaps_test.go:526`의 evidence 두 줄 단언은 그대로 통과해야 한다.
3. `scripts/autoresearch/label_score_eval.py` 추가: `gh issue list --state all --limit 1000 --json number,title,body,labels`로
   라벨이 정확히 하나인 이슈를 읽어 제목 앞 `[..]` 태그와 본문 `#번호`·GitHub 이슈/PR URL을 지우고, `gh label list`의 이름·설명을
   후보로 넣어 `--binary`의 `remote score --judge none --json`을 실행한다(요청 파일은 임시 디렉터리). 번호 범위의 대상 수(`n_total`)·
   선택 수·정답 수·정답 비율을 JSON 한 줄로 내고, `--check`면 기준(선택 50 이상, 정답 비율 0.85 이상)을 넘을 때 마지막 줄에
   `PASS` 한 단어만 출력하고 아니면 `FAIL`과 종료 코드 1을 낸다.
4. DDD inventory와 `ddd_responsibility_test.go` 등록을 갱신한다.
5. `go build -o bin/issueops ./cmd/issueops` 후 게이트를 채운다.

## 게이트

- G1: 품질 표 테스트가 실제 이슈 발췌의 선택과 기권을 고정한다(테스트가 ScoreIssueOpsRemoteCandidates를 실행해 selected_labels를 기대값과 비교) | CHECK: go test ./internal/domain/issueopsremote/ -run TestScoreIssueOpsRemoteLabelQualityTable -count=1 -v | EXPECT: --- PASS: TestScoreIssueOpsRemoteLabelQualityTable
- G2: 검증 집합(번호 232-560 단일 라벨 이슈)에서 기본 threshold로 선택 50개 이상, 정답 비율 85% 이상(실제 bin/issueops remote score를 실행해 GitHub 라벨과 비교) | CHECK: python3 scripts/autoresearch/label_score_eval.py --binary bin/issueops --min-number 232 --max-number 560 --check | EXPECT: PASS
- G3: 관련 이슈 점수·명시 score·CLI remote score 동작이 유지된다 | CHECK: go test ./internal/domain/issueopsremote/ ./cmd/issueops/issueopscli/remotecmd/ ./cmd/issueops/issueopscli/ -count=1
- G4: DDD 책임 원장이 새 심볼과 스크립트를 포함한다 | CHECK: go test ./internal/architecture/ -count=1

G2는 `gh`로 GitHub를 읽으므로 `gates check --network`로 실행한다. 자식 `gh`는 `--env` 기본값(`HOME,PATH`)으로 키체인 인증을
쓴다. G3이 기본 timeout 120초를 넘으면 `--timeout-seconds 600`으로 실행한다.

## 검증(게이트 외)

`gofmt -l`, `go vet ./...`, `go test ./... -count=1`, `go test -race ./... -count=1`은 7단계에서 한 번 실행한다.
