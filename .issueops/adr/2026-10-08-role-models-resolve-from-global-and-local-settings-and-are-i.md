---
name: 2026-10-08-role-models-resolve-from-global-and-local-settings-and-are-i
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Role models resolve from global and local settings and are injected into owner sessions

- Date: 2026-10-08
- Kind: `adr`
- Source: issueops-docs #553
- Summary: IssueOps resolves the model and effort of seven roles per host from flag, main-worktree local settings, XDG global settings, and built-in defaults, and injects the review, research, and reader-check roles into the owner sessions it launches.
- Context: Before #553 the model of every IssueOps role was a constant in internal/domain/agentmodel, review-round escalation lived only in skill prose, and users could not choose models per host or per repository. The 2026-09-24 ADR fixed the Claude defaults (Sonnet implements, Opus plans and reviews, Fable manual-only).
- Decision: (1) Roles are implement, child-implement, plan-review, diff-review, review-escalate, research, reader-check. internal/domain/agentmodel.Resolve applies flag > local > global > built-in per field (model, effort). child-implement inherits implement; docs-only lowers only a built-in review effort; rounds 3 and later use review-escalate settings or one effort rung above the review, capped at the host maximum. (2) Settings are JSON (TECH_STACK encoding/json): global at $XDG_CONFIG_HOME/issueops/agent-models.json or ~/.config/issueops/agent-models.json, local at the main worktree's .issueops/agent-models.local.json, registered in <git-common-dir>/info/exclude and never committed. Linked worktrees read the main worktree's file. A file under .issueops/ that git ignores is outside the docs layout check. (3) The env layer of the fixed config precedence stays empty for now. claude and codex are configurable; omo keeps built-in defaults. (4) `issueops model show|set|unset|resolve` is the only write surface; no MCP tool is added. Unknown models only warn; malformed host, role, or effort is a usage error. (5) Orca and cmux owner sessions receive the plan-review, diff-review, review-escalate, research, and reader-check roles at launch: Claude through one --agents JSON, Codex through -c agents.issueops-<role>.config_file pointing at content-addressed files under the state dir. Prepare, resume, and reconcile compute these arguments before MarkInvoking or BeginIntent, never seal them in the intent, and fail without an Orca call when settings are broken. (6) A broken settings file never falls back silently: prepare and `issueops model` fail before changing state, `issueops next` warns and blanks review.model and review.effort. (7) Built-in defaults: Claude implement claude-opus-5-5/high (replacing Sonnet from the 2026-09-24 ADR), plan-review and diff-review claude-opus-5-5/high, research claude-sonnet-5-5/medium, reader-check claude-haiku-5-5/medium; Codex implement gpt-6.1-sol/high, reviews gpt-6-astra/high (was xhigh), research gpt-6-luna/medium, reader-check gpt-6-luna/low. Fable never appears in defaults, inheritance, tiers, or escalation; users may still set it explicitly.
- Consequences: The 2026-09-24 Claude role-model ADR is superseded in part (the implement default is now Opus; Fable remains manual-only). Sealed Orca bindings keep their owner model; settings apply from the next execution prepare. Owner packets gain reader_check_model/effort; older packets still decode. The state-policy §6 and runtime location table now list the settings files.
- Evidence:
  - internal/domain/agentmodel/resolve.go, validate.go, defaults.go
  - internal/adapter/outbound/agentmodelconfig/config.go
  - cmd/issueops/modelcli/model.go
  - internal/adapter/hostprotocol/role_agents.go
  - internal/application/issueopspreparation/prepare.go, internal/application/issueopslease/resume.go, reconcile.go
  - .issueops/issues/553/live-check.md (claude subagent ran claude-sonnet-5-5; codex child thread gpt-6-luna/low)
  - issue #553
- Alternatives / rejected options:
  - Write native agent definition files under ~/.claude/agents or ~/.codex/agents: cannot express per-repository settings, drifts from the settings source, and copying them into worktrees pollutes the change fingerprint.
  - TOML settings files: conflicts with the encoding/json decision in TECH_STACK.md and needs a parser.
  - Only pass model aliases to the Agent tool: no effort control and no full model IDs.
