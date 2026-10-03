# 구현 실패 예시

[본문](../SKILL.md)의 규칙을 설명하는 선택적 예시다. 실행 계약은 본문에 있다.

| 나쁜 행동 | 문제 |
|---|---|
| `next` 없이 phase 추정 | 단계 판별은 CLI가 소유한다 |
| "3줄 수정이니까" source checkout 수정 | canonical worktree와 readiness HEAD가 어긋난다 |
| 구두 확인만으로 `replace --revoke --confirm` | quiescence 증거가 아니다 |
| direct에 `resume`, Orca에 임의 `claim` | 반환된 `next_command`를 따라야 한다 |
| timeout 뒤 prepare/create 재실행 | reconcile 없이 이중 mutation을 만들 수 있다 |
| 구현 직후 커밋·푸시 또는 정리/리뷰 기록 | 뒤 단계의 변경이 봉인을 stale로 만든다 |
| 운영 DB `SELECT COUNT(*)` | 카탈로그 추정치 대신 전수 스캔으로 커넥션을 소모한다 |
| 실측 없이 schema evidence 기록 | 관찰 불가면 waive 근거를 적어야 한다 |
| implement 직후 `execution complete` | pr phase와 검증된 remote artifact가 필요하다 |
| 구두 승인으로 child scope 확대 | 새 child 분리 또는 plan 개정과 review freshness 확인이 필요하다 |
| `--help` 성공만으로 명령 존재 단정 | usage 카탈로그와 소스를 확인한다 |
