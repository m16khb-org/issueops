# 요청자 의도 계약

- lifecycle: io-7426b49dc042
- issue: https://github.com/m16khb-org/issueops/issues/532
- intent_class: standard

## 원문 요청
5개작업 $issueops 로 병렬 인계 작업해줘 완료되면 다시 main에 머지하고 $issueops-cleanup 하고 $io-update 까지 진행

## 해석
정책 명령 실행기의 자식 프로세스 종료와 출력 메모리 상한을 구현하고 검증한 뒤 draft PR을 게시한다. root가 최종 머지, cleanup, io-update를 담당한다.

## 성공 기준
- - [ ] 허용 명령이 띄운 자식과 stdout/stderr 상속 사례에서 timeout+유예 안에 반환하고, 반환 뒤 자식의 완료 파일이 생기지 않습니다.
- [ ] stdout·stderr 대량 출력에서 보관량이 명시된 상한을 넘지 않으며, 전체 출력 크기에 비례한 버퍼·문자열 복사가 없어집니다.
- [ ] 출력 경계에 걸친 비밀값과 UTF-8 처리를 검증하고 기존 응답 형태를 유지합니다.
- [ ] 정상 종료, 비정상 종료, timeout, 정책 거부를 실제 프로세스로 검증합니다. mock의 `TimedOut=true`만 확인하는 테스트로 대체하지 않습니다.
- [ ] 실행기 전체를 합치는 추상화는 필요성을 입증한 경우에만 도입합니다. 수정 범위는 정책 실행 경계와 직접 연결된 테스트입니다.

## 비목표
- 별도 runner 전체 통합, 관련 없는 네 작업 변경, owner의 전역 설치는 하지 않는다.

## 제약
- 이 owner는 #532 canonical worktree만 수정하고 검증된 draft PR 및 최신 push/PR CI 성공과 execution complete/release까지 수행한다. root가 main 머지, cleanup, io-update를 진행한다.

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
