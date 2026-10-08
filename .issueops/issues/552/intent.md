# 요청자 의도 계약

- lifecycle: io-7426b49dc042
- issue: https://github.com/m16khb-org/issueops/issues/552
- intent_class: standard

## 원문 요청
찾은 결함들을 다 해결하는 이슈를 만들고 /issueops 로 진행

## 해석
#548·#550 사이클에서 찾고 범위 밖으로 남긴 결함 세 가지를 한 이슈로 해결한다. (1) Linux HTTP MCP supervisor: installer가 unit을 install의 HOME 아래(.config/systemd/user)에 쓰는데 이미 실행 중인 user systemd manager는 자기 HOME 기준 경로에서 unit을 찾아, HOME이 다른 설치에서 enable이 'Unit file ... does not exist'로 실패한다. (2) pr-review mr_context.py가 grep 대체 경로로 찾아도 출력 머리에 'source: rg'로 표시한다. (3) grep 대체 경로가 .gitignore를 따르지 않아 무시 대상 산출물이 결과에 섞일 수 있다. 종료점은 draft PR 발행과 execution complete다.

## 성공 기준
- HOME이 user systemd manager와 다른 Linux 설치에서 HTTP transport 설치가 성공하거나, 변경 전에 원인을 명시한 오류로 실패한다(조용한 stdio 전환 없음, ADR 2026-10-02 유지)
- 같은 HOME의 기존 Linux HTTP 설치와 macOS launchd 설치 동작이 바뀌지 않는다
- pr-review 출력의 source 표시가 실제로 쓴 검색 경로(codegraph, rg, git grep, grep)와 일치한다
- rg가 없을 때의 검색이 git 저장소에서는 .gitignore로 무시되는 파일을 결과에 넣지 않는다
- 각 결함은 실패하는 테스트로 재현한 뒤 고치고, gofmt·go vet·golangci-lint·race·Python 스위트·self-verify와 PR CI가 통과한다

## 비목표
- Linux 기본 MCP transport 정책 변경, supervisor 실패 시 stdio 자동 전환
- CLI·MCP 공개 출력 계약 변경
- pr-review의 rg 경로 동작 변경

## 제약
- (없음)

## 모호함
- resolved: 대상 결함 범위 — #551 PR의 남은 일 두 건과 리뷰 포인트의 .gitignore 한계
- deferred: supervisor 수정 방향(unit을 manager가 찾는 경로로 연결 vs 불일치를 사전 감지해 명시 오류) — 계획 단계에서 ADR과 격리 요구(임시 HOME 설치가 실제 계정 서비스에 영향을 주는지)로 정함

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
