# 계획 검토 기록

## 1차: 수정 요청

- 리뷰 모델 anthropic/claude-opus-5-5 high(scout 서브에이전트 기본 실행). 잔여 이름 게이트 G4는 대소문자를 무시해 design review를 찾는데, 응답 계약 스냅숏 테스트의 검증 문자열 design review checked contract drift risk를 바꾸는 항목이 계획에 없어 golden 재생성 뒤에도 남으므로 G4가 0건이 될 수 없다.
- 계획은 위임 자식이 부모 리뷰를 승계하면 아키텍처 스테이징 요구와 stale 판정을 건너뛴다고 정했지만, 이 경로를 관측하는 게이트나 테스트가 없어 예외가 빠져도 자식 사이클이 plan.architecture에 갇히는 회귀를 잡지 못한다.

## 2차: 수정 요청

- 2라운드 delta 리뷰(같은 리뷰 모델). 스냅숏 테스트 문자열 지적은 해결됐다. 위임 자식 예외는 분류 측 게이트만 생겼고, 부모 리뷰를 복사하는 생산 코드가 복사본의 reviewer_context를 delegated-parent-review로 바꾸는 TODO와 그 값을 단언하는 게이트가 없어 실제 위임 자식은 여전히 plan.architecture에 갇힌다.

## 3차: 수정 요청

- 3라운드 delta 리뷰, 상향 모델 anthropic/claude-opus-5-5 xhigh(omp -p 빈 컨텍스트 세션). 위임 자식의 생산 측 변경과 G10으로 이전 지적은 해결됐다. 다만 부모 승계 표시 상수를 다른 capability의 contract 패키지로 옮기면 domain이 다른 capability의 contract를 import하게 되어 의존 방향 규칙(conventions/go-and-packages.md 99-102행)과 아키텍처 테스트가 실패하므로, 두 사용 패키지가 이미 의존하는 internal/domain/issueops에 상수를 두어야 한다.

## 4차: 통과

- 4라운드 delta 리뷰, 상향 모델 anthropic/claude-opus-5-5 xhigh(omp -p 빈 컨텍스트 세션). 부모 승계 표시 상수를 internal/domain/issueops로 옮기는 처방을 검증했다. 이 상수를 쓰는 domain·application 패키지는 이미 이 패키지를 import하므로 새 의존이나 순환이 생기지 않고, HEAD 사본에서 상수를 옮겨 보니 빌드, vet, domain 테스트, 의존 그래프 테스트가 모두 통과했다.
- 위임 자식 복사 지점은 BuildDelegatedProfile 한 곳이고 호출부도 하나라서 G10이 유일한 생산 경로를 잡는다. G2가 단언하는 delegated-parent-review 기록 거부는 subagent와 inline만 허용하는 기존 DA 검증 규칙과 맞는다. 비차단 메모로, 허용값 검사는 CLI 파서가 아니라 기록 검증 함수에 두는 것이 G2와 맞는다.
