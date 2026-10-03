# 구현 후보 검증

## 채택

| 후보 | 직접 확인한 소스 메커니즘 | 검증 계약 |
|---|---|---|
| T1 span 수명 | `sqlstore.go`의 `runSpan` callback과 `BeginTx(ctx)`의 자동 취소 rollback 수명이 다르다 | 실제 별도 handle에서 취소된 holder의 callback 종료까지 busy 유지 |
| T2 State 삭제·취소 | `service.go` Delete가 WithSpan을 우회하고, prune이 List 뒤 별도로 Delete한다. 사전 취소 검사도 OpenStore보다 앞에 없다 | barrier 기반 writer/prune·Delete 순서, 사전 취소 시 I/O 0 |
| T3 MCP schema | handler마다 tool이 고정돼도 요청마다 catalog 탐색과 ClosedProjection을 반복한다 | handler-local 준비, 오류·격리·권한 계약 유지, allocation 비교 |
| T4 문서 라우팅 | 동일 task의 반복 FieldsFunc와 중복 집합 재생성 | Unicode·순서·중복 회귀, 동일 입력 전후 benchmark |

메인 에이전트가 위 세 production 경로와 기존 테스트를 직접 읽었다. T1/T2
기존 패키지 검사는 다음 명령이 exit 0으로 끝났다:

```sh
go test ./internal/adapter/outbound/sqlstore ./internal/adapter/outbound/state \
  ./internal/application/state ./internal/adapter/outbound/issueopsrecord \
  -count=1 -timeout=120s
```

T4의 측정과 회귀 결과는 [routing-evidence.md](routing-evidence.md)에 있다.
T1/T2의 새 실패 순서는 아직 regression으로 입증해야 하며, 기존 테스트 통과를
새 결함 재현으로 간주하지 않는다. T3은 allocation benchmark를 추가하고
측정하기 전에는 성능 효과를 주장하지 않는다.

## 기각·보류

- SQLite DSN·dependency 버전 문자열 고정 테스트: 동작 회귀 증거가 아니다.
- pprof endpoint·서비스 통계 추가: 실제 병목을 고치는 데 필요한 변경이 아니다.
- status CLI의 메모리를 서비스 메모리로 보고하는 방법: 관측 대상이 다르다.
- root lock 축소: cross-process·cross-record 계약 변경으로 이번 범위를 벗어난다.
- policy catalog 공유: mutable override 격리와 복제 비용을 먼저 측정해야 한다.

## 모델 및 검증 이력

최초 일부 카테고리 실행이 지정 밖 모델로 배정돼 중단 또는 재검증했다.
복구 노드의 실제 모델은 `chatgpt-subscription/gpt-6-luna`로 확인했다.
Astra의 state 보고서와 Luna의 MCP·중복 보고서를 메인 에이전트가 검토했다.
후속 공급자 인증은 `401 refresh_token_reused`로 실패했으며 사용자에게
재로그인을 요청했다. Sol의 독립 종합 검증은 현재 완료 증거가 아니다.

실행 계획은 [계획 파일](../../plans/2026-10-03-harness-improvement.md)이 소유한다.
