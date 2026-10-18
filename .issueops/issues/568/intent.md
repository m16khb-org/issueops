# 요청자 의도 계약

- lifecycle: io-7426b49dc042
- issue: https://github.com/m16khb-org/issueops/issues/568
- intent_class: standard

## 원문 요청
opencode를 omp처럼 issueops에 완전통합 하고 싶어
(질문 답변) 진행 방식: IssueOps 사이클
(질문 답변) lease 소유: --standalone 세션만 허용

## 해석
opencode v2(2.0.32)를 codex, claude, omo, omp에 이은 다섯 번째 first-party host로 추가한다. omp ADR(2026-10-08)과 role model ADR(2026-10-12)이 정한 모든 통합 면 (사용자 범위 MCP·plugin·스킬 설치와 readback 검증, project-doc catalog 주입, native actor·process receipt, role model 설정, print argv, Herdr·cmux·worktree 실행 profile, trace 입력 형식, host-probe runner, MCP host enum·golden·문서)을 opencode에도 같은 깊이로 제공한다. opencode v2는 공유 백그라운드 서비스가 모든 셸을 띄우므로 IssueOps lease 변경은 --standalone 세션의 전용 serve --stdio 프로세스를 receipt로 쓰는 경우에만 받고 공유 서비스 세션은 fail closed한다. 이미 제거된 Orca 면은 대상이 아니다.

## 성공 기준
- 임시 HOME 설치와 dry-run이 opencode MCP(mcp.servers.issueops, codemode:false)·plugin을 기록·검증하고 기존 opencode 설정은 보존된다; opencode 세션에서 issueops 스킬이 이름마다 한 번, MCP 도구가 issueops_* 이름으로 노출·호출된다; 새 세션과 압축 뒤 모델 요청에 catalog가 있다; --standalone 셸의 whoami가 host=opencode와 OPENCODE_SESSION_ID를 반환하고 claim이 성공하며 공유 서비스 셸의 claim은 구조화된 오류로 거부된다; model show가 다섯 host를 보이고 model set --host opencode가 prepare·리뷰 모델에 반영된다; 실행 profile argv가 2.0.32 --help의 플래그만 쓴다; trace --input-format opencode-json이 step_finish 토큰을 합산한다; golden과 omo·omp·agy 템플릿 digest가 재생성된다; ADR과 현행 문서·스킬이 다섯 host를 말한다; gofmt·vet·test·race·build·self-verify가 통과한다.

## 비목표
- (없음)

## 제약
- (없음)

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
