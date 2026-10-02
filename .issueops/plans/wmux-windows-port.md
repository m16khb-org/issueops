# wmux: macOS cmux의 Windows 포팅 설계안

작성일: 2026-10-02. 상태: 검토용 설계·실행 계획. 제품 구현은 시작하지 않았다.

## 목표와 기본값

사용자 요구는 Ghostty 엔진을 재사용하면서 Mac의 cmux를 Windows에 완전히 포팅하는 것이다. 완료 기준은 비슷한 UI가 아니라 고정한 cmux 버전의 사용자 기능·CLI·설정·자동화 계약을 재현하는 것이다.

- 별도 wmux 저장소에서 구현한다. issueops는 계획 보관 위치일 뿐 제품 코드나 런타임 의존성이 아니다.
- Windows 11 x64를 첫 지원 대상으로 한다. ARM64와 이전 Windows는 별도 검증 대상이며 지원한다고 표시하지 않는다.
- WSL2 bash/zsh/fish를 우선 검증하고 PowerShell 7/cmd도 지원하는 것을 기본값으로 제안한다. 사용자 답변에 따라 우선순위를 조정한다.
- 앱 이름과 기본 명령은 wmux. cmux 명령·환경변수·socket 프로토콜 호환 계층을 제공한다. 다른 cmux 설치를 덮어쓰지 않는다.
- 개인 사용판도 기능 동등성 테스트를 유지한다. 공개 배포 시 원본과 third-party 라이선스·고지를 보존한다.
- 본문 기간은 숙련 개발자의 전업 작업 추정이다. AI 활용을 전제하지만 실제 측정 일정은 아니다.

## 확인한 기준 소스

조회한 cmux 기준 SHA는 `37ee6af9846b7bef8b5ec5e2eeeb64f51832fe3f`다. 이 SHA에 맞춘 요구사항 목록을 만들고 구현 중 main을 자동 추종하지 않는다. 설치된 cmux 버전을 사용자가 지정하면 T0에서 그 버전으로 기준을 교체한다.

