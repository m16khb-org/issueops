---
name: 2026-11-03-support-native-windows-install-and-runtime
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Support native Windows install and runtime

- Date: 2026-11-03
- Kind: `adr`
- Source: omp
- Summary: Windows 10/11 x64를 네이티브 설치·실행 대상으로 지원한다. OS 차이는 *_windows.go 파일과 두 공용 primitive(outbound/osfile, outbound/osprocess)에 모으고, darwin/linux 동작은 바꾸지 않는다.
- Context: 사용자가 Windows PC에 issueops 설치를 요청했고, WSL 대신 Windows 지원 코드 작업을 선택했다. GOOS=windows 빌드는 unix 전용 파일(flock, Stat_t, Setpgid, openat, ps/lsof) 때문에 10개 패키지에서 실패했다. 2026-06-24 ADR은 Windows를 release에서 제외했고, 2026-10-06 ADR 두 건은 windows/!unix stub을 지우면서 Windows를 다시 지원하려면 stub부터 새로 설계해야 한다고 남겼다.
- Decision: 1) outbound/osfile: Unix mode 비트를 Windows에서는 소유자 SID와 DACL로 계산한다(Perm). owner-only는 현재 사용자·SYSTEM·Administrators만 허용하는 protected DACL이다(Chmod/Mkdir/MkdirAll/OpenFile). 파일 identity는 volume serial과 file index다. ExecutableName은 .exe를 붙인다. 2) outbound/osprocess: Toolhelp32 snapshot, 생성 시각·image 경로, PEB의 command line과 cwd, 프로세스 트리 종료를 제공한다. ps·lsof·kill(-pgid)·Signal(0)을 대신한다. 3) processlease는 LockFileEx 대신 share mode로 잠근다. LockFileEx 범위는 상속받은 자식이 살아 있어도 부모 핸들을 닫으면 풀리기 때문이다. 4) 설치: bin\issueops.exe와 issueops.exe·io.exe shim, HKCU\Environment 사용자 Path, Codex commandWindows(PowerShell & 호출), Claude exec form(command+args). 추적되는 configs/ 템플릿은 OS 중립 shell form을 유지한다. 5) io update는 Windows에서 install-native.sh 대신 같은 begin/교체/seal/abort 단계를 Go로 수행하고, 실행 중인 바이너리는 .issueops.retired-*.exe로 rename한다. 6) MCP 기본 transport는 stdio다(supervisor 없음). --adopt-command-file과 cmux는 명시적 오류로 거부한다. 7) SQLite는 Windows에서 유휴 연결을 남기지 않는다(SetMaxIdleConns(0)). 열린 파일은 삭제·rename을 막기 때문이다. 8) .gitattributes로 LF checkout을 고정하고 release matrix에 windows/amd64를 넣는다.
- Consequences: 2026-06-24의 Windows 제외 결정과 2026-10-06 ADR들의 Windows stub 관련 결론을 대체한다. Windows에서는 HTTP MCP service, --adopt-command-file, cmux handoff, workspace 점유 관측 중 열린 쓰기 핸들 열거를 지원하지 않는다(cwd 점유만 본다). `go test ./...`는 Windows·Linux에서 모두 통과한다. 테스트 fixture는 internal/testsupport의 AbsPath·WriteFakeCommand·SetHome·MakePermissive·MakeUnreadable로 OS 차이를 흡수하고, Windows 의미가 실제로 다른 경우만 이유를 밝혀 skip하거나 build tag로 제한한다. 쓰기 경로는 owner-only가 필요한 곳에서 osfile 쓰기 함수를 쓰고, 동시 reader가 있는 원자적 교체는 osfile.Replace를 쓴다(Windows는 공유 위반을 잠시 재시도한다).
- Evidence:
  - GOOS=windows/linux/darwin go vet ./... 통과
  - scripts/release-build-matrix.sh: ok=true targets=5 (windows/amd64 포함)
  - issueops.exe install --json: committed=true, sealed after strict Codex/Claude/Omo/omp/opencode readback
  - io update --json: committed=true, 실행 중 바이너리를 .issueops.retired-*.exe로 rename
  - codex mcp get issueops: command C:\Users\hb\workspace\issueops\bin\issueops.exe
  - PowerShell & 'bin\issueops.exe' hook session-start --host codex: hookSpecificOutput.additionalContext 출력
  - MCP stdio initialize/tools/list(51)/docs_index 호출 성공
  - go test ./internal/adapter/outbound/osfile ./internal/adapter/outbound/osprocess: ok
  - go test ./... -count=1: Windows 11(owner-only TMP) exit 0, 334 packages ok; Linux(WSL Ubuntu, go1.27.3) exit 0; go test -race ./... (Linux) exit 0
- Alternatives / rejected options:
  - WSL에만 설치 — Windows에서 실행하는 host(Codex, omp, opencode)가 issueops에 연결하지 못한다.
  - LockFileEx로 lease 구현 — 실험에서 부모가 핸들을 닫으면 상속한 자식이 살아 있어도 잠금이 풀렸다.
  - 소유자 SID만 검사 — 다른 principal에게 읽기를 허용한 ACL을 막지 못한다. 사용자가 ACL 검증을 선택했다.
  - Windows에서 install-native.sh를 Git Bash로 실행 — python3·shasum이 필요하고 실행 중인 exe를 os.replace로 덮을 수 없다.
