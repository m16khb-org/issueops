---
name: 2026-09-08-review-checks-enter-the-ledger-at-stage-4-entry-never-during
description: Caution record for a solved false case or recurring risk.
---

# Review CHECKs enter the ledger at stage-4 entry, never during verify

- Date: 2026-09-08
- Kind: `caution`
- Source: cli
- Summary: Adding a review finding's CHECK to gates.md during the verify stage rewrites a file and makes the ai-slop-clean seal stale.
- Context: issueops-review asks the reviewer for a CHECK/EXPECT line per blocking finding. The ledger file itself is created by the single gates init at stage-4 entry (skills/issueops-implement), so there is no ledger to append to during plan review, and appending during verify would change the sealed change set.
- Resolution: Run review CHECKs with issueops verify-work during any review round. Put the surviving plan-review CHECKs into the stage-4 gates init spec as additional --gate arguments. Never call a gates command from the review skill, and never run gates check --write in the verify stage.
