# 계획 검토 기록

## 1차: 수정 요청

- 1차(anthropic/claude-opus-5-5, high, 전체 리뷰): 감사 파일에 남은 orca 런처 줄 86개 때문에 런처 값만 지우면 cmux 인계와 manual claim 관측이 읽기 단계에서 실패한다. 이번 lifecycle에도 옛 줄이 있다.
- 대소문자 무시 orca 검사가 Orca와 무관한 이름(ForCandidate, ErrorCamel 등 18개 파일)에도 걸리는데 계획에 이름 변경 범위가 없었다.
- 새 prepare가 사유 없는 direct 영수증을 쓰면 옛 검증기가 거부하므로 롤백 주장이 결정과 모순됐다.
- orca 단어가 없는 지운 플래그·명령 이름의 잔여(owner_command, gitlab-usecase 스킬, SUB_AGENT_PATTERNS, owner 프롬프트 문서)를 찾는 단계가 없었다.
- G5 회귀 테스트가 지우는 Orca fixture로 만들어져 있어 그대로 재사용할 수 없었다.

## 2차: 수정 요청

- 2차(anthropic/claude-opus-5-5, high, 사용자 방향 변경 뒤 전체 리뷰): 감사 파일 전체 읽기는 lifecycle과 무관하게 orca 줄 하나로 실패하므로, 이 기기의 모든 사이클에서 cmux 인계가 막히고 manual claim 관측이 빠진다는 영향 서술이 계획에 빠져 있었다.
- G4는 sync-base 이벤트의 mode 필드에, G9는 cleanup이 계속 쓰는 provider issue_snapshot 파일 경로에 걸려 어떤 구현으로도 통과할 수 없었다.
- 완료 기준 4의 인계 체인 smoke가 게이트로 올라가 있지 않았다.
- 롤백 뒤 abandon하고 다시 prepare하라는 절차는 옛 바이너리가 새 기록을 읽지 못해 실행할 수 없었다.

## 3차: 수정 요청

- 3차(anthropic/claude-opus-5-5, xhigh 상향, delta 리뷰): 2차 지적 네 건은 해소됐다. 고친 G4의 requested_mode 토큰이 tool conformance 보고서의 requested_model 필드에 걸려 통과할 수 없었다.
- G9가 남기기로 한 Orca orchestration caution 모듈의 resolved_mode·preserve-terminal 문구에 걸렸고, 그 모듈에는 Orca와 무관하게 유효한 두 절이 섞여 있었다.

## 4차: 수정 요청

- 4차(anthropic/claude-opus-5-5, xhigh 상향, delta 리뷰): 3차 지적은 해소됐다. Orca orchestration caution 모듈을 지우면 다른 cautions 모듈 두 곳의 링크와 architecture·documentation 문서의 참조가 끊기는데, 계획은 그 참조 수정과 삭제 뒤 배치 검사를 담지 않았다.

## 5차: 통과

- 5차(anthropic/claude-opus-5-5, xhigh 상향, delta 리뷰): orchestration caution 모듈 삭제로 끊기는 네 참조와 삭제 뒤 배치 검사 게이트가 계획에 들어갔다. 수정 후 상태를 메모리에서 흉내 내 배치 검사기를 돌린 결과 위반이 0건이어서 통과했다.
- 새 게이트가 계획이 남기는 문서·과거 기록·manifest 소유 항목과 충돌하는지 공격했지만, 지우는 문서를 Markdown 링크로 가리키는 곳이 계획의 수정 대상뿐이고 옮길 절이 모듈 예산 안에 들어가 충돌이 없었다.
- 비차단 메모: 현행 cautions 모듈(issueops-lifecycle, issueops-execution, audit-and-process)에 남는 Orca 동작 설명과 prompt-engineering manifest 설명 한 줄은 구현 단계에서 함께 고치고, 그 확인 게이트를 4단계 원장에 추가한다.