| 자료 | 확인 사항 |
|---|---|
| [cmux CLI 계약](https://github.com/manaflow-ai/cmux/blob/37ee6af9846b7bef8b5ec5e2eeeb64f51832fe3f/docs/cli-contract.md) | 명령, alias, flags, ID, exit, socket 없이 동작하는 help까지 호환 대상이다. |
| [Ghostty 포크 문서](https://github.com/manaflow-ai/cmux/blob/37ee6af9846b7bef8b5ec5e2eeeb64f51832fe3f/docs/ghostty-fork.md) | cmux는 일반 upstream과 다른 Ghostty 패치를 사용한다. |
| [submodule 설정](https://github.com/manaflow-ai/cmux/blob/37ee6af9846b7bef8b5ec5e2eeeb64f51832fe3f/.gitmodules) | manaflow-ai/ghostty를 사용한다. Git tree의 gitlink는 `324c0273815ddc383d8d2fbf8d59d590cb65dfdb`다. 빌드 artifact pin과 차이가 있으면 실제 기준 빌드가 로드하는 엔진을 별도 기록한다. |
| [브라우저 포팅 계약](https://github.com/manaflow-ai/cmux/blob/37ee6af9846b7bef8b5ec5e2eeeb64f51832fe3f/docs/agent-browser-port-spec.md) | window/workspace/pane/surface 의미와 browser 자동화 테스트가 있다. 과거 검증 기록이므로 현재 실행 성공으로 간주하지 않는다. |
| [이벤트 계약](https://github.com/manaflow-ai/cmux/blob/37ee6af9846b7bef8b5ec5e2eeeb64f51832fe3f/docs/events.md) | boot_id, seq, 재접속 및 resume gap을 보존해야 한다. |
| [세션 유지 계약](https://github.com/manaflow-ai/cmux/blob/37ee6af9846b7bef8b5ec5e2eeeb64f51832fe3f/docs/local-tmux.md) | ordinary shell 복원과 opt-in tmux live persistence가 다르다. |
| [현재 라이선스](https://github.com/manaflow-ai/cmux/blob/37ee6af9846b7bef8b5ec5e2eeeb64f51832fe3f/LICENSE) | 기본 GPL-3.0-or-later와 일부 서버 디렉터리 BUSL-1.1이 구분된다. 모든 서버 코드를 같은 조건으로 재배포할 수 있다고 가정하지 않는다. |
| [Windows 참고 포크](https://github.com/sweetcornna/cmux-for-windows/tree/c7e5960cfe23534833bc69b5fe0ea129b8c2768e) | WinUI 3/ConPTY/Rust/DirectWrite·Direct2D/WebView2 구현이다. README상 원본 전체 기능을 제공하지 않는다. |
| [기존 wmux 이름의 저장소](https://github.com/amanthanvi/wmux/tree/a5a70ff90633d11d617d8f8ee788761b1961e01c) | 동일 이름 프로젝트가 이미 있다. 공개 저장소 owner와 배포 ID를 구분한다. 이름만 보고 Windows 완성 구현으로 채택하지 않는다. |

## 접근 선택

1. **선택: cmux 기준 기능을 포팅하고 Windows 포크의 검증된 구성요소를 선별 재사용한다.** Ghostty VT 코어, cmux의 재사용 가능한 Rust 코드·protocol·tests를 최대한 보존한다. 기존 Windows 포크 전체를 완성품으로 취급하지 않는다.
2. 기존 Windows 포크 전체를 출발점으로 삼으면 첫 데모는 빠르지만 구조와 CLI 의미 차이를 먼저 해결해야 한다. T1에서 계약 차이와 렌더링 손실을 평가해 가져올 모듈을 결정한다.
3. cmux Swift/AppKit와 Ghostty 전체 embedded renderer를 Windows로 직접 이식하면 macOS 의존 API 교체가 커진다. 기본 경로로 선택하지 않는다. VT 경로가 필수 기능을 표현하지 못하면 T1에서 full renderer backend 포팅의 증분 비용을 평가하고 설계를 재검토한다.

## 구조와 책임

| 경계 | 선택 | 책임 |
|---|---|---|
| Windows 화면 | C# / WinUI 3 | workspace sidebar, panes, menus, shortcuts, accessibility, focus, DPI |
| 터미널 코어 | Zig Ghostty VT + 얇은 Rust wrapper | VT parser/state, scrollback, key/mouse encoding. cmux fork와의 patch 차이를 추적한다. |
| 렌더러 | DirectWrite/Direct2D 기반 Windows renderer | shaping, glyph fallback, ligatures, selection/cursor, images. 필요 시 GPU effect 단계로 shader 호환을 구현한다. 셀마다 독립 shaping하는 방식으로 완전 호환을 주장하지 않는다. |
| 세션·자동화 | Rust | session lifecycle, workspace model, CLI/socket compatibility, persistence, notifications routing |
| Windows 셸 | ConPTY | PowerShell/cmd process creation, I/O, resize, teardown |
| WSL 셸 | WSL 내부 POSIX PTY helper + framed byte bridge | 원본 VT bytes, resize, exit, cwd를 명시적 프레임으로 전달한다. ConPTY가 확장 VT를 보존한다고 가정하지 않는다. |
| 브라우저 | WebView2 | WKWebView 사용자 동작과 browser RPC를 대응한다. host 명령 실행 권한을 페이지에 직접 노출하지 않는다. |
| OS 통합 | Windows adapter | notifications, clipboard, drag/drop, taskbar, hotkeys, updater |

터미널 출력은 session I/O → Ghostty state → renderer로 흐른다. 입력은 WinUI key/IME → Ghostty encoding → session I/O로 흐른다. 입력 조합 중 문자와 확정 문자를 구분해 중복 전송을 막는다. UI가 업무 상태를 별도로 복제하지 않고 Rust core의 ID와 이벤트를 사용한다.

UI/Rust는 C ABI handle과 명시적 소유권으로 연결한다. terminal cell을 JSON으로 매 프레임 복사하지 않는다. 화면 갱신은 dirty region/버전이 붙은 immutable snapshot 또는 bounded buffer로 전달하며 renderer가 상태 수명 밖 메모리를 참조하지 않게 한다.

## 호환성 계약

- GUI: window/workspace/pane/surface 모델, 분할 이동과 focus, 탭 순서, unread 상태, command palette, 설정, 접근성을 각각 기능 행으로 추적한다.
- CLI/API: cmux 원본 CLI와 GUI socket dispatcher를 기준으로 명령·alias·인자·JSON·오류·exit code·event ordering을 수집한다. cmux-tui의 별도 resource API가 Mac API와 같다고 가정하지 않는다.
- IPC: native Windows에서는 사용자 전용 ACL을 둔 named pipe를 기본 transport로 쓴다. 직접 AF_UNIX를 쓰는 기존 client용 endpoint를 제공하고, WSL에는 사용자 전용 Unix socket helper를 둔다. request/response 의미는 같은 dispatcher로 수렴한다. WSL↔Windows 경계를 같은 socket 경로로 처리하지 않는다.
- 기존 cmux auth mode의 허용/거부 결과를 fixture로 보존한다. Windows/WSL에서 ancestry 증명이 불가능한 경우 허용으로 우회하지 않고 문서화된 credential 경로를 사용한다. browser/CDP/control port를 외부에 무인증으로 열지 않는다.
- 설정: Ghostty parser 또는 같은 grammar 계약을 재사용하고 cmux.json 공통 필드를 보존한다. macOS 전용 설정은 원본 파일을 유지한 채 Windows 대응 여부를 보여준다. import는 preview 후 사용자가 적용하며 원본 설정을 덮어쓰지 않는다.
- shell context: 경로를 host/distro/path로 구분한다. `C:\\...`, WSL `/home/...`, SSH remote path를 문자열 치환만으로 같은 공간처럼 취급하지 않는다.
- 복원: layout/cwd/scrollback 복원과 살아 있는 process 재접속은 별도 기능이다. 임의 프로세스의 재부팅 후 생존을 약속하지 않는다. WSL tmux의 live detach/reattach를 원본 opt-in 기능에 대응한다.

## 완전 포팅의 판정

T0에서 모든 발견 기능에 ID, 근거, 원본 동작, Windows 구현, 테스트 ID, 상태를 부여한다. 상태는 exact / platform-equivalent / pending / external-dependency / not-applicable로 구분한다. 범위 밖으로 둔 항목을 성공 분모에서 조용히 빼지 않는다.

- 원본과 같은 기능은 exact로 완료한다.
- Cmd 예약 키, Dock, macOS fullscreen, AppleScript 등은 Windows 동등 동작을 제안하고 사용자 확인 후 platform-equivalent로 인정한다.
- macOS simulator처럼 대상 런타임이 없는 기능은 원격 Mac 연결 등 동등 목적을 구현할지 별도 결정한다. 구현 없이 완전 포팅이라고 표시하지 않는다.
- Cloud 계정/VM, 모바일 pairing/relay, 브라우저 credential import, 외부 provider 연동도 목록에 포함한다. 적법한 서비스 접근과 계약을 확인해 테스트한다. 외부 서비스 접근 불가를 로컬 구현 성공으로 대체하지 않는다.
- 자체 SaaS 운영이나 새 iOS 앱 제작은 이번 Windows client 포팅과 별도 제품이다. 이를 요구하는 기능이 남으면 전체 기능 동등성 완료는 보류하고 데스크톱 독립 기능 완료만 선언한다.

## 실행 순서와 예상 공수

공수는 인·주이며 순차 합계 29~47 인·주, 통합·불확실성 여유 25% 포함 약 36~59 인·주다. 외부 서비스 협의와 별도 서버/모바일 제품 제작 시간은 포함하지 않는다.

| 작업 | 공수 | 선행 | 책임/범주 |
|---|---:|---|---|
| T0 기준 버전·기능·라이선스·실행 환경 고정 | 1~2 | 없음 | lead / deep |
| T1 엔진·renderer·ConPTY·WSL 경로 기술 검증 | 3~5 | T0 | terminal / deep |
| T2 Windows 터미널과 입력·렌더링 | 4~6 | T1 | terminal / deep |
| T3 workspace·분할·설정·CLI·IPC | 5~8 | T1, GUI 통합은 T2 | app/core / visual-engineering+deep |
| T4 에이전트·알림·복원·셸 통합 | 5~8 | T2,T3 | core / deep |
| T5 브라우저·SSH·remote·외부 연동 | 6~10 | T3, 통합은 T4 | integration / deep |
| T6 차등 검증·성능·배포·업데이트 | 5~8 | T2~T5 | QA/release / deep |

두 사람이면 T2와 T3의 비의존 작업, T4와 T5의 독립 adapter 개발을 병행한다. GUI state와 FFI 계약을 여러 사람이 동시에 바꾸지 않는다. 한 명이면 표 순서로 실행한다. 제안 달력 기간은 1명 9~15개월, 2명 6~10개월이며 T1 결과로 재산정한다.

### T0: baseline과 oracle 만들기

원본 SHA, 실제 Mac binary/engine artifact SHA, Windows 버전, GPU/driver, DPI, WSL distro, shell 버전을 `compat/baseline.json`에 저장한다. `compat/features.yaml`에는 README뿐 아니라 menus/CLI dispatcher/config/tests에서 발견한 기능을 모두 넣는다. 파일별 license/provenance도 기록한다.

- 완료: 원본 기능마다 실행 가능한 시나리오와 owner 또는 외부 의존성이 지정된다. 화면/CLI/config inventory에 미분류 행이 없다.
- 정상 QA: Mac에서 workspace 2개, pane 3개, terminal/browser 각 1개 이상을 만들고 IDs/이벤트/설정/스크린샷을 수집한다.
- 실패 QA: 원본 앱을 종료한 상태의 `cmux --help`, 잘못된 surface ID, 깨진 JSON 요청의 stdout/stderr/exit/error를 각각 기록한다.
- 증거: `evidence/T0-baseline/`. 커밋 제안: `docs(compat): pin cmux parity baseline`.

### T1: 가장 위험한 경계를 먼저 검증

cmux가 실제 사용하는 engine artifact와 gitlink 차이를 확인하고 최소 patch set을 고정한다. Windows 참고 포크의 engine ABI, input, shaping, image/shader, restoration, CLI 차이를 audit한다. WinUI 터미널 1개에서 PowerShell과 WSL PTY helper를 각각 연결한다. WSL helper는 stdin/stdout pipe 위 framed protocol로 byte stream, resize, exit, metadata를 분리한다.

- 완료: 같은 VT fixture를 원본과 Windows 코어에 입력했을 때 grid/cursor/style/scrollback이 일치한다. Kitty keyboard/graphics, OSC 8/52/9/99/777, alternate screen, bracketed paste를 계층별로 비교한다.
- 정상 QA: 한글 `한글 입력 테스트`, combining mark, ZWJ emoji, ligature text, inline image, resize를 왕복한다. 출력은 byte 로그와 renderer 캡처로 확인한다.
- 실패 QA: ConPTY 경로에서 제어문자가 변형·유실되면 지원 성공으로 표시하지 않는다. WSL helper/SSH raw stream과 대조하고 Windows native 경로의 미해결 기능으로 남긴다.
- 종료 규칙: 5 인·주 안에 required terminal fixture가 해결되지 않으면 broad GUI 구현을 확대하지 않는다. full renderer/transport 대안의 비용과 변경 계획을 제출한다.
- 증거: `evidence/T1-engine-spike/`. 커밋 제안: `test(terminal): establish Windows parity probes`.

### T2: 제품 터미널 구현

DirectWrite shaping을 grapheme/cell mapping과 연결하고 ligature run, wide cell, bidi의 원본 지원 범위, emoji fallback, cursor/selection, hyperlinks, image placement, shader 설정을 맞춘다. GPU device loss/recreate, 100/150/200% DPI와 multi-monitor 이동을 처리한다. WinUI IME 조합·확정·취소와 shortcut dispatch를 분리한다.

- 완료: engine fixture와 사용자 입력 fixture가 100% 통과한다. renderer 누락 기능을 VT parser 지원으로 대신 완료하지 않는다.
- 정상 QA: 한국어 IME로 100회 입력/수정/확정, 분할 간 focus 이동 후 전송 bytes에 누락·중복이 없다. 100MB 출력 후 스크롤·선택·copy가 동작한다.
- 실패 QA: 출력 중 resize 100회, GPU 재생성, PTY 강제 종료에도 UI가 멈추지 않고 닫힌 세션 상태가 일치한다.
- 증거: `evidence/T2-terminal/`. 커밋 제안: `feat(terminal): implement Windows terminal surface`.

### T3: cmux UI와 자동화 계약 포팅

window/workspace/pane/surface 모델을 포팅하고 CLI/socket/event adapter를 만든다. Mac CLI golden fixture를 wmux와 cmux-compatible shim에 재생한다. tmux prefix를 요구하지 않는 원본 동작을 유지한다. ghostty config와 cmux.json import/live reload를 구현한다.

- 완료: workspace/surface ID는 이동·분할·복원 후 계약대로 유지된다. CLI 성공·오류·flags·alias·auth·event ordering이 golden과 일치한다.
- 정상 QA: 3 panes를 T자 배치로 만든 뒤 terminal과 browser surface를 이동한다. CLI identify 결과와 실제 focused surface가 일치한다.
- 실패 QA: 다른 사용자·잘못된 credential·재시작 전 stale cursor·이미 닫힌 surface 요청을 각각 원본 계약에 따라 거부/보고한다. agent가 focus를 임의로 빼앗지 않는다.
- 증거: `evidence/T3-contract/`. 커밋 제안: `feat(workspace): port cmux workspace and control contracts`.

### T4: 실제 에이전트 작업 흐름

원본 지원 agent inventory를 기준으로 hooks/resume/notification을 포팅한다. Claude Code/Codex/OpenCode를 첫 실사용 대상으로 하되 나머지를 미지원 목록에서 숨기지 않는다. Linux hook와 Windows hook 실행을 분리하고 provider별 native session ID를 보존한다. ordinary shell 재시작과 tmux live session을 구분한다.

- 완료: notification이 정확한 surface로 라우팅되고 unread 이동·dismiss가 일치한다. 설정 importer가 사용자 hook를 덮어쓰지 않는다. restore schema/version과 atomic write를 검증한다.
- 정상 QA: 독립 agent session 3개를 실행하고 하나만 입력 대기로 만든다. 해당 pane만 표시되며 재시작 후 해당 native session ID로 resume한다.
- 실패 QA: 잘린 snapshot, 두 앱 동시 시작, provider 미설치, notification 권한 거부, 관리자 권한 실행을 각각 시험한다. 실패한 resume를 성공으로 표시하거나 임의 명령을 재실행하지 않는다.
- 증거: `evidence/T4-agents/`. 커밋 제안: `feat(agents): port notification and restore workflows`.

### T5: 브라우저·SSH·서비스 경계

WebView2로 browser RPC를 대응하고 source의 browser parity tests를 재사용한다. DOM snapshot/ref/click/fill/navigation/download/dialog/cookies/storage/errors를 실제 페이지에서 검증한다. ref는 문서 교체 시 stale 처리한다. SSH/port forwarding/file transfer/tmux와 Windows/WSL path 변환을 구현한다. 인증된 외부 서비스 기능은 별도 fixture와 계정으로 검증한다.

- 완료: 원본 지원 browser/remote 기능의 required 시나리오가 통과하고 원본 unsupported 동작도 같은 오류를 낸다. Cloud/mobile 등 external-dependency 행은 실제 증거가 있을 때만 완료한다.
- 정상 QA: 로컬 fixture 페이지의 `#name`에 `wmux 한글`을 입력해 submit 결과를 확인한다. SSH remote dev server를 browser pane에서 열고 연결된 workspace에 파일을 전송한다.
- 실패 QA: browser renderer crash, TLS 오류, SSH 단절·재접속, 잘못된 host key, stale element ref를 처리한다. 연결 재시도 때문에 원격 명령을 중복 실행하지 않는다.
- 증거: `evidence/T5-integrations/`. 커밋 제안: `feat(integration): port browser and remote workflows`.

### T6: 차등 QA·성능·배포

Mac oracle과 Windows에 같은 시나리오를 재생한다. UUID/time/OS path 등 명시된 가변 값만 정규화하고 오류나 누락 기능은 mask하지 않는다. screenshots는 OS font rasterization 차이를 분리해 layout bounds와 semantic tree를 우선 비교한다. Windows GPU 실기기에서 soak와 입력 지연을 측정한다.

- 완료: required 기능 pending=0, 사용자 합의 없는 platform 차이=0, 핵심 회귀=0. 외부 의존 미완료가 있으면 전체 완료를 주장하지 않는다.
- 제안 성능 예산: 지정한 Windows 실기기 60Hz에서 8-pane 시나리오의 foreground key-to-present p95 ≤33ms, p99 ≤100ms. frame 측정과 실제 화면 측정을 구분한다. 8시간 soak의 마지막 2시간 idle RSS 기울기 <1MB/분, session 1,000회 생성/종료 후 orphan child=0. 이는 목표이며 달성된 수치가 아니다.
- 정상 QA: 깨끗한 Windows 사용자 계정에 설치→첫 시작→업데이트→재시작→설정/세션 복원. WebView2 포함 조건을 설치 문서와 맞춘다.
- 실패 QA: 업데이트 중 종료, 손상된 패키지, GPU reset, sleep/resume, WSL shutdown, browser profile lock을 처리한다. 이전 버전과 데이터 rollback 경로를 보존한다.
- 증거: `evidence/T6-release/`. 커밋 제안: `build(release): validate and package Windows distribution`.

## 검증 도구와 명령 계약

아래 파일/명령은 wmux 저장소에서 구현할 산출물이며 현재 존재하는 도구나 이미 수행한 검증이 아니다.

- Rust: `cargo test --workspace --locked`로 core/CLI/protocol/property tests.
- C#: `dotnet test tests/Wmux.Tests/Wmux.Tests.csproj`로 input/model/interop tests.
- `python tools/parity/run.py --baseline compat/baseline.json --target wmux --suite required`로 Mac/Windows golden 비교.
- `pwsh -File tools/qa/windows.ps1 -Suite input,layout,browser,agents,remote,restore`로 UI Automation 기반 실제 GUI 경로 검증. mock만으로 완료하지 않는다.
- `pwsh -File tools/qa/soak.ps1 -Hours 8 -Panes 8`로 Windows GPU 실기기 측정.
- core·protocol·입력 버그에는 재현 테스트를 먼저 작성한다. UI는 작은 자동화 시나리오와 screenshot/semantic evidence를 함께 보존한다.

## 최종 검토와 유지보수

T6 뒤 독립 검토자는 기능 inventory, 구현 diff, 실행 증거, 제외/외부 의존 목록을 대조한다. 소스 빌드 성공을 사용자 흐름 성공으로 대체하지 않는다. 결과를 사용자에게 제시해 OS 동등 동작을 확인한 뒤 배포 범위를 확정한다. 커밋·원격 공개·배포는 구현 단계에서 정한 범위로 별도 실행한다.

upstream 업데이트는 자동 merge하지 않는다. 새 release마다 CLI/config/feature/Ghostty patch 차이를 추출하고 기존 golden에 추가한다. 최초 포팅 이후에도 회귀 검증과 업데이트 공수를 계속 확보한다.

## Gap Analysis와 남은 결정

엔진 지원과 renderer 지원을 혼동하지 않도록 분리했다. 기준이 움직이는 문제는 SHA 고정으로 해결했다. 기존 Windows 포크의 CLI가 Mac과 동일하다는 가정, ConPTY가 모든 escape를 통과시킨다는 가정, 재시작이 임의 process 복원이라는 가정을 제거했다. cloud/mobile/server 범위는 누락하지 않고 의존성으로 명시했다.

계획 작성에 막히는 질문은 없다. Windows 지원 버전과 WSL 우선순위는 제안 기본값이다. 실행 전 T0에서 원본 baseline과 OS별 대체 동작을 확정한다. T1은 renderer/transport 경로를 증명하는 필수 의사결정 단계이며 결과를 얻기 전에 장기 일정을 확약하지 않는다.

Repo grounding: 위 SHA의 CLI/engine/browser/events/config/tmux/license 문서와 Git tree, Windows 포크, Microsoft ConPTY/WebView2 문서를 읽었다.
Decision-complete plan: Windows 전용 UI와 renderer, Ghostty VT 재사용, cmux 계약 호환, T0~T6의 책임·선행 조건·정상/실패 QA를 지정했다. 기술 feasibility는 T1의 명시된 통과/중단 규칙으로 결정한다.
Assumptions/defaults: Windows 11 x64, WSL2 우선+native 지원, 별도 제품 저장소, source 계약과 테스트 우선.
Unresolved questions: 계획 작성에는 none blocking. 실제 WSL 우선순위·baseline 선택·외부 서비스 접근·T1 feasibility는 실행 전후 지정 게이트에서 확인한다.
Acceptance criteria: required parity tests 전부 통과, orphan=0, GUI/IME/remote/restore 실기기 증거, 성능 예산 충족, 외부 의존과 OS 차이 명시.
