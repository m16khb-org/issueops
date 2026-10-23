---
name: 2026-10-24-readiness-cleanup-observation-is-network
description: Caution record for a solved false case or recurring risk.
---

# PR readiness carried a network cleanup observation that status and next paid for

- Date: 2026-10-24
- Kind: `caution`
- Source: autoresearch 2026-10-23 usability session
- Summary: `Readiness.PR` added the cleanup status (`git status`, `branch`, `remote`, and a network `git ls-remote --heads`) for `cleanup_ready`/`cleanup_missing` fields that no skill or command read, and that could never be ready before the merge (the request was always `Merged: false`). `status` (phase-ledger projection through `Readiness.Completion`), `next` (`ObserveLocalPR`, a port documented as network-free), and `pr-readiness` all paid the network round trip.
- Context: Real transcripts showed `status --id` bimodal at ~75 ms vs ~2.4 s (median 2,393 ms). With a GitHub remote configured the bench reproduced it: `status` 550 ms, `next` 615 ms; without a remote the call was skipped, which hid it from local tests.
- Resolution: PR readiness no longer observes cleanup; the fields and the `Readiness.Cleanup` dependency are gone, and `issueops cleanup status` alone owns post-merge cleanup readiness. `NewReadinessGitObservations` now only dedupes readiness's own branch/status reads per operation. After, with the same remote: `status` 550→66 ms, `next` 615→152 ms, non-strict `pr-readiness` makes no git call. Measure latency with a real remote configured (`git remote add origin <public https repo>`), not only in a remote-less fixture, and do not add network observations to a readiness method `status`/`next` reach.
