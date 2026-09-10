# Issue #507 verification report

## Outcome

`execution prepare`가 `record.intent`를 결정적으로 렌더한 `intent.md`를 plan·spec과 같은
봉인 artifact 경로(`<artifact_dir>/intent.md`, 0600, 불변 writer)에 쓰고 manifest에
`intent` digest를 넣는다. direct와 Orca가 같은 `materializeStagedArtifacts`를 지나므로
한 곳만 바뀌었다. owner prompt 시작 절차 3은 intent.md를 요청자 의도 계약으로 먼저 읽고
이슈 본문과 충돌하면 mutation 없이 blocker를 보고한다. issueops-review(diff 자료),
issueops-verify(1절 의도 대조·4절 readiness), issueops-create-pr(`## 의도` 출처),
execution.md, CONVENTIONS, ADR이 같은 규칙을 적는다.

## TDD evidence

- RED: `internal/domain/issueopsintent` 테스트 3건이 `Document`·`Render` 미정의로 빌드 실패.
  GREEN: 결정성, 9개 필드 포함과 `recorded_at` 부재, 빈 목록·미링크 표기 통과.
- RED: `TestMaterializeStagedArtifactsSealsDerivedIntent`가 `intent.md` 부재로 실패,
  `RejectsSecretLikeIntentWithRecordHint`·`IntentIsImmutableAcrossRerecords`가 오류 없음으로 실패.
  GREEN: 0600 파일과 manifest digest 일치, 자격 증명 형태는 `issueops intent record` 안내가 든
  오류, 기록 시각만 바뀐 재-materialize는 통과하고 내용이 바뀐 재-materialize는
  `immutable owner artifact already exists with different identity`로 거부.
- RED: `TestRecordIntentRejectsColonFormSecretBeforeWriting`이 오류 없음으로 실패.
  GREEN: 콜론 형태는 기록 시점에 거부되고 등호 형태는 종전대로 `<redacted>`로 저장.
- RED: `TestExecutionOwnerPromptReadsSealedIntentBeforeImplementing`이 템플릿 문구 부재로 실패.
  GREEN: 템플릿과 parity 문서를 같은 바이트로 고쳐 parity·resume identity 테스트 포함 통과.
- 회귀 고정: `NormalizeName("intent")` 거부 항목 추가(기존 동작).

## Verification

- 게이트 원장 `.issueops/issues/507/gates.md` G1–G13 모두 met(`gates check --write`).
  G13은 `go test ./... -count=1`을 출력 캡처 래퍼로 감싼 `ALL_PASS`.
- `go test ./internal/architecture/ -count=1` ok (domain은 contract를 import하지 않도록
  렌더러 입력을 순수 필드로 두고 매핑은 `intentdesign.IntentDocument`가 담당).
- `go test ./cmd/issueops/issueopsapp -run Golden -count=1` ok (response contract 드리프트 없음).
- `go test -race` on issueopsintent, intentdesign, issueopsartifact ok.
- `gofmt -l ./cmd ./internal` 0건, `go vet ./...` 종료 코드 0.
- `validate-skill`·`verify-skill-shell`: issueops-review, issueops-verify, issueops-create-pr, issueops 통과.

## Side effects

- prepare가 `<artifact_dir>/intent.md`를 하나 더 쓴다(ignore 경로). Orca packet의
  `artifact_manifest`에 `intent` 키가 봉인된다. record 필드·CLI JSON 키·schema_version은 그대로다.
- `intent record`가 콜론 형태 자격 증명을 새로 거부한다. 등호 형태는 종전대로 redaction된다.
- owner prompt 템플릿 바이트가 바뀌었다. 이미 봉인된 generation은 durable digest로만 비교된다.

## AI-slop clean

- Changed: `internal/adapter/issueops/issueops_artifact_stage.go`에서 intent 블록의 크기 검사와
  `issueopsartifactcontract` import를 제거했다(duplication). `writeExecutionOwnerArtifact`가 같은
  상한(`OwnerArtifactMaxBytes`)으로 이미 거부한다.
- Preserved intentionally: intent 블록·`IntentDocument`·`RecordIntent`의 주석 세 개는 기록 시각 제외와
  검사 시점이라는 비자명한 결정을 설명하므로 남겼다.
- Out-of-scope findings: 없음.
- 측정(approximate shell metric, Go 추가 줄 기준, untracked 포함): SNR 0.93 → 0.93
  (total 348→344, noise 23→23), 새 함수 분기 수 최대 3(ceiling 6), 같은 파일 중복 블록 0,
  boilerplate 50% 초과 파일 0. `git diff --check` 통과.
- 재검증: `gates check --write` G1–G13 met(G13 `ALL_PASS`), `go vet ./internal/adapter/issueops/` ok.

## 성능 측정

hot path가 아니다(prepare·resume는 사이클당 수 회). 추가 비용은 record 하나의 문자열 렌더와
파일 쓰기 1회다. `go test ./...` 전체 소요는 정리 전후 모두 G13 timeout 580초 안에 끝났고
`-race`는 touched 세 패키지에서 1.6s/2.0s/5.5s였다.

## Deviations

- 계획의 `materializeStagedArtifacts` 코드 블록은 domain 렌더러가 contract를 받는 형태였으나,
  architecture ratchet(`domain_must_not_import_implementation`)이 domain의 contract import를
  금지해 렌더러 입력을 순수 필드 구조체로 바꾸고 record→Document 매핑을
  `intentdesign.IntentDocument`에 두었다. 동작과 테스트 범위는 계획과 같다.
- 게이트 EXPECT는 원장 매칭 규칙(줄 단위 완전 일치 또는 `/정규식/`)에 맞춰 정규식 형식으로 적었다.

## 의도 대조

봉인 intent 문서의 성공 기준 7개(AC-01~AC-07)는 G1(AC-02), G2(AC-01), G3(AC-02 기록 시점),
G4(AC-03), G5·G6(AC-04), G7–G11(AC-05), G12(AC-06), G13(AC-07)이 덮는다. 비목표 5개는
diff에 없다: 추적 INTENT.md 없음, 이슈 템플릿 미변경, `completionArtifactNames` 미변경,
stale 게이트 없음, `NormalizeName` 목록 미변경.
