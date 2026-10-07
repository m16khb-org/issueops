---
name: 2026-10-07-helper-cover-home
description: Caution record for a solved false case or recurring risk.
---

# 테스트 helper 프로세스와 호스트 상태가 -cover·부하·실제 HOME에서 결론을 바꾼다

- Date: 2026-10-07
- Kind: `caution`
- Source: #548
- Summary: go test -cover, 병렬 race 부하, 개발자 HOME이 같은 테스트의 결과를 바꿨다. #548에서 세 경우를 재현하고 고쳤다.
- Context: 1) internal/adapter/policy의 helper는 테스트 바이너리를 재실행하는데, CommandExecutor가 env를 allowlist로 거르므로 커버리지 빌드 helper가 종료 시 stderr에 'GOCOVERDIR not set' 경고를 찍었다. stderr를 정확히 비교하는 테스트가 go test -cover에서만 실패했고 quality inspect가 collection=error가 됐다. 2) doctor 계열(status, doctor, TestCLIHelpers)이 실제 ~/.claude.json의 loopback MCP gateway를 HTTP probe하고 lsof로 FD를 세어 호출당 약 2.6초가 걸렸고 결과가 머신 상태에 묶였다. 3) MCP 동시성 테스트가 80개 세션마다 고정 10초 컨텍스트를 썼는데, 세션이 CPU를 나눠 쓰므로 패키지 병렬 race 실행에서 만료돼 ListTools 에러 뒤 nil 결과를 읽고 panic했다(GOMAXPROCS=1 인스턴스 12개 동시 실행으로 12/12 재현).
- Resolution: 커버리지 빌드(testing.CoverMode()!="")일 때만 helper 요청에 임시 GOCOVERDIR과 EnvAllowlist를 넘긴다. doctor를 부르는 테스트는 HOME을 임시 디렉터리로 둔다(statuscli TestMain, TestCLIHelpers t.Setenv). 동시 세션 테스트는 세션별 고정 타이머 대신 t.Deadline에서 여유를 뺀 컨텍스트 하나로 묶고, 에러를 결과보다 먼저 확인한다. quality inspect의 coverage warning은 이제 실패 패키지를 이름으로 남기므로 collection=error면 그 패키지를 go test -cover로 단독 재현한다.
