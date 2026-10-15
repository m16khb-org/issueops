---
name: 2026-10-15-linux-exec-text-file-busy
description: Caution record for a solved false case or recurring risk.
---

# Linux에서 병렬 테스트가 방금 쓴 스크립트를 exec하면 text file busy로 실패한다

- Date: 2026-10-15
- Kind: `caution`
- Source: issueops-docs io-9e1b5fa96747 (#563)
- Summary: 스크립트를 os.WriteFile로 쓴 직후 exec하는 테스트를 t.Parallel로 돌리면, 다른 병렬 테스트의 fork가 쓰기 fd를 잠깐 복제해 Linux에서 exec가 ETXTBSY(text file busy)로 확률적으로 실패한다. macOS에서는 재현되지 않는다.
- Context: #563 PR #564의 pull_request CI에서 internal/adapter/update TestRuntimeInstall이 'fork/exec .../install-native.sh: text file busy'로 두 번 연속 실패했다. 같은 head의 push run은 통과했다. lint 실패에 가려 Race test 단계가 한동안 돌지 않아 늦게 드러났다. Linux arm64 test binary를 ubuntu:24.04 컨테이너에서 -test.count=300 -test.cpu=4 -test.shuffle=on으로 돌리면 재현됐다(TestProcessTableRead, TestRuntimeInstall, TestRuntimeList 실패).
- Resolution: 스크립트를 쓰고 곧바로 실행하는 테스트(TestProcessTableRead, TestRuntimeInstall, TestRuntimeList와 그 하위 테스트)에서 t.Parallel을 뺐다. 직렬 테스트는 병렬 테스트가 재개되기 전에 끝나고 병렬로 남은 테스트는 fork하지 않으므로 창이 사라진다. 같은 컨테이너 실행(count 1500, cpu 2,8,16, shuffle)이 text file busy 없이 통과했고 #563 게이트 원장 G5가 그 실행을 고정한다. macOS 로컬 실행만으로는 이 부류를 확인할 수 없으므로 exec 테스트를 병렬화할 때는 Linux 컨테이너에서 확인한다.
- Evidence:
  - internal/adapter/update/runtime_test.go (writeScript 주석)
  - .issueops/issues/563/gates.md (G5)
  - go test -c GOOS=linux 바이너리를 ubuntu:24.04 컨테이너에서 실행: 수정 전 text file busy 재현, 수정 후 -test.count=1500 통과
- Alternatives / rejected options:
  - writeScript 뒤 exec를 ETXTBSY 재시도로 감싸기: 원인을 두고 증상을 가린다
  - 병렬 테스트 전체를 직렬로 바꾸기: fork하지 않는 테스트까지 늦춘다
