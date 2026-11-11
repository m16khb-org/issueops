---
name: 2026-11-12-lint-local-lints-the-windows-goos-too
description: Accepted decision record with rationale, alternatives, and consequences.
---

# lint-local lints the Windows GOOS too

- Date: 2026-11-12
- Kind: `adr`
- Source: claude-code main session
- Summary: scripts/lint-local.sh는 비Windows 호스트에서 GOOS=windows lint 단계도 실행하고, 교차 lint는 CGO_ENABLED=0으로 돌린다.
- Context: ADR 2026-11-05(gate exported API doc comments with revive)의 Consequences는 "windows 전용 파일은 lint-local이 보지 않으므로 GOOS=windows 측정은 필요할 때 따로 돌린다"고 적었다. CI는 Linux에서 lint-local을 실행하므로 Windows 전용 파일의 발견이 쌓였고, 2026-11-11 측정에서 21건(errcheck 12, modernize 4, unused 3, intrange 1, unparam 1)이 나왔다. 또 CGO_ENABLED=1을 명시한 환경에서는 교차 lint가 typecheck에 실패하면서 "0 issues"를 찍었다.
- Decision: 1) lint-local.sh의 교차 lint 단계는 `CGO_ENABLED=0`으로 실행한다. 2) 비Windows 호스트에서 `GOOS=windows` lint 단계를 추가로 실행한다. CI lint job은 이 스크립트를 부르므로 두 GOOS를 함께 검사한다. 3) 21건은 `//nolint` 없이 코드로 정리했다. unix에서만 쓰는 선언은 unix 파일로 옮겼다. 4) `scripts/ci_workflow_test.py`가 두 교차 단계 줄을 고정하고, linter가 "0 issues."를 찍고 exit 1로 끝나면 스크립트도 실패하는지 부정 테스트로 확인한다. 이 결정은 ADR 2026-11-05의 해당 문장을 대체한다.
- Consequences: CI lint job은 lint를 한 번 더 돈다(16코어 Linux에서 28초, CI runner에서는 더 길다). Windows 전용 파일을 고치는 변경은 Linux에서도 lint에 걸린다. macOS 호스트에서는 native, linux, windows 세 단계가 돈다.
- Evidence:
  - commits a3867eff0, 4e8cc31c8
  - WSL Ubuntu: GOOS=windows CGO_ENABLED=0 golangci-lint run ./... -> 21 issues at 39b78191c, 0 issues at a3867eff0
  - scripts/lint-local.sh, scripts/ci_workflow_test.py (test_lint_local_*)
- Alternatives / rejected options:
  - CI workflow에 Windows lint job을 따로 추가: ci_workflow_test.py가 lint job이 lint-local.sh 하나만 부르도록 고정하고, 로컬과 CI의 lint가 갈라진다
  - GOOS별 nolint 지시문: nolintlint가 다른 GOOS의 lint에서 unused directive로 잡는다
  - Windows runner에서 lint: runner 시간이 더 들고 Linux에서 GOOS=windows lint가 같은 21건을 재현했다
