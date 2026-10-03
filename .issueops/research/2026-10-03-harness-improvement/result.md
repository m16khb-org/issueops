# 하네스 개선 결과

후속 수정으로 아래 검증 실패를 해결했다. 현재 상태는
[후속 검증 결과](remaining-fixes.md)의 self-verify 28/28 통과 기록을 따른다.
아래 본문은 최초 개선 run의 이력이다.

## 결론

조사와 계획에 따라 네 개선을 구현했다. 전체 Go race·vet·build·golden과 실제
CLI/MCP 검증은 통과했다. self-verify 최종 run은 28개 중 25개가 통과했으며,
기존 문서·native integration 검사 문제 3개 때문에 전체 성공은 아니다.
커밋·푸시·설치는 하지 않았다.

계획: [2026-10-03-harness-improvement.md](../../plans/2026-10-03-harness-improvement.md).
조사: [상태 저장](state.md), [MCP](mcp.md), [중복](duplication.md),
[공식 자료](sources.md), [후보 검증](verified-shortlist.md).

## 구현

| 항목 | 변경 | 증거 |
|---|---|---|
| T1 잠금 수명 | 요청 취소가 callback 실행 중 SQLite span을 자동 해제하지 않도록 잠금 transaction context를 분리했다. 획득 전후·재시도·callback·data write의 요청 취소는 유지한다 | `span_lifetime_test.go`: 실제 SQLite driver에 전달된 context와 다른 연결의 typed busy/해제를 검사 |
| T2 State 직렬화 | Delete가 writer span을 사용하고 confirmed prune의 목록 선택부터 삭제까지 같은 span을 유지한다. 이미 취소된 요청은 저장소를 열지 않는다 | `state_serialization_test.go`: 목록·삭제 순간 별도 SQLite 연결이 잠금을 획득하지 못함을 검사. `cancellation_test.go`: 7개 진입 경로의 OpenStore·callback 호출 0 |
| T3 MCP 요청 비용 | handler 생성 때 input schema의 closed projection을 준비해 요청마다 catalog 탐색·schema 복제를 하지 않는다 | `input_validation_test.go`: 병렬 정상/오류 입력, nested unknown property, protocol error와 effect 이전 거부 |
| T4 문서 라우팅 | 호출당 토큰 집합과 문서 중복 집합을 한 번만 생성한다. 사용하지 않는 두 helper를 제거했다 | `route_test.go`: Unicode·숫자·구두점 경계, 중복 제거와 첫 등장 순서 |

`internal/architecture/testdata/ddd_responsibility_inventory.json`은 정식 생성기로
추가한 helper 1개와 삭제한 helper 2개만 반영했다. 공개 DTO·MCP schema·상태
schema·workspace별 policy reload는 변경하지 않았다.

## 측정

Go 1.27.1, darwin/arm64, Apple M4에서 동일 입력·명령으로 각 5회 측정했다.
각 표의 시간은 중앙값이다. 병렬 조사와 같은 머신을 사용했으므로 ns/op를
서비스 전체 throughput이나 보장된 지연 개선으로 확대 해석하지 않는다.

| benchmark | 전→후 ns/op | 전→후 B/op | 전→후 allocs/op |
|---|---:|---:|---:|
| MCP handler | 3227 → 2963 | 약 3186 → 1500 | 39 → 29 |
| 라우팅 general | 4543 → 3875 | 1800 → 1704 | 57 → 51 |
| 라우팅 복합 task | 6619 → 7869 | 4192 → 2536 | 62 → 50 |
| 라우팅 긴 Unicode task | 51286 → 37790 | 27312 → 2536 | 68 → 50 |

MCP handler의 할당 bytes는 약 52.9%, 긴 라우팅 입력은 약 90.7% 감소했다.
복합 라우팅 입력의 시간 중앙값은 증가했으므로 모든 입력이 빨라졌다고
주장하지 않는다. 상세 입력과 수치는 [routing-evidence.md](routing-evidence.md)에 있다.

```sh
go test ./cmd/issueops/mcpcli -run '^$' \
  -bench '^BenchmarkSDKToolHandler$' -benchmem -benchtime=100ms -count=5
go test ./internal/domain/projectdoc -run '^$' \
  -bench '^BenchmarkRouteDocsForTask$' -benchmem -benchtime=100ms -count=5
```

## 검증 결과

최종 단일 run:

```sh
PATH="/tmp/issueops-improvement-python.QksUWu/bin:$PATH" \
  ./bin/issueops self-verify --seed=100 --target-score=95 \
  --llm-eval=false --collect-all-steps --progress=jsonl --json
```

원본 JSON: `/tmp/issueops-improvement-final.json`.
소요 297,867ms. exit 1, `termination_eligible=false`. 서로 다른 run의 부분
성공을 합쳐 95점 게이트 통과로 표시하지 않았다.

