# Ghostty와 cmux의 Windows 사용성 재현 검토

확인일: 2026-10-02. 공개 문서와 소스를 읽은 조사이며 Windows 실행 QA는 수행하지 않았다.

## 판단

터미널, 분할, 작업공간, 내장 브라우저, 알림은 Windows에서 구현할 수 있다. macOS 시스템 UI와 예약 단축키까지 동일하게 만드는 목표는 별도로 제한해야 한다. 기존 Windows 포크를 먼저 평가하는 편이 합리적이다.

## 근거

- [Ghostty 플랫폼 설명](https://ghostty.org/docs/features): 공식 앱은 macOS/Linux를 지원한다. macOS Metal과 Linux OpenGL을 사용한다.
- [Ghostty 1.3 설명](https://ghostty.org/docs/install/release-notes/1-3-0): Windows 앱과 Windows를 지원하는 libghostty를 구분한다.
- [cmux 원본](https://github.com/manaflow-ai/cmux): Swift/AppKit 기반이며 작업공간, 브라우저, 알림, CLI 기능을 제공한다.
- [cmux 터미널 소스](https://github.com/manaflow-ai/cmux/blob/main/Sources/GhosttyTerminalView.swift): AppKit, Metal, CoreText, Darwin을 사용한다. 단순 재컴파일로 Windows 앱이 되지 않는다.
- [cmux 브라우저 소스](https://github.com/manaflow-ai/cmux/blob/main/Packages/macOS/CmuxBrowser/Sources/CmuxBrowser/WebView/CmuxWebView.swift): WKWebView 기반 API를 사용한다.
- [Windows 커뮤니티 포크](https://github.com/sweetcornna/cmux-for-windows): WinUI 3, ConPTY, DirectWrite/Direct2D, WebView2 기반 구현을 설명한다. 원본의 공식 Windows 배포판이 아니다. README상 SSH/remote 기능과 시스템 알림이 빠져 있으며 출력과 스크롤백 복원을 제공하지 않는다. 이 지원표는 제작자 설명이며 실행 검증 결과가 아니다.
- [ConPTY](https://learn.microsoft.com/en-us/windows/console/pseudoconsoles): Windows가 외부 터미널 호스팅 경계를 제공한다.
- [WebView2](https://learn.microsoft.com/en-us/microsoft-edge/webview2/): Chromium 기반 내장 브라우저를 제공한다. WKWebView와 기능별 대응 및 회귀 검증이 필요하다.
- [RegisterHotKey](https://learn.microsoft.com/en-us/windows/win32/api/winuser/nf-winuser-registerhotkey): Windows 키 조합의 OS 예약과 등록 충돌을 설명한다. Cmd를 Win으로 일괄 치환해 동일한 동작을 보장할 수 없다.
- [Ghostty 설정](https://ghostty.org/docs/config/reference): macOS 전용 제목 표시줄, Dock, AppleScript 및 글꼴 관련 옵션이 있다. 기능의 현재 플랫폼 제한을 다른 OS에서 구현 불가능하다는 뜻으로 해석하면 안 된다.
- [Windows 알림](https://learn.microsoft.com/en-us/windows/apps/develop/notifications/app-notifications/): 시스템 알림을 지원한다. Windows App SDK의 해당 알림 API에는 관리자 권한 실행 앱의 송수신 제한이 있다.
- [WSL 파일시스템](https://learn.microsoft.com/en-us/windows/wsl/filesystems): Windows와 Linux 사이에 경로와 파일 접근 성능 차이가 있다.
- [WSLg](https://learn.microsoft.com/en-us/windows/wsl/tutorials/gui-apps): Linux GUI 실행 기능이다. macOS cmux를 실행시키는 호환 계층이 아니다.

## 일정 추정

Windows 데스크톱과 터미널 개발 경험이 있는 개발자 1명이 전업으로 AI 도구를 활용하는 조건의 초기 추정이다. 측정된 개발 실적이나 공식 일정이 아니다.

| 범위 | 예상 기간 |
|---|---|
| 기존 포크 적합성 평가 | 3~7일 |
| 기존 포크에서 개인 필수 작업 흐름 보완 | 4~8주 |
| 기존 엔진을 재사용해 Windows UI와 OS 연동 신규 구현, 일상 사용 수준 | 3~6개월 |
| 합의한 광범위한 기능 동등성과 배포 품질 | 6~12개월 이상 |
| 터미널 엔진까지 신규 구현 | 12~24개월 이상 |

범위별 대안이므로 기간을 합산하지 않는다. Ghostty 독립 앱과 cmux 독립 앱 두 개를 모두 유지하는 비용, 지속적인 upstream 추종은 별도다. 한글 IME, DPI, GPU, 스크롤, 키 입력, WSL과 native shell, 프로세스 종료, 브라우저 자동화가 주요 검증 항목이다.

## 반증 검토와 한계

독립 검토에서 ConPTY와 WebView2를 근거로 Windows라서 터미널이나 브라우저 구현이 불가능하다는 해석을 배제했다. 모든 Win 단축키가 불가능하다는 단정도 배제했다. Windows 실기기가 연결되지 않은 이번 조사에서는 실제 입력 지연이나 포크의 기능 완성도를 측정하지 않았다.

Source fan-out: 원본 프로젝트, 독립 Windows 포크, Microsoft OS API 문서.
Source index: 위 링크는 모두 2026-10-02에 읽은 1차 자료다.
Claim verification: Windows의 대체 구현 가능성은 OS API와 포크 설명을 교차 확인했다. 포크별 지원 상태는 제작자 단일 출처다. 일정은 조건부 공학 추정이다.
Access boundary: 공개 본문과 GitHub raw/API를 사용했다. 인증이나 접근 제한 우회는 하지 않았다.
