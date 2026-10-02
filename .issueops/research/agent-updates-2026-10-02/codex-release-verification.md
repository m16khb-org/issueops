# Codex 정식 릴리스 직접 검증

조회일: 2026-10-02.

공식 `https://api.github.com/repos/openai/codex/releases/latest`를 직접 읽었다.
최초 webfetch 응답은 361,933 bytes 중 50KB만 표시되어 JSON 파싱이 실패했다.
이는 네트워크 오류가 아니라 출력 잘림이다. 같은 공개 URL에서 전체 JSON을
받아 아래 필드를 확인했다.

- HTTP 200.
- tag: `rust-v0.160.0`.
- published_at: `2026-10-01T20:19:13Z`.
- prerelease: `false`.
- release: <https://github.com/openai/codex/releases/tag/rust-v0.160.0>.

검색 요약이나 alpha 태그 대신 이 응답을 이번 조사의 정식 버전 근거로 사용한다.

## 원문에서 확인한 변경

- SQLite 연결 초기화와 logging으로 인한 stall 방지.
- initialization 오류를 timeout으로 덮지 않고 노출.
- plugin manifest parsing cache와 remote plugin HTTP connection pool 재사용.
- 오래된 task history를 “Show more”로 조회.
- reconnect 뒤 불확실한 전송을 먼저 해소한 후 미전송 메시지를 재개.
- 명시적 provider model catalog를 권위로 취급하고 실패 뒤 stale catalog 재사용 방지.
- app-server running turn을 incremental하게 추적.
- log DB의 사용하지 않는 공간을 background에서 회수.

개별 PR의 코드·테스트 설명은 `codex-01.md`와 `codex-02.md`에 있다.
위 내용은 공급자 릴리스의 변경 사실이며 issueops의 같은 병목이나 성능 개선을
입증하지 않는다.

## 수집 DAG 복구 대기

첫 수집 DAG의 `claude-08`, `claude-09`는
`getaddrinfo ENOTFOUND chatgpt.com`으로 실패했다.
`claude-08.md` 파일은 남아 있지만 작업 완료로 간주하지 않는다.
run이 settle하면 실패 노드만 retry하며 성공 노드의 결과는 재사용한다.
다른 조사 노드는 진행 중이므로 전체 DAG를 취소하거나 복제하지 않는다.
