# Claude.dev 후속 조사: mods와 effort

조회일: 2026-10-02. 최초 사이트 조사에서 본문을 읽지 못한 두 feed lead를
메인이 직접 webfetch로 읽었다. 공개 본문 모두 접근 가능했고 잘림 없이 확인했다.

## Mods

원문: <https://claude.dev/blog/getting-started-with-claude-code-mods/>.
게시일은 최초 조사에서 RSS로 확인한 2026-10-01을 따른다.

- Claude Code 2.1.287 이상에서 JavaScript/TypeScript module을 plugin으로
  로딩하는 계약이다. event마다 shell을 시작하는 settings hook과 달리
  module은 세션에 유지된다.
- observe/rewrite/answer middleware와 UI render가 가능하다.
  로드 시 생성되는 해당 build의 type declarations가 버전별 권위이며
  API가 릴리스 사이에 바뀔 수 있다고 명시한다.
- `$.session.usage()`는 기본 사용량을 읽고 breakdown을 요청할 때 별도 token
  count 요청을 보낼 수 있다. 예제의 화면 사용량은 현재 context를 뜻하며
  전체 비용이나 모든 하위 agent를 뜻하지 않는다.
- hot reload는 module local state를 초기화하므로 host-managed state를 쓰는
  예제다.
- 위험 명령을 표시하는 예제는 permission system이 아니며 alias·script 등으로
  탐지를 벗어날 수 있다고 글 스스로 밝힌다.

**적용 판단:** 선택적 Claude UI adapter의 가능성은 있으나 공용 Go core
정책·권한·SessionStart 계약을 mod로 옮길 근거는 없다. 이벤트마다 shell을
시작하지 않는 설계가 issueops의 실제 절감량을 입증하지도 않는다.
호스트 전용 편의 기능과 core invariant를 분리하는 현재 설계를 유지한다.

## Effort

원문: <https://claude.dev/blog/spending-your-effort/>.
게시일은 최초 조사에서 RSS로 확인한 2026-09-25를 따른다.

- 저자는 구현·반복에는 낮은 effort, 경계 사례와 검증에는 높은 effort를
  사용하는 방식을 제시한다.
- 높은 effort는 일부 edge-case 실패를 줄이지만 잘못된 접근법까지 자동으로
  해결하지 않는다는 내부 실험 해석이다.
- task별 예시는 같은 benchmark에서도 작은 표본이며, 실행 시점·token cap·
  인터넷 접근과 safety intervention 조건이 다르다. 글도 공개 leaderboard와
  숫자가 맞지 않을 수 있다고 명시한다.
- 따라서 특정 모델·effort의 성공률이나 시간 수치를 issueops로 일반화하지 않는다.

**적용 판단:** Luna의 단순 수집, Sol의 중간 구현 분석, Astra의 구조 판단,
마지막 Fable 5.1 독립 검증이라는 사용자 배분과 양립한다.
이 배분의 효율이 실측으로 입증됐다고 주장하지 않는다. 후속 도입 실험에서는
동일 task와 품질 기준을 맞춰 effort별 총사용량·재시도·성공당 시간을 비교한다.

## 후속 lead 상태

두 미조회 문서 lead는 닫았다. 새 mandatory 구현 요구는 발견하지 않았다.
별도의 mod 설치, host 설정 변경, benchmark 실행은 하지 않았다.
