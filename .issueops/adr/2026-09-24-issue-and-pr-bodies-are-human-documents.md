---
name: 2026-09-24-issue-and-pr-bodies-are-human-documents
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Issue and PR bodies are human documents; implementation materials stay in .issueops/issues/<n>/

- Date: 2026-09-24
- Kind: `adr`
- Source: IssueOps #513, design `docs/superpowers/specs/2026-09-24-readable-issue-pr-design.md`
- Summary: Issue and PR bodies follow a summary-first contract of 4-5 required sections owned by `internal/domain/artifacttemplate`, and every `issueops remote` publish and sync command runs a readability check that refuses critical findings on `--confirm`. Hashes, commit SHAs, plan and spec texts, label scores, and cleanup audits leave the bodies: they live in the record and in tracked copies under `.issueops/issues/<n>/`.
- Context: Published bodies mixed three audiences: the team, the next agent session, and audit data. PRs were forced into 13 sections, every issue carried a label-score section, the completion region took 44-95% of sampled issue bodies with SHA-256 manifests and plan and spec full texts, and cleanup re-rendered that region to append an audit line with local paths. The Korean check was a skill script agents could skip, and a body posted through a provider MCP tool skipped every check.
- Decision:
  1. `artifacttemplate` owns the body contract (required and optional sections per kind, summary-first, placeholder rules, legacy `--field` aliases). `internal/domain/artifactreadability` is a pure check that restates the structural findings and adds Korean ratio, SHA-256 outside code, and nine warnings.
  2. `create-issue`, `create-child`, `create-pr`, `sync-issue`, and `sync-pr` always run both; previews report `readability` (sync also `live_readability`, warning-only), confirms refuse critical findings before any provider call. `create-issue --confirm` requires `--template` and refuses before the issue-create intent is sealed. `create-child` and `create-pr` default to their single template, so the owner packet's `create-pr` command is unchanged.
  3. The completion region is a human-written progress report: `reflect-completion --body-file`, required with `--confirm`, checked as a completion draft (hashes, full commit SHAs, and local paths critical even inside code, because the draft is rendered verbatim; 2,000-character cap). It still requires provider-verified merge evidence and stays in post-merge cleanup.
  4. `cleanup finish` and `cleanup remote-branch` no longer write the issue body; the audit line is reported in the response `audit` only. A branch kept with `--keep-remote-branch` stays traceable from the issue because `branch prepare` names it `<issue number>-<slug>` and the provider links it to the issue.
  5. The plan-review region shows one Korean flow line over every round, lists reasons only for a stop, and masks hashes and local paths. It adds no refusal, so `regress` keeps working for short stop findings.
  6. The `implement` and `ai-slop-clean` transitions write tracked copies (`plan.md`, `intent.md`, `spec.md`, `plan-review.md`) into `.issueops/issues/<n>/`. This partially supersedes the 2026-09-09 rejection of a tracked intent file: reviewer visibility is now required. The sealed originals under `artifact/` stay ignored; a linked plan outside `artifact/` is already tracked and is not copied over; the transition reports copies in its return value and the phase JSON (`tracked_materials`), never in the record.
  7. "No provider adapter API change" means the `IssueProvider` method set and provider CLI invocation stay the same. The managed-section DTOs change: `IssueProviderCompletionSection` keeps the PR URL and adds `ResultBody`; `IssueProviderUpdateIssueBodySectionRequest` adds the plan-review verdict and rounds.
- Consequences: Old-format bodies cannot be confirmed any more; they are rewritten in the new contract when next synced. Cycles that entered implement before this change have no tracked copies; `cleanup finish` preview and `reflect-completion` warn with `tracked_materials_missing`. Until the first commit after implement entry, the untracked copies keep `worktree_clean` false, so the implement skill commits them first. Tracked copies alone do not satisfy `implementation_changes`. JSON consumers of `audit_reflected`/`audit_error` read `audit`.
- Alternatives / rejected options:
  - Keep the check in the skill script — rejected: it is skippable, which is the failure this decision fixes.
  - Make harness terms critical — rejected: issueops issues discuss those terms and would be refused falsely.
  - Store the progress report or the template kind in the record — rejected: the record decoder rejects unknown fields, so older builds could not read new records.
  - Reflect completion at `execution complete` — rejected: `reflect-completion` requires merge evidence, which does not exist before merge.
  - Write tracked copies at `link-plan` — rejected: prepare fills `plan_path` in both modes, so link-plan is skipped.
  - Refuse short plan-review findings — rejected: a short stop must still reflect so `regress` can proceed.
- Evidence:
  - internal/domain/artifacttemplate/template.go, template_test.go, repo_templates_test.go
  - internal/domain/artifactreadability/readability.go, readability_test.go
  - cmd/issueops/issueopscli/remotecmd/remote_child_pr.go, remote_readability.go, remote_readability_test.go
  - internal/adapter/issueops/issueops_remote_body_sync.go, issueops_completion_remote.go, issueops_cleanup_finish.go, issueops_cleanup_remote_branch.go, issueops_materials.go, issueops_phase.go
  - internal/adapter/provider/issuebody/issue_body_section.go
  - go test ./... -count=1
