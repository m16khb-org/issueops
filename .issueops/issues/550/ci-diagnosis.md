# #550 CI install 실패 원인 판정

- 관측 run: https://github.com/m16khb-org/issueops/actions/runs/37595246857 (진단 커밋 `69ed5d85` 단독 push, 2026-10-07, ubuntu GitHub-hosted runner)
- 실패 단계: `Deterministic self-verify gate`의 `HOME="$tmp_home" ... ./scripts/install-native.sh --skip-build --path-mode=skip`

실패 로그(진단 커밋이 덧붙인 명령과 출력 포함):

```text
install: mcp http service is not ready (status=stopped error_code=supervisor_failed); host MCP configs were not changed: load mcp service job: systemctl --user enable issueops-mcp.service: exit status 1: Failed to enable unit: Unit file issueops-mcp.service does not exist.
```

## 판정: (b) 임시 HOME unit 경로 불일치

- `systemctl --user daemon-reload`는 성공했다. supervisor는 daemon-reload, enable, start 순서로 실행하고 첫 실패에서 멈추므로, runner에 user systemd manager가 있고 접속도 된다. 그래서 후보 (a) user manager 부재는 반증됐다.
- 실패한 것은 `enable`이고 이유는 `Unit file issueops-mcp.service does not exist`다. installer는 unit을 `$HOME/.config/systemd/user/`, 곧 CI가 바꾼 임시 HOME 아래에 쓴다(`internal/adapter/mcpservice/supervisor.go`의 `systemd.unitPath`, `Home: native.Home`). 이미 실행 중인 user manager는 자기 세션의 HOME(runner 계정의 원래 HOME) 기준 경로에서 unit을 찾는다. 그래서 임시 HOME의 unit을 보지 못한다.

## 결과

- CI는 ADR 2026-10-02(supervisor를 쓸 수 없을 때 stdio로 자동 전환하지 않음)에 따라 install에 `--mcp-transport=stdio`를 명시한다.
- 후속: HOME을 바꿔 설치하는 모든 Linux 환경에서 같은 결함이 난다. user manager의 unit 검색 경로와 installer가 쓰는 경로가 갈리기 때문이다. 이번 범위에서는 고치지 않고 PR과 완료 기록에 후속으로 남긴다.
