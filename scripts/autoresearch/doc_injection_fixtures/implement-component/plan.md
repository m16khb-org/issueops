# 계획: worker 상태 저장소를 새 outbound adapter 패키지로 분리

## 문제
worker job lifecycle record(`queued/running/succeeded/failed/cancelled`) 저장 코드가 `cmd/issueops/workercli`의
명령 처리 함수 안에 섞여 있어, MCP `worker_*` 도구가 같은 저장 로직을 다시 구현하고 있다.

## 변경
1. 새 패키지 `internal/adapter/outbound/workerstate`에 `Store`(Enqueue/Get/List/Cancel)를 만든다.
2. job 상태 전이 판정(`CanCancel`, `NextState`)과 SQLite 쿼리 함수(`selectJobsByState`, `updateJobState`)는
   재사용하기 쉽도록 `internal/domain/worker/store_sql.go`에 함께 둔다. adapter는 이 함수를 호출만 한다.
3. `cmd/issueops/workercli`와 `cmd/issueops/mcpcli`의 worker 도구가 `workerstate.Store`를 직접 import해 쓴다.

## 적용되는 결정과 주의사항
- `.issueops/CONVENTIONS.md`: 대조했으나 없음
- `.issueops/ARCHITECTURE.md`: 대조했으나 없음

## 재사용하는 기존 구현
- `internal/adapter/outbound/` 아래 기존 sqlite store helper

## 성능 영향
없음. 쿼리 수는 그대로다.

## 하위 호환성과 side effect
worker CLI·MCP 출력 JSON은 그대로다. 저장 위치(user state의 `worker` bucket)도 그대로다.

## 수용 기준
- G1: `go test ./internal/adapter/outbound/workerstate/ ./internal/domain/worker/ -count=1` 통과
- G2: `issueops worker enqueue`/`list`/`cancel` 출력이 변경 전과 같다.
