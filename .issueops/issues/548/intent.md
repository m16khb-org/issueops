# 요청자 의도 계약

- lifecycle: io-7426b49dc042
- issue: https://github.com/m16khb-org/issueops/issues/548
- intent_class: standard

## 원문 요청
issueops 레포 코드베이스 분석하고 최신화/품질/검증/최적화 가 필요한 내용이 있는지 파악해봐 / (분석 보고 후) 전부 /issueops로 진행 하나의 이슈로 묶어서 진행

## 해석
2026-10-07 코드베이스 분석에서 찾은 최신화·품질·검증·최적화 항목 전부를 한 이슈·한 브랜치·한 PR로 처리한다. 네 묶음: (1) 보안·검증 신뢰성 — go.mod 1.26.6 상향으로 실제 호출되는 stdlib 취약점 7건 해소, 부하에서 깨지는 race 테스트 2건 수정, quality inspect 커버리지 수집 실패 진단성 개선 (2) 문서 드리프트 정리 — OPERATIONS/runtime CLI 목록, --help 누락 하위 명령, 바뀐 테스트·심볼 이름, worker 서술, PROJECT_AUDIT 재대조, ADR 폐기 표시, stability-audit 스킬의 사라진 hook 검사, 코드·golden의 internal/core 잔재 (3) 의존성·CI — 직접 의존성 갱신, CI action 메이저 버전, shellcheck 단계, .gitignore evidence 고정 (4) 코드 품질·성능 — 중복 헬퍼 통합, coverage.go rename 에러 처리, issueopsremote ctx 전파, 테스트 격리(git 전역 설정, 실제 state dir, os.Setenv, glab PATH 가드), 느린 테스트 축소

## 성공 기준
- go.mod가 go1.26.6 이상을 요구하고 govulncheck ./... 가 코드에서 호출되는 취약점 0건을 보고한다
- go test -race ./... -count=1 전체가 통과하고, MCP 동시성 테스트는 ListTools 에러를 panic 없이 보고하며, readability 성능 테스트는 부하 상황에서도 고정 wall-clock 기준으로 실패하지 않는다
- issueops quality inspect의 collection_status가 error가 아니고, 커버리지 수집이 실패하면 실패한 패키지를 evidence에 남긴다
- 분석에서 확인한 문서 드리프트 항목이 코드와 일치하고 --help가 실제 하위 명령·플래그를 모두 나열하며 PROJECT_AUDIT.md가 현재 HEAD 기준으로 재대조된다
- 직접 의존성이 최신 호환 버전이고 go mod tidy -diff가 비어 있으며 CI에 shellcheck 단계와 최신 action 메이저 버전이 반영된다
- printJSON·원자적 쓰기·containsString 등 중복 헬퍼가 공용 구현으로 모이고, issueopsremote가 호출자 ctx를 전파하며, git 테스트가 전역 git 설정과 실제 state dir에서 격리된다
- gofmt·go vet·golangci-lint v2.12.2·architecture ratchet·golden·self-verify가 모두 통과한다

## 비목표
- 로컬 개발 환경에 golangci-lint v2를 설치하는 일 자체(저장소 밖 작업, 하네스는 외부 도구 설치를 대행하지 않음). 문서에 v2 설치 경로만 명시한다
- issueops* 패키지 대규모 병합이나 분기 많은 함수의 구조 재설계
- CLI·MCP 공개 계약(출력 스키마, 도구 이름) 변경

## 제약
- 공개 CLI·MCP 출력 계약 하위 호환 유지
- 외부 도구 설치·readiness gate를 하네스에 추가하지 않음

## 모호함
- resolved: 한 이슈로 묶을지 — 사용자가 하나의 이슈로 명시
- resolved: 로컬 lint v2 — 저장소 밖 설치이므로 문서 안내로 한정(AGENTS.md 철학)
- deferred: quality inspect 커버리지 실패의 정확한 원인 패키지 — 구현 중 재현으로 확정, 방향은 바뀌지 않음

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
