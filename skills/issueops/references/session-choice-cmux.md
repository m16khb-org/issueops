# 사용자가 명시한 cmux 인계

[세션 인계](session-choice.md)의 환경 판별을 마친 뒤, 사용자가 cmux를 명시했을 때만 이 문서를 읽는다.

cmux는 위 자동 선택에 참여하지 않는다. 사용자가 cmux 사용을 명시했고, 기존
execution이 정확한 generation에서 released이며 canonical worktree가 이미 있을 때만 다음
명령을 실행한다. bare `cmux <path>`는 앱을 자동으로 시작할 수 있으므로 실행하지 않는다.
Omo도 generic terminal 안에서 native Omo 실행 파일을 직접 실행하며 `cmux omo`를 호출하지
않는다. omp는 bun script로 실행되어 cmux receiver 검사에서 `bun`으로 관측되므로 cmux로 omp에
인계하면 그 검사에서 fail closed 된다. opencode는 `/usr/bin/env`로 `OPENCODE_CONFIG_CONTENT`에
모델을 실어 `--standalone` 세션을 연다. TUI에는 모델·effort 플래그가 없어 effort는 전달하지 않는다.

prompt 파일은 canonical worktree 안의 absolute regular file로 만들고 mode 0600, 현재
프로세스의 effective UID 소유, 최대 64 KiB, SHA-256 조건을 모두 충족해야 한다. prompt는
native host에 단일 argv 문자열로 전달되므로 64 KiB를 넘길 수 없다. leaf의 owner, mode,
크기, 파일 identity는 같은 file descriptor로 읽기 전후에 확인한다. 상위 디렉터리는
no-follow handle traversal과 namespace identity 검사가 보호하므로 이 leaf owner 규칙의
대상이 아니다. material digest는 봉인된 인계 자료의 SHA-256이다. cmux와 native host
실행 파일, socket, window UUID, model을 실측한 exact 값으로 채우고, 해당 host가 지원하는
effort만 전달한다. cmux의 private launcher는 Claude Code에
`--dangerously-skip-permissions`, Codex에
`--dangerously-bypass-approvals-and-sandbox`, Omo에 `--permission-preset full-access`, omp에 `--auto-approve`,
opencode에 `opencode --standalone --auto`를 붙인다.

```bash
issueops execution handoff-cmux \
  --id "$ISSUEOPS_ID" --generation "$GENERATION" \
  --cmux-executable "$CMUX_EXECUTABLE" --cmux-version "$CMUX_VERSION" \
  --cmux-build-identity 'cmux 0.64.10 (90) [fafa50702]' \
  --socket "$CMUX_SOCKET" --window "$CMUX_WINDOW_UUID" --cwd "$ISSUEOPS_EXPECTED_WORKTREE" \
  --host "$HOST" --host-executable "$HOST_EXECUTABLE" \
  --model "$MODEL" --effort "$EFFORT" \
  --prompt-file "$PROMPT_FILE" --prompt-sha256 "$PROMPT_SHA256" \
  --material-sha256 "$MATERIAL_SHA256" --json
```

이 명령은 앱을 시작하거나 설치하지 않고 socket을 검색하거나 권한을 바꾸지도 않는다.
absolute non-symlink Unix socket의 owner, mode, parent, device, inode, ctime을 읽어 **endpoint
incarnation**으로 봉인하고, exact executable/version, `ping`, `capabilities`, exact window의
`identify --no-caller`만 bounded read-only preflight로 실행한다. socket 부재·접근 거부,
불완전하거나 중복된 응답, 필요한 capability 부재, endpoint incarnation 변화는 mutation 전에
fail-closed된다.

첫 검사에서 canonical worktree 디렉터리 handle과 filesystem identity를 고정한다. cmux의
`Preflight`, `CreateWorkspace`, `Send`를 각각 호출하기 직전에 durable record를 다시 읽고,
released direct status, exact generation, durable worktree, 요청의 `--cwd`, 실제 issueops
프로세스 cwd, Git top-level, 고정한 filesystem identity가 같은 canonical 디렉터리인지
다시 확인한다. workspace 생성에는 이때 검증한 canonical path를 전달한다. 같은
lifecycle·generation에 cmux `call_staged`가 하나라도 있으면 window나 prompt가 달라도 새
시도를 거부한다. preflight가 끝나면 같은 handoff-delivery lineage에 exact window와 cwd만 `call_staged`로 먼저
기록한다. 아직 없는 workspace/surface identity를 만들지 않는다. 그 뒤 exact window에 빈
workspace 하나를 만들고, 반환된 workspace의 단일 pane/surface와 cwd를 다시 확인해 같은
관측을 보강한 다음, 그 exact window/workspace/surface로 private launcher command를 한 번만
보낸다. private artifact는 mode 0700 directory, mode 0600 prompt, mode 0700 launcher를 쓰며
receiver는 private expected 변수와 cmux가 제공한 실제 `CMUX_WORKSPACE_ID`,
`CMUX_SURFACE_ID`, `CMUX_SOCKET_PATH`를 대조하고 `CMUX_WINDOW_ID`가 있으면 window도
대조한다. cwd와 scope가 맞을 때만 receipt를 남긴 뒤 prompt와 launcher를 지운다.

cmux 0.64.10의 성공한 raw input은 `input_accepted`의 `raw_input` 증거일 뿐이다.
`native_turn_observed`나 `owner_claimed`를 설정하지 않는다. bootstrap PID에서 관측한 실행
파일이 기대한 native host executable과 같은 파일임을 입증한 경우에만 process receipt를
붙인다. launcher가 별도 자손을 시작해 정확한 agent descendant를 입증할 수 없으면 process
evidence를 비워 두고 raw input 접수까지만 보고한다. 수신자의 별도 IssueOps claim CAS만
owner claim을 만든다. 이 버전은 runtime, machine, server ID를 노출하지 않으므로 빈 값을
추측해 채우지 않는다. endpoint incarnation은 경로에서 관측한 socket 항목이 preflight와
mutation 사이에 같았다는 관측값일 뿐, 실제 peer identity나 socket race의 완전한 차단,
cmux runtime identity, live host 지원 인증을 뜻하지 않는다. canonical root도 각 호출 직전에
다시 확인하지만, adapter가 고정한 directory file descriptor를 cmux 자체의 `--cwd` path
lookup에 강제로 연결할 수는 없다. 따라서 검사와 cmux의 path lookup 사이에 남는 경쟁 조건을
완전히 제거했다고 보고하지 않는다.

workspace create 또는 send가 timeout, 응답 유실, malformed receipt, target/cwd/runtime 변화로
끝나면 같은 attempt/lineage의 ambiguous evidence와 recovery artifact를 확인하고 명령을 다시
실행하지 않는다. durable request ID가 반환되지 않으므로 새 값을 만들거나 blind retry에 쓰지
않는다. exact 반환 ID와 read-only `identify`/`list-*`로 기존 workspace를 조사하며 자동으로
workspace를 닫지 않는다. 명시적인 복구 결정을 내리기 전에는 다른 런처나 current로 전환하지
않는다.
