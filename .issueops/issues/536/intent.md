# 요청자 의도 계약

- lifecycle: io-d103d53f4d1a
- issue: https://github.com/m16khb-org/issueops/issues/536
- intent_class: standard

## 원문 요청
5개작업 $issueops 로 병렬 인계 작업해줘 완료되면 다시 main에 머지하고 $issueops-cleanup 하고 $io-update 까지 진행; 인계작업 세션은 6.1 sol high

## 해석
#536 활성 실행 지침과 self-verify 설명 계약을 현재 구현에 맞춰 정리하고 독립 검증 후 Draft PR과 CI 통과까지 완료한다. 상위 세션이 main 병합, cleanup, io-update를 수행한다.

## 성공 기준
- - [ ] 활성 스킬에서 제거된 패키지·테스트 실행 지시와 hook 기반 권한 차단 설명을 제거하거나 현재 CLI 중심 계약으로 바꿉니다.
- [ ] 상시 감사 지침이 현재 SessionStart catalog 출력과 단일 self-verify 실행 계약을 올바르게 판정합니다.
- [ ] 설치 안내는 in-process MCP 재연결과 legacy proxy 동작을 구분하며 README·runtime 문서와 일치합니다.
- [ ] self-verify 응답 설명이 지원하는 옵션과 실제 반복 횟수를 설명합니다.
- [ ] 수정한 활성 지시만 대상으로 좁은 회귀 검사를 추가하고, 예시 테스트가 이름 일치 0건으로 통과하지 않게 합니다.

## 비목표
- 역사 기록 삭제, legacy daemon 및 hookinput 코드 제거, #534 golden/drift 및 #535 CI 비용 로직 변경

## 제약
- 구현은 canonical worktree의 gpt-6.1-sol/high native Codex owner가 수행한다. 상위 세션만 병합·정리·전역 설치를 수행한다.

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
