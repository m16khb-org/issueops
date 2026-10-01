# 요청자 의도 계약

- lifecycle: io-339cd3178172
- issue: https://github.com/m16khb-org/issueops/issues/533
- intent_class: standard

## 원문 요청
5개작업 $issueops 로 병렬 인계 작업해줘 완료되면 다시 main에 머지하고 $issueops-cleanup 하고 $io-update 까지 진행

## 해석
#533 하위 이슈 중복 생성 결함을 격리된 native 세션에서 구현하고 검증된 PR까지 완료한다. Root가 모든 이슈를 main에 병합하고 cleanup 및 io-update를 완료한다.

## 성공 기준
- - [ ] URL+오류, timeout, 연결 단절에서 자동 두 번째 create가 발생하지 않습니다. 미지원 `--parent`의 안전한 대체 경로는 유지합니다.
- [ ] 원격 성공 후 로컬 연결 기록 실패·프로세스 중단을 재현하고 같은 요청의 복구가 동일 URL을 사용함을 확인합니다.
- [ ] 복구되지 않은 요청은 새 생성 전에 차단하며, 알려진 URL과 복구 명령을 CLI/MCP에 제공합니다.
- [ ] GitHub·GitLab 양쪽에서 생성 요청의 영속 식별과 복구를 검증합니다. 제목이 같다는 이유만으로 서로 다른 의도적 생성을 막지 않습니다.
- [ ] preview는 쓰지 않고, 정상 생성의 계층·라벨·담당자 검증은 유지합니다.

## 비목표
- 별도 이슈인 process bounds, self-verify, CI 비용, 전체 문서 정리는 수정하지 않는다.

## 제약
- 이 native owner는 #533만 수정한다. global install, main merge 및 cleanup은 root만 수행한다.

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
