# 요청자 의도 계약

- lifecycle: io-7426b49dc042
- issue: https://github.com/m16khb-org/issueops/issues/550
- intent_class: standard

## 원문 요청
머지 /io-update /issueops-cleanup 후 후속 후보들을 하나의 이슈로 만들어줘 /issueops

## 해석
#548(PR #549) 정비 과정에서 범위 밖으로 남긴 후속 후보를 하나의 이슈로 확정해 발행한다. 이번 요청의 종료점은 이슈 발행이다. 후보: (1) Linux CI의 install 단계가 systemctl --user 부재로 MCP HTTP 서비스를 띄우지 못해 main CI가 10-03부터 실패, (2) codexLifecycleHookEvents에 남은 옛 hook 이벤트, (3) benchmark artifact 예시 문구의 internal/core 경로, (4) readability 성능 테스트가 CPU만 늘어나는 회귀를 못 잡는 문제, (5) 로직이 있는데 커버리지 0%인 패키지, (6) quality inspect가 매번 전체 go test -cover를 돌리는 비용, (7) Python 스위트의 skip 판정 불일치와 로컬 의존성, (8) 표준 라이브러리 관용구 혼용(sort.Slice·os.IsNotExist), (9) 크게 추적되는 에이전트 산출물 문서, (10) 설계 판단이 필요한 원자적 쓰기 통합·분기 많은 함수

## 성공 기준
- main CI의 verify job이 install·self-verify 단계까지 통과한다
- 각 후보에 대해 수정, 근거 있는 유지, 별도 설계 이슈 분리 중 하나의 결론이 기록된다
- 수정한 후보마다 재현 또는 측정 근거와 focused 검증이 남는다

## 비목표
- CLI·MCP 공개 출력 계약 변경
- issueops* 패키지 대규모 병합
- PROJECT_AUDIT에서 근거와 함께 범위 밖으로 결론 난 CP3·W6 재개

## 제약
- (없음)

## 모호함
- resolved: 하나의 이슈로 묶음 — 사용자가 명시
- deferred: CI install 실패의 해결 방향(CI에서 stdio transport 사용, systemd 부재 시 대체 경로, CI user systemd 준비) — 구현 계획에서 하네스 철학(독립 설치, readiness gate 금지)과 대조해 정함
- deferred: 원자적 쓰기 통합과 분기 많은 함수 재설계는 설계 판단이 먼저라 이 이슈에서 결론만 내고 구현은 분리할 수 있음

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
