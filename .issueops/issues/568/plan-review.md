# 계획 검토 기록

## 1차: 수정 요청

- 1라운드 리뷰어(anthropic/claude-opus-5-5): basename 사슬만 보는 receipt 규칙은 공유 서비스를 띄운 일반 TUI가 살아 있으면 통과하므로 argv(serve --stdio) 판정이 필요하다.
- 선언 host를 바꾼 명시적 ACTOR_FLAGS로 opencode receipt 규칙을 우회할 수 있으므로 규칙 키를 receipt 실행 파일로 잡아야 한다.
- 공유 서비스 세션의 MCP 사용 범위를 코드와 다르게 적었고(harness_inspect 분류, stdio 경로), 이슈 계약 변경 기록이 늦다.
- G10 live 테스트가 구현 세션의 omp 세션 ID와 사용자 PATH를 물려받아 ambiguous가 되고, opencode updater를 끄는 env가 빠졌다.
- bin을 쓰는 게이트 앞에 빌드가 없다.
- claim 성공, 스킬 중복, 세션당 1회 실행 같은 성공 기준을 관측하는 게이트가 없다.
- JSONC 주석 제거가 문자열 안의 // 를 깨뜨려 정상 사용자 파일에서 모든 host 설치를 막을 수 있다.
- host-probe runner가 SessionStart 증거와 도구 제한을 정하지 않았다.

## 2차: 수정 요청

- 2라운드 delta 리뷰: 1라운드 지적 8건 중 7건 해결, claim 성공 관측 1건은 남음.
- live 테스트가 actor 플래그 없이 mcp authorize를 불러 항상 실패하므로 whoami의 claim_actor_flags를 붙여야 한다.
- opencode run의 제목 생성 요청은 catalog와 도구가 없어 mock의 요청 번호와 모든 요청 표지 단언이 깨진다.
- probe runner의 private serve에 준비 대기 상한과 모든 종료 경로의 정리가 없다.

## 3차: 수정 요청

- 3라운드 delta 리뷰(상향 모델 anthropic/claude-opus-5-5 xhigh, omp -p 빈 컨텍스트): 2라운드 지적 3건은 모두 해결됐다.
- argv 판정이 프롬프트 안의 opencode serve --stdio 문자열로도 참이 될 수 있어 첫 opencode 토큰으로 위치를 고정해야 한다.
- T1 QA의 결합 규칙 오류가 ancestry 포함 검사에 먼저 막히므로 규칙 위치를 ancestry 검사 앞으로 정해야 한다.
- probe runner의 private serve가 cwd 없이 시작돼 사용자 repo를 기본 location으로 읽을 수 있다.

## 4차: 수정 요청

- 4라운드 delta 리뷰(anthropic/claude-opus-5-5 xhigh, omp -p 빈 컨텍스트): 3라운드 지적 3건은 모두 해결됐다.
- 결합 규칙이 internal/domain/issueops에서 internal/domain/nativehost를 import하면 foundation owner 의존 규칙(domain_must_only_import_contract) 테스트가 실패한다. 리뷰어가 복사본에서 실측했다. 판정 함수를 internal/domain/issueops 안으로 옮겨야 한다.

## 5차: 통과

- 5라운드 delta 리뷰(anthropic/claude-opus-5-5 xhigh, omp -p 빈 컨텍스트) 통과. 판정 함수를 internal/domain/issueops에 두는 수정은 리뷰어가 복사본에 계획대로 구현해 architecture 테스트(inventory 재생성 뒤)와 foundation owner 규칙이 통과함을 실측했다.
- 리뷰어가 계획의 receipt 판정 표(허용 2행, 거부 5행, verifyNative 6행)를 복사본 임시 테스트로 모두 재현했고, 기존 테스트 회귀는 계획의 갱신 목록에 있는 오류 문구 고정 테스트 하나뿐이었다.
- 비차단 메모: G1 패키지 목록의 nativehost는 이름 있는 ExecutableMatchesHost opencode 테스트를 더해 no tests to run을 없애고, 세 곳의 opencode 실행 파일 판정 정규화와 T9 env의 LC 고정 목록은 구현과 diff 리뷰에서 맞춘다.
