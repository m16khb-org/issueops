# HTTP 설치 검증과 린터 toolchain 일치

[주의사항 인덱스](../CAUTIONS.md)

## Kind

검증기의 설치 계약 drift와 개발 도구 버전 불일치.

## Source

하네스 개선 후 남은 self-verify 및 lint 실패를 직접 재현했다.

## Summary

실제 설치가 HTTP로 바뀌어도 검증기가 stdio의 command/args/env만 검사하면
정상 설치를 누락으로 판정한다. 린터의 빌드 Go와 분석 대상 GOROOT가 다르면
프로젝트 코드가 아니라 표준 라이브러리에서 typecheck가 실패할 수 있다.

## Context

- `hasCanonicalOmoMCP`가 `~/.omo/mcp.json`의 HTTP entry를 거부했다.
- 과거 검증 보고서의 식별 경로와 합성 인증 입력이 fixture/redaction 검사에 걸렸다.
  입력이 합성이어도 검증용 원문을 일반 문서에 복사하면 안 된다.
- v1 설정과 v2 바이너리가 충돌했고, CLI flag로 우회한 실행도 Go 1.27.1
  표준 라이브러리를 Go 1.26.3 기반 analyzer가 읽지 못해 실패했다.

## Resolution

- 기존 stdio 판정을 유지하며 loopback HTTP endpoint와 Bearer 형식을 검증한다.
  remote host, 잘못된 path·port·인증, HTTP와 command가 섞인 설정은 거부한다.
- 과거 보고서는 실행 결과를 보존하고 개인 경로를 `$WORKSPACE`로 정규화했다.
  인증 입력 원문은 제거했다. fixture/redaction 검사는 완화하지 않았다.
- CI와 로컬은 golangci-lint v2.12.2를 사용한다. 로컬 실행의 GOTOOLCHAIN은
  go.mod에서 선택하며 명령은 [테스트 규칙](../testing/unit-and-contract.md)이 소유한다.
- v1의 staticcheck(SA)·gosimple(S1) 검사 범위를 명시해 v2의 stylecheck·quickfix
  자동 추가와 구분한다. 기존 제외 preset은 공식 migrate 결과를 유지한다.
  실제 errcheck·deprecated API·unused 지적은 코드와 테스트에서 해결한다.

## Evidence

- `internal/adapter/verification/probe/nativeintegration/omo_http_test.go`:
  기존 구현에서 정상 HTTP 두 사례가 실패했고 수정 후 통과했다.
- `scripts/meeting_notes_skill_contract_test.py`: 식별 정보 제거 후 기존 검사가 통과했다.
- `.golangci.yml`, `.github/workflows/ci.yml`: 같은 v2 버전과 기존 검사 범위를 사용한다.
- 전체 run 결과는 [후속 검증 보고서](../research/2026-10-03-harness-improvement/remaining-fixes.md)에 기록한다.

이 기록의 장애 설명은 발생 당시 근거다. 현재 실행 명령은 정규 테스트 문서를 따른다.
