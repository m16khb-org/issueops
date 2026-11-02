---
name: 2026-11-02-herdr-app-is-not-herdr-tui
description: Caution record for a solved false case or recurring risk.
---

# Herdr 앱 터미널에는 `HERDR_ENV`가 없어 Herdr 세션 판별이 `current`로 빠지고, 앱 창은 worktree를 자동으로 보여 주지 않는다

- Date: 2026-11-02
- Kind: `caution`
- Source: 외부 저장소 IssueOps 사이클의 Herdr 앱 세션 인계
- Summary: Herdr에는 터미널용 `herdr`(TUI)와 Herdr 앱(`dev.herdr.app`, `herdr-app`과 herdr-core `hc`) 두 front-end가 있다. session-choice.md가 `HERDR_ENV=1`·`HERDR_SOCKET_PATH`만 Herdr 세션으로 보아 앱 세션은 `current`로 처리됐고, worktree를 폴더로 하는 space가 없어 앱 창에 worktree가 보이지 않았다.
- Context: Herdr 앱 0.1.11이 연 터미널의 env에는 `HC_TERMINAL_SOCKET`·`HC_TERMINAL_ID`·`HC_SCOPE`가 있고 `HERDR_ENV`는 없다. 앱 창은 이미 만든 space만 보여 주며, 앱 안내문 그대로 "spaces sidebar nests worktree spaces under their repo's"다. `herdr worktree open`은 TUI 서버(`~/.config/herdr/herdr.sock`)를 대상으로 하므로 앱 창에 반영되지 않는다.
- Resolution: session-choice.md가 두 front-end를 env로 구분하고, Herdr 앱은 `herdr-app status`·`hc server status`·`hc get "$HC_TERMINAL_ID"`로 사용 가능 여부를 실측한다. 인계는 session-choice-herdr.md의 **Herdr 앱 실행** 절대로 `herdr-app do space.new --name <worktree 이름> --folder <worktree>`로 space를 만들고, 그 terminal에 `hc send`로 host 실행 명령을 한 번 보낸다. 인계문은 비공개 임시 파일에 두고 `"$(cat …)"`로 넘겨 shell이 내용을 평가하지 않게 한다. Codex는 `hc --skill`의 요구대로 `--no-daemon`을 붙인다.
- Evidence: `herdr-app commands`의 `space.new {name, folder, tab, argv, task, machine}`, `space.new`가 `{space, tab, terminal}`을 반환하고 `tab.new`가 `{tab, terminal}`을 반환하는 것을 임시 space로 확인했다. `--task`는 프로그램 argv 마지막 인자로 붙는다(`hc get`의 argv). `hc send`로 보낸 `"$(cat …)"`가 `$`·backtick·따옴표·개행을 그대로 전달했다. launcher identity(`hc` 경로·버전, core pid와 시작 시각, IOPlatformUUID, `core.id`)는 `ValidateHandoffDeliveryObservation`을 통과했다.
