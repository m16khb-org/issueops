# 릴리스 정보 교차검증

## Gemini CLI: 정식 changelog의 지연을 API로 해소

`other-01.md`는 stable changelog의 v0.61.0과 release index의 v0.62.0이
충돌한다고 보고했다. 당시 GitHub API rate limit 때문에 날짜를 확인하지 못했다.

메인은 2026-10-02에 다음 두 URL에서 HTTP 200을 받았다.

- <https://api.github.com/repos/google-gemini/gemini-cli/releases/tags/v0.62.0>
- <https://api.github.com/repos/google-gemini/gemini-cli/releases/latest>

두 응답 모두 아래 값을 반환했다.

- `tag_name`: `v0.62.0`.
- `published_at`: `2026-09-29T21:17:07Z`.
- `prerelease`: `false`.
- `html_url`: <https://github.com/google-gemini/gemini-cli/releases/tag/v0.62.0>.

따라서 이번 보고서에서는 v0.62.0을 조회 시점의 최신 정식 릴리스로 기록한다.
v0.63.0-preview와 v0.64.0-nightly는 정식 릴리스와 분리한다.
preview의 memory lifecycle 변경을 v0.62.0의 기능으로 옮겨 적지 않는다.

v0.62.0 원문은 PTY file descriptor 정리, process exit/output finalization,
terminal buffer memory 관리 개선을 포함한다. 개선량이나 issueops의 같은
결함 존재는 이 릴리스만으로 확인할 수 없다.
