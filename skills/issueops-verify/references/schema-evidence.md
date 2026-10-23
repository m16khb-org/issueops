# 스키마 실측 세부

SKILL.md의 `## 2` 절에서 넘어온 세부 규칙이다.

활성화되면 추정이 아니라 **실제 데이터베이스 관찰값**을 요구한다. 대상 테이블의 기존
인덱스 현황과 row 수, 그리고 그 값을 어디서 봤는지다. 조회는 [`database-design`](../../database-design/SKILL.md)
또는 DB MCP 서버로 한다.

커넥션을 소모하는 대형 스캔을 던지지 않는다. `COUNT(*)` 전수 대신 카탈로그의 추정 row
수(`pg_class.reltuples`, `information_schema`, `SHOW INDEX`)를 쓰고 필요하면 `LIMIT`을
건다. 운영 DB에서 무거운 쿼리 하나가 커넥션 풀을 마르게 한다.

```bash
issueops schema-evidence record --id "$ISSUEOPS_ID" \
  --measurement "orders: 8.4M rows(reltuples), idx_orders_user_id 없음" \
  --source "mcp db-bc-prod execute_sql_bc_prod_market" \
  $RECORD_ACTOR_FLAGS --json
```

- measurement와 source는 짝이다. 출처 없는 수치는 추정과 구분되지 않는다.
- 관찰이 불가능하면 `--waive --waiver-rationale "<근거>"`로 남긴다. rationale 없는
  waive는 게이트를 열지 않는다.
- 실측 결과가 구현을 바꿔야 한다면 그것은 이 단계가 아니라 4단계의 일이다. row 수가
  크면 인덱스 생성 전략, 마이그레이션 잠금 시간, 백필 배치 크기가 달라진다.
