# 계획: `issueops status --watch` 추가 (#61)

## 문제
구현 세션이 도는 동안 다른 터미널에서 사이클 상태 변화를 보려면 `issueops status --id ID`를 손으로 다시
실행해야 한다.

## 변경
1. `cmd/issueops/issueopscli/issueops_subcommands.go`의 `runIssueOpsStatus`에 `--watch` 플래그와
   `--interval` 플래그(기본 2초)를 더한다.
2. `--watch`면 `cli.Runtime.IssueOpsStatus`를 interval마다 다시 호출해 출력한다. 사람 출력은 화면을 지우고
   다시 그리고, `--json`이면 한 번에 JSON 객체 한 줄(NDJSON)을 출력한다.
3. 루프는 사용자가 Ctrl-C로 끝낼 때까지 돈다. SIGINT를 받으면 exit 0으로 끝낸다. 상태 조회가 실패하면
   오류를 출력하고 다음 interval에 다시 시도한다.

## 적용되는 결정과 주의사항
- `.issueops/conventions/cli-mcp-and-output.md` / CLI 컨벤션: "`--json` 출력 field는 snake_case를 사용한다." → NDJSON 각 줄은 기존 `status --json` 객체 그대로다.
- `.issueops/adr/2026-05-25-plugin-vs-external-worker.md`: "worker daemon은 장기 작업·상태 공유·watch가 실제로 필요해진 뒤 추가한다." → 상주 daemon 없이 이 CLI 프로세스 안의 foreground 루프로만 구현한다.
- `.issueops/ARCHITECTURE.md`: 대조했으나 없음(새 패키지·레이어 이동 없음, cmd 계층의 flag 처리만 바뀜).
- `.issueops/CAUTIONS.md`: 대조했으나 없음.
- `.issueops/TESTING.md` / Go 코드 변경 기본 검증 → G3, G4.

## 재사용하는 기존 구현
- 조회는 기존 `cli.Runtime.IssueOpsStatus`, 출력은 기존 `printIssueOpsResult`를 그대로 쓴다. 새 조회 경로는 만들지 않는다.

## 성능 영향
2초마다 상태 저장소를 한 번 읽는다. 읽기 전용이고 lock을 잡지 않는다.

## 하위 호환성과 side effect
- 새 플래그만 더한다. `--watch` 없는 `status`의 출력·exit code는 그대로다.
- 상태를 쓰지 않는다.

## 수용 기준
- G1: watch 루프 테스트 | CHECK: go test ./cmd/issueops/issueopscli/ -run TestStatusWatchPrintsEachInterval -count=1 -v | EXPECT: --- PASS: TestStatusWatchPrintsEachInterval
- G2: NDJSON 테스트 | CHECK: go test ./cmd/issueops/issueopscli/ -run TestStatusWatchJSONIsOneObjectPerLine -count=1 -v | EXPECT: --- PASS: TestStatusWatchJSONIsOneObjectPerLine
- G3: 패키지 테스트 | CHECK: go test ./cmd/issueops/issueopscli/ -count=1 | EXPECT: ok
- G4: usage golden | CHECK: go test ./cmd/issueops/contractgolden -run Golden -count=1 | EXPECT: ok
