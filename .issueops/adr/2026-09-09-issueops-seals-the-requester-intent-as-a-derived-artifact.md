---
name: 2026-09-09-issueops-seals-the-requester-intent-as-a-derived-artifact
description: Accepted decision record with rationale, alternatives, and consequences.
---

# IssueOps seals the requester intent as a derived artifact next to the plan

- Date: 2026-09-09
- Kind: `adr`
- Source: IssueOps #507
- Summary: `execution prepare` renders `record.intent` into `<artifact_dir>/intent.md` through the same immutable 0600 writer as plan/spec, seals its digest as `artifact_manifest.intent`, and the owner prompt, review, verify, and PR skills read that file as the requester intent contract while the remote issue body stays the implementation contract.
- Context: The intent contract (raw request, interpretation, success criteria, non-goals, constraints, ambiguities) was recorded once by `issueops intent record` and then read only by the grill gate, child delegation, and the plan review. The owner context packet carried the issue body but not the intent, the diff review received only the plan, the PR body's intent section was written from scratch, and the record lived in SQLite where a handed-over session or a reviewer could not open it as a file. A long cycle could therefore satisfy the issue body while drifting from what the requester actually asked.
- Decision: (1) `internal/domain/issueopsintent.Render` projects the intent into deterministic markdown with no clock, no JSON, and no contract import; the adapter maps the record into it. The recorded_at stamp is deliberately excluded because `intent record` renews it on every run and the sealed writer is immutable, so the same content re-recorded yields the same bytes. (2) `materializeStagedArtifacts` appends the `intent` entry after the staged loop, so direct and Orca share one code path; staging `--name intent` stays rejected. (3) `RecordIntent` rejects secret-like values on the rendered document at record time (the redaction path only covers the equals form); materialize keeps a second check for older records and delegated child intents and points the operator at `intent record`. (4) The owner prompt reads intent.md before implementing and reports a blocker without mutation when the issue body conflicts with it. (5) intent.md is the requester intent contract; the issue body remains the implementation contract SSOT.
- Consequences: A changed intent cannot be resealed in place: on Orca an operator deletes the sealed intent.md and runs `execution replace --reseed`, which renders the current record into the empty slot; a resume before that stops on the missing entry. Direct executions have no CLI path to regenerate intent.md after the seal, so readers fall back to `status --json` `.intent`. On Orca, `execution resume` without a reseed passes with a stale intent.md because manifest and file still agree; that window is accepted (no stale gate) and observable through `.intent.recorded_at`. Tracked INTENT.md copies, issue template sections, and completion publication of the intent were not adopted.
- Alternatives / rejected options:
  - Commit a tracked `.issueops/issues/<n>/INTENT.md` — rejected: two copies (record and file) diverge and the PR diff gains document noise; deferred until a reviewer-visibility need is shown.
  - Write the file from `intent record` into the source checkout — rejected: the worktree does not exist yet and the file would need relocation once the issue number is known.
  - Add an intent field to the context packet JSON — rejected: it changes the packet schema and ripples into Orca validation and goldens; the manifest is already a map.
  - Accept `intent` in `artifact stage` — rejected: a staged intent could diverge from the record.
  - First-seal pinning (reuse an existing intent.md without re-rendering) — rejected in review: it bypassed the immutable writer for intent only and laundered in-worktree edits into the next reseed.
- Evidence:
  - internal/domain/issueopsintent/render.go, render_test.go
  - internal/adapter/issueops/issueops_artifact_stage.go (materializeStagedArtifacts), issueops_intent_artifact_test.go
  - internal/adapter/issueops/intentdesign/intent_design.go (RecordIntent, IntentDocument), intent_secret_test.go
  - internal/adapter/issueops/testdata/execution_owner_prompt.txt, .issueops/prompt-engineering/prompts/issueops-v1-owner-execution-v1.md, execution_owner_prompt_intent_test.go
  - go test ./internal/domain/issueopsintent/ ./internal/adapter/issueops/ ./internal/adapter/issueops/intentdesign/ ./internal/architecture/ -count=1
