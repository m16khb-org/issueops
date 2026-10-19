---
name: 2026-10-18-gate-checks-that-run-go-tests-or-self-verify-need-lang-and-t
description: Caution record for a solved false case or recurring risk.
---

# Gate CHECKs that run Go tests or self-verify need LANG and the CI Python env

- Date: 2026-10-18
- Kind: `caution`
- Source: issueops-implement #568
- Summary: gates check passes only HOME and PATH by default, so git output follows a non-English system locale and git-parsing tests fail, and self-verify's Python suite fails without the CI requirements venv.
- Context: Issue #568 gate ledger: G3, G8 and G9 failed under `gates check` with `resolve git identity: exit status 128` and a Korean git error from TestMCPAuthorizeIssuesPreLeaseCapabilityAndGuardRechecksEachSpan and TestMemoizedReviewModelLoadsSettingsOncePerRepository; G11 isolated self-verify failed at `Python script tests` with `No module named 'pydantic'` from skills/slack-delegate/scripts/test_capability_routing.py. The same failures reproduce on base e65a621 with `env -i HOME PATH`, so they are environment gaps, not regressions.
- Resolution: Run ledger checks with `--env HOME,PATH,LANG` and with PATH led by a venv built like CI (`uv venv --python 3.13 DIR && uv pip install --python DIR/bin/python -r scripts/python_test_requirements.txt`, then `PATH=DIR/bin:$PATH`). The per-CHECK timeout cap is 15 minutes (`timeout_exceeds_15m`), so use `--timeout-seconds 900` for full race or self-verify CHECKs.
- Evidence:
  - env -i HOME PATH go test ./cmd/issueops/issueopsapp fails on base; adding LANG passes
  - python3 scripts/python_suite_runner.py on base: ModuleNotFoundError: No module named 'pydantic'
  - .github/workflows/ci.yml:44-46 builds the venv from scripts/python_test_requirements.txt
