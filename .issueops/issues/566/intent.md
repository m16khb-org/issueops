# 요청자 의도 계약

- lifecycle: io-61996bf15223
- issue: https://github.com/m16khb-org/issueops/issues/566
- intent_class: standard

## 원문 요청
[1] https://www.threads.com/share/FNvOor1oF/ 확이;ㄴ해봐
[2] workflowz 이 내용을 바탕으로 isseuops 개선이 가능할까?
[3] /issueops 로 진행
[4] (범위 확인 질문에 대한 답) 5번 제외

## 해석
Anthropic Claude Code 팀의 코드 리뷰 글(https://claude.dev/blog/solving-code-review-at-anthropic/)에서 1인 하네스에 옮길 만한 관행을 스킬 문서 규칙으로 반영한다. issueops-review의 리뷰어 출력 계약에 (1) 필수 결함마다 같은 유형의 다른 발생 위치를 대상 전체에서 찾아 함께 적게 하고, (2) delta 리뷰에서 직전 지적은 해결됨/남음 상태로만 표시하고 새 결함만 새로 보고하게 하고, (3) 변경이 만들지 않은 결함은 기존 결함으로 표시해 비차단 메모로 두게 하고, (4) 버그 수정 사이클의 diff 리뷰에서 근본 원인을 고쳤는지 확인하는 렌즈를 넣는다. issueops-create-pr에는 next.review.tier가 schema-auth 또는 contract인 PR에서 '위험과 되돌리기' 절을 필수로 쓰게 하고, issueops-sync-pr의 낡은 본문 절 표를 artifacttemplate의 정규 PR 절 제목(변경 내용, 확인한 것, 리뷰 포인트, 위험과 되돌리기, 호환성과 마이그레이션 등)으로 맞춘다. 사용자가 CI 워크플로 경로를 schema-auth 등급으로 분류하는 안(5번)은 제외했다.

## 성공 기준
- skills/issueops-review/SKILL.md 출력 계약에 같은 유형 결함의 전체 발생 위치 나열 규칙이 있다
- skills/issueops-review/SKILL.md delta 리뷰 규칙에 직전 지적을 해결됨/남음으로만 표시하고 새 결함만 보고하는 규칙이 있다
- skills/issueops-review/SKILL.md에 변경이 만들지 않은 결함을 기존 결함으로 표시해 비차단 메모로 두는 규칙이 있고 design-review Blocking threshold와 모순되지 않는다
- skills/issueops-review/SKILL.md diff 렌즈에 버그 수정 사이클의 근본 원인 확인 규칙이 있고 CONSTITUTION의 우회 허용 조건과 일치한다
- skills/issueops-create-pr/SKILL.md가 next.review.tier가 schema-auth 또는 contract일 때 '위험과 되돌리기' 절을 요구한다
- skills/issueops-sync-pr/SKILL.md의 본문 절 표 제목이 internal/domain/artifacttemplate/template.go의 PR 절 제목과 일치한다
- python3 scripts/validate-skill.py와 verify-skill-shell.py가 바뀐 스킬 세 개에서 통과하고 self-verify가 성공한다

## 비목표
- CI 워크플로 경로를 change tier에 추가하지 않는다(사용자가 제외, schema-auth 등급은 500줄 초과 diff 분할 외에 동작 차이가 없음)
- 구현 리뷰 이력 보존과 diff 리뷰 라운드 상한의 Go 강제는 record 형식 변경이 필요해 별도 이슈로 둔다
- 병렬 버그 탐색 후 투표, 사람 승인 인원 등급, 머지 큐, 리뷰 횟수 상한을 3회로 낮추기는 하지 않는다
- artifacttemplate의 Required 플래그나 render-template 동작을 바꾸지 않는다

## 제약
- (없음)

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
