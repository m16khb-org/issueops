---
name: 2026-10-08-user-systemd-manager-base-ci
description: Caution record for a solved false case or recurring risk.
---

# user systemd manager 경로 조회와 base 커밋에서 거짓 통과하는 CI 게이트

- Date: 2026-10-08
- Kind: `caution`
- Source: issueops-docs #552
- Summary: systemd user manager의 unit 검색 경로는 manager에게 물어야 하고(systemd-analyze는 호출 프로세스 환경으로 계산한다), HEAD의 CI run을 읽는 게이트는 첫 커밋 전에는 base 커밋의 run을 읽어 거짓으로 통과한다.
- Context: #552(2026-10-08)에서 HOME이 다른 Linux HTTP 설치를 쓰기 전에 거부하도록 고치면서 세 가지를 만났다. (1) 계획 1라운드는 systemd-analyze --user unit-paths로 검색 경로를 보려 했는데, 공식 문서상 이 명령은 실행 중인 manager와 통신하지 않고 호출 프로세스의 환경으로 경로를 계산한다. HOME을 바꾼 설치에서는 바뀐 HOME 경로가 그대로 나와 불일치를 감지하지 못하고, fake Runner 테스트도 이를 가린다. systemctl --user show -p UnitPath --value는 manager의 실제 목록이지만 아직 없는 디렉터리를 빼므로 같은 HOME의 첫 설치에서 거짓 불일치가 난다. systemctl --user show-environment는 특수문자가 든 값을 $'...'로 이스케이프해 출력한다. (2) 게이트 원장의 'HEAD의 PR CI run이 green' CHECK가 구현 진입 직후 실행에서 CI_GREEN으로 충족됐다. 아직 커밋이 없어 HEAD가 base 커밋이었고, main에서 이미 통과한 run을 읽은 것이다. (3) unittest stderr 전체에서 'skipped'를 찾는 CHECK가 테스트 이름과 로그의 같은 단어에 걸려 skip 0인데도 실패했다. (4) 로컬 게이트 8개가 모두 통과했지만 darwin에서만 돌았다. Linux 배선 테스트(TestSupervisorUnitRunsAbsoluteBinaryWithExplicitRootAndState)는 임시 HOME에서 실제 systemctl로 Prepare를 부르므로, user manager가 있는 CI runner에서는 새 거부에 걸려 실패할 상황이었다. 구현 리뷰 2라운드가 찾았다.
- Resolution: (1) manager의 환경(show-environment의 HOME·XDG_CONFIG_HOME)으로 기대 디렉터리를 계산하고, 다를 때만 UnitPath 목록으로 일치를 확인한다. UnitPath는 일치 확인에만 쓰고 불일치의 근거로 쓰지 않는다. 조회 실패, HOME 부재, $'...' 값은 판정 불가로 보고 막지 않는다(readiness gate 금지). 외부 도구 출력을 fake로 테스트할 때는 그 도구가 실제로 무엇을 근거로 답하는지 문서로 먼저 확인한다. (2) HEAD의 CI run을 읽는 게이트는 원장으로 채울 수 없다. 첫 커밋 전에는 base의 run을, 커밋 뒤에는 원장 커밋 이전 HEAD의 run을 읽기 때문이다. 처음부터 abandon하고 push 뒤 PR 발행 전에 run URL로 확인한다. 이미 충족으로 찍혔으면 체크를 풀고 EVIDENCE를 pending으로 되돌린 뒤 abandon 사유에 그 사실을 적는다. (3) skip 판정은 unittest 요약의 '(skipped=' 표기로 한다. (4) GOOS로 갈리는 경로를 바꾸면 그 OS의 테스트를 실제로 돌린다. darwin에서는 `GOOS=linux go test -c`로 테스트 바이너리를 만들어 linux 컨테이너에서 실행하고, 외부 manager의 응답은 PATH 앞의 가짜 명령으로 흉내 낸다. 실제 manager에 닿는 테스트는 manager에 닿지 않는 가짜 systemctl을 PATH 앞에 둬 격리한다.
- Evidence:
  - internal/adapter/mcpservice/supervisor.go (systemd.checkUnitPath)
  - internal/adapter/mcpservice/service_test.go (TestPrepareChecksTheUserManagerUnitSearchPath, 8 subtest)
  - .issueops/issues/552/gates.md (G5 CHECK 수정, G8·G9 abandon 사유)
  - .issueops/issues/552/plan-review.md (1라운드 revise: systemd-analyze가 manager와 통신하지 않음)
  - .github/workflows/ci.yml (HTTP MCP install refuses a HOME the user systemd manager does not search)
  - cmd/issueops/issueopsapp/mcp_service_wiring_test.go
