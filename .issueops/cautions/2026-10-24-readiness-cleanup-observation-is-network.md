---
name: 2026-10-24-readiness-cleanup-observation-is-network
description: Caution record for a solved false case or recurring risk.
---

# PR readiness includes a network cleanup observation; phase completion must not use it

- Date: 2026-10-24
- Kind: `caution`
- Source: autoresearch 2026-10-23 usability session
- Summary: `Readiness.PR` adds the cleanup status (`git status`, `branch`, `remote`, and a network `git ls-remote --heads`) for the `cleanup_ready`/`cleanup_missing` fields only `pr-readiness` prints. `status` (phase-ledger projection through `Readiness.Completion`) and `next` (`ObserveLocalPR`, a port documented as network-free) used the same method and paid the network round trip on every call.
- Context: Real transcripts showed `status --id` bimodal at ~75 ms vs ~2.4 s (median 2,393 ms). With a GitHub remote configured the bench reproduced it: `status` 550 ms, `next` 615 ms; without a remote the call was skipped, which hid it from local tests.
- Resolution: `Readiness.localPR` is the PR readiness without cleanup; `Completion` and `ObserveLocalPR` use it, `PR` (pr-readiness) adds cleanup on top. After: `status` 66 ms, `next` 152 ms with the same remote. When a readiness consumer reads only `ready`/`missing`, wire `localPR`; measure latency with a real remote configured (`git remote add origin <public https repo>`), not only in a remote-less fixture.