- 최종 `risk QA tier`의 실제 명령
  `go test -race ./... -count=1 && go vet ./...`가 통과했다.
- 같은 run의 전체 테스트·contract golden은 성공한 full race suite를 재사용했다.
- build, binary drift, inspect, docs, contract/conformance, MCP smoke,
  state roundtrip, parallel isolation, daemon resilience, QA gate가 통과했다.
- MCP 패키지에는 실제 loopback HTTP 두 클라이언트의 동시 진입·authority·trace
  격리, stdio/daemon transport, invalid argument 및 structured output 검사가 포함된다.
- 별도 최신 CLI smoke에서 임시 state에 `smoke=verified`를 저장·조회하고,
  `state prune --max-age 1h --confirm`이 새 checkpoint를 보존함을 확인했다.
  `project route-docs --repo . --task 'implement ci pr' --json`도 정상 순서로 반환했다.
- LSP error 0, gofmt와 `git diff --check` 통과.

수정 전 실패 증거:

- 원래 `BeginTx(ctx)`를 적용하면 결정적 driver-context 테스트가
  `request cancellation reached the acquired lock: context canceled`로 실패했다.
- 사전 취소 테스트는 기존 구현에서 7개 경로 모두 저장소를 열거나 callback을
  실행해 실패했다. 수정 후 전체 race suite에서 통과했다.
- 새 helper의 책임 목록 누락을 전체 검사가 잡았다. 목록을 정식 갱신한 뒤
  전체 검증을 처음부터 다시 실행했다.

## 남은 기존 실패와 검증 한계

1. **Python fixture 검사와 redaction audit:** 기존
   `.issueops/plans/agent-improvements-2026-10-02/host-usage-verification.md`가
   두 검사에서 거부됐다. redaction 위치는 69행이다. 파일의 working-tree
   blob과 HEAD blob이 모두 `ebf59704ba85cc8723a60776d18c558fee60262c`로 같아
   이번 변경이 아님을 확인했다. 원문 데이터는 보고서에 복사하지 않았다.
2. **Omo native integration:** 실제 `~/.omo/mcp.json`에는 HTTP issueops 설정이
   존재한다. 그러나
   `internal/adapter/verification/probe/nativeintegration/validation_native_integration_contract.go:88`
   의 `hasCanonicalOmoMCP`는 command/args/env를 가진 stdio 형식만 인정한다.
   HTTP 설정을 바꾸거나 검증기를 이번 작업에서 수정하지 않았다.
3. **golangci-lint:** 저장소와 CI는 v1 설정/v1.64.8인데 설치 도구는
   v2.12.2다. 정규 명령은 설정 버전 오류로 실패했다. 설정을 바꾸지 않고
   동일한 검사 집합을 CLI flag로 지정한 시도도 Go 1.26.3으로 빌드된 린터가
   실행 환경 Go 1.27.1의 `math/rand/v2`를 해석하지 못해 실패했다.
   Linux lint는 선행 실패로 실행되지 않았다. Go vet는 별도로 통과했다.
4. **독립 모델 리뷰:** 초기 카테고리 라우팅이 일부 지정 밖 모델로 돌아갔고,
   직접 지정한 Luna 6로 조사 결과를 재검증했다. 이후 OAuth refresh 401이
   발생했다. 사용자가 추가 에이전트 대신 현재 세션에서 직접 완료하도록
   선택해 남은 구현과 최종 검토는 메인 에이전트가 수행했다.
   Astra·Luna의 조사 실행은 확인했지만 Sol 6.1 리뷰가 완료됐다고 주장하지 않는다.
5. **기존 내부 schema validator 한계:** 임의의 알 수 없는 `type` 문자열을
   직접 validator에 넣으면 거부되지 않는 기존 동작을 발견했다. 새 오류
   보존 테스트는 실제로 지원하지 않는 `$ref` keyword로 기존 오류 계약을
   검사한다. schema validator 자체의 범위 확장은 하지 않았다.

## 검토와 제외 범위

메인 에이전트가 production diff, Delete 호출자, 실제 transport 테스트,
책임 목록 delta를 직접 대조했다. 새 전역 cache·dependency·authority 우회는 없다.
임시 `.omo/omo.jsonc`와 계획 초안은 제거했다.

key별 잠금 재설계, policy catalog 공유, pprof endpoint, 서비스 통계 API는
근거나 범위가 부족해 채택하지 않았다. 기존 문서 정리·native validator의 HTTP
지원·CI 린터 업그레이드는 별도 후속 태스크다.

공식 근거:
[Go 취소 전파](https://go.dev/doc/database/cancel-operations),
[SQLite transaction과 writer 배제](https://www.sqlite.org/lang_transaction.html),
[Go profiling](https://go.dev/doc/diagnostics).
