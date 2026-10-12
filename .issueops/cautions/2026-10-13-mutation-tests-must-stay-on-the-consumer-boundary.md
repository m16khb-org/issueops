---
name: 2026-10-13-mutation-tests-must-stay-on-the-consumer-boundary
description: Caution record for a solved false case or recurring risk.
---

# Mutation-killing tests must stay on the consumer boundary

- Date: 2026-10-13
- Kind: `caution`
- Source: quality autoresearch session (gremlins mutation segments 3-17)
- Summary: 살아남은 변이를 죽이려고 쓴 테스트는 같은 결함을 반복했다. 호출자가 만들 수 없는 입력을 쓰거나, 공개 진입점 대신 비공개 함수를 부르거나, panic으로만 실패했다. 또 gremlins v0.6.0은 모듈 이름과 같은 디렉터리(`issueops/`) 아래의 변이를 모두 NOT COVERED로 잘못 보고한다.
- Context: 세그먼트마다 리뷰어가 6~13건을 지적했는데, 유형이 거의 같았다.
  - 입력 범위 위반: MCP 스키마 검증이 먼저 거부하는 인자, `ValidateIssueURL`이 거부하는 스킴 없는 issue URL, lsof 파서가 버리는 PID 0, 어떤 호출자도 넘기지 않는 빈 git argv, `ValidateExecution`이 거부하는 Direct 실행과 Orca 바인딩 조합.
  - 경계 우회: 포트 메서드(`ReplacementRecords.Persist`, `OwnerContextFiles.CreateOrAdoptToken`, `Command.RunGuard`, `mcpcli.NewRequestScope`)가 있는데 그 안의 비공개 함수를 직접 호출했다.
  - 실패하지 않는 단언: nil 역참조 panic으로만 변이를 잡거나, 오류 경로에서 호출자가 버리는 반환값을 단언했다.
  - 이름 붙은 상한을 숫자로 다시 적거나(`regressionFixtureLimit`를 `64 << 10`으로), 문서에 없는 휴리스틱(2 MiB 스캔 상한, 30초 timeout, AC 번호 최소 자릿수)을 계약처럼 고정했다.
  - gremlins `internal/coverage/coverage.go:203`은 `strings.ReplaceAll(p.FileName, mod+"/", "")`로 모듈 접두사를 지운다. 그래서 `issueops/internal/domain/issueops/x.go`에서 뒤쪽 `issueops/`까지 지워지고, `domain/issueops`, `adapter/issueops`, `cmd/issueops/*`의 커버리지가 전부 사라졌다.
- Resolution:
  - 변이 대응 테스트는 호출자가 실제로 쓰는 진입점(포트 메서드, CLI `Run`, MCP 도구 디스패치, 공개 생성자)을 거치고, 호출자의 검증을 통과하는 입력만 쓴다. 이 조건을 지키면 잡을 수 없는 변이는 사유를 남기고 살려 둔다.
  - 단언은 변이가 들어가면 assertion으로 실패해야 하고, panic에 기대지 않는다. 이름 붙은 상한은 상수를 그대로 참조한다.
  - 테스트, 헬퍼, 픽스처 이름에는 `mut` 같은 작업 출처를 붙이지 않고 동작을 쓴다.
  - 수동 변이로 증명할 때는 `git checkout -- <file>`로 되돌린다. 같은 패키지에서 여러 에이전트가 동시에 운영 코드를 바꾸면 서로의 테스트 실행이 잠깐 실패하므로, 판단 전에 다시 돌린다.
  - gremlins는 위 한 줄을 `strings.TrimPrefix`로 바꾼 사본을 빌드해 쓴다. `--exclude-files` 정규식은 패키지 기준 상대 경로에 적용되므로 `'^[a-z]+/'`로 하위 패키지를 빼고 상위 패키지만 측정할 수 있다. gremlins는 하위 디렉터리도 함께 변이하므로 상위와 하위를 같은 집합에 넣지 않는다.
- Evidence:
  - internal/adapter/issueops/execution_lease_boundaries_test.go (`ReplacementRecords.Persist`, `replacementProcessSnapshot.Inspect` 경유)
  - cmd/issueops/mcpcli/mcp_tool_worker_policy_state_boundaries_test.go (스키마를 통과하는 인자로 -32000 경로 고정)
  - internal/adapter/toolconformance/replay_internal_test.go (`regressionFixtureLimit` 참조)
- Alternatives / rejected options:
  - 살아남은 변이를 모두 죽일 때까지 테스트 추가: 결과가 같은 변이와 도달할 수 없는 경로까지 고정하게 되어, 정당한 리팩터링을 막는 테스트가 생긴다
  - gremlins를 쓰지 않고 `issueops/` 아래 패키지 측정을 포기: 저장소에서 가장 큰 패키지들이 빠지므로 채택하지 않았다
