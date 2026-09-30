# 리뷰 실행 설정 검증 자료

Lifecycle: io-814b092d660e / issue #522 / generation 2

## 의도 대조

owner가 구현 리뷰 직전에 exact ID의 next.review를 읽고 issueops-review에 전달하도록 변경했다. 생성 implementation-review 명령은 실제 실행 model/effort를 입력받는다. prepare 기본값과 구현자 override, 기존 봉인 파일은 보존한다. 품질 scanner와 latest status는 이 변경의 범위 밖이다.

## 검증 경로

- RED: 실제 owner 렌더링의 runtime 조회 누락과 고정 감사 인자를 재현했다. 원본 출력은 artifact/red.log에 보존한다.
- GREEN/SURFACE: 렌더링·catalog·3호스트 model-role·봉인 보존 focused tests가 통과했다. prompt mirror byte parity도 검사한다.
- G1~G4의 실제 명령·종료 코드·출력은 gates.md가 소유한다. G1은 계획의 focused tests를 포함한 adapter package 전체를 검사한다. G4는 candidate self-verify의 단일 run, 모든 step, 모든 goal의 95 초과를 검사한다. 전체 원본 출력은 artifact/self-verify.json에 저장한다.
- 최종 Go battery는 gofmt, self-verify가 실행한 race/vet/test/build/golden/docs/inspect step을 대조하며, 빠진 vet/race가 있으면 별도 보완한다.
- Manual-QA channel: auxiliary CLI surface. production buildExecutionOwnerArtifacts로 원본과 candidate의 Codex/Claude prompt를 렌더하고 빈 컨텍스트 리뷰어에게 5개 시나리오를 제공한다. 실행값과 기록 인자를 함께 평가한다. 평가 입력·출력과 실제 reviewer launch 설정은 artifact에 보존한다. 문자열 테스트는 semantic 평가를 대신하지 않는다.

## Side effect와 하위 호환

새 prepare의 owner prompt와 생성 감사 명령만 바뀐다. CLI/MCP/record schema와 기본 모델 정책, owner override는 그대로다. 기존 봉인 자료는 재생성하거나 migration하지 않는다. 같은 generation의 변경된 자료는 immutable writer가 거부하며 새 generation을 생성한 뒤에도 이전 packet/prompt digest가 유지되는지 3호스트에서 검사한다. 원격 side effect는 승인된 이슈 브랜치 push와 Draft PR 발행 및 cycle 완료 기록이다.

## 성능과 정리

리뷰 직전에 읽기 전용 next 결과를 소비한다. polling·provider 조회·cache·tier 계산 중복을 추가하지 않는다. runtime 속도 개선은 주장하지 않는다. 변경된 Go 추가행의 comment-proxy SNR은 정리 전 37/38=0.974, 정리 후 39/40=0.975다. 추가 package/import boilerplate는 두 시점 모두 0행이다. AST 기반 측정이 아닌 근사치이며 문서와 prompt에는 이 지표를 적용하지 않았다. 측정 입력에는 추적 diff와 미추적 cycle 자료 목록을 포함했다. 입력 0행일 때 비율은 N/A로 처리한다.

정리에서 실제 사용하지 않는 테스트의 prepare model/effort 필드와 1필드 struct를 제거하고, temporary rendering exporter를 제거했다. 기록 명령의 runtime placeholder는 의도된 입력 계약이므로 유지한다. 무관한 파일은 수정하지 않았다. 테스트용 state와 packet은 Go t.TempDir에서 제거하며 production record를 실험용으로 수정하지 않는다.

## 문서와 독립 리뷰

CONSTITUTION·ARCHITECTURE·CONVENTIONS·CAUTIONS·ADR·TESTING을 계획과 diff 양방향으로 대조했다. runtime 정책을 복제하지 않고 소비하며 새 layer나 schema를 추가하지 않았다. CONVENTIONS의 미구현 TODO를 project_docs route/read/SHA-CAS revise로 실제 동작에 맞췄다. owner template과 버전 관리 prompt는 byte parity를 유지한다. 독립 diff 리뷰의 실제 verdict와 reviewer model/effort는 durable implementation_review가 소유한다. hook은 project-doc context만 제공하며 구현·리뷰 기록·publication·completion은 active owner와 CLI가 수행한다.

## 종료 경계

승인된 종료점은 Draft PR 발행과 execution complete다. merge, 이슈 종료, worktree/branch 삭제는 수행하지 않는다. 기존 source 설치 binary를 교체하거나 다른 두 사이클을 기다리지 않는다.

## 최종 battery 결과

G1~G4 모두 충족했다. candidate self-verify의 단일 run은 26단계 전부 성공했고 최저 goal score는 100이다. risk QA tier의 실제 명령에 `go test -race ./... -count=1`과 `go vet ./...`가 포함됐으며, 해당 step도 성공했다. 별도 중복 전체 테스트는 실행하지 않았다. 일반 Go test와 golden은 full race suite에 포함되는 계약을 self-verify step에서 확인했다. gofmt 출력은 비어 있었다. 검증 중 implementation source 변경은 없었다. 준비 중 gate spec의 pipe/newline 형식 오류와 timeout 상한 오류는 실행 전에 해소했으며 성공 evidence에 합치지 않았다.
