---
name: 2026-10-12-omp-role-models-resolve-from-the-agent-model-settings-files
description: Accepted decision record with rationale, alternatives, and consequences.
---

# omp and omo role models resolve from the agent model settings files

- Date: 2026-10-12
- Kind: `adr`
- Source: user request
- Summary: omp and omo join claude and codex as hosts the global and local agent-models settings files configure; every native host now reads the settings files.
- Context: The 2026-10-08 omp first-party host ADR kept omp role models on built-in defaults like Omo, and the 2026-10-08 role-model settings ADR left Omo on built-in defaults. An omp or Omo owner could only change the implement model by passing --owner-model and --owner-effort on every execution prepare. The user asked for global implement defaults per host: Claude Sonnet 5.5 high, omp Sonnet 5.5 high, Omo GPT-6.1 Sol high.
- Decision: Config gains omo and omp sections (json keys omo and omp). issueops model set/unset accept every native host, model show lists all four by default, and the resolver reads local then global layers before the built-in defaults for every host. execution prepare, issueops next review models, and owner packet review/research/reader-check models therefore read the settings files for omo and omp, and a broken file fails prepare before Orca state changes even when both owner flags are given. omo and omp still get no role-agent injection, and model ids keep each host's provider/model form (anthropic/claude-sonnet-5-5, chatgpt-subscription/gpt-6.1-sol); no model warning is emitted for them.
- Consequences: Supersedes in part the role-model bullet of the 2026-10-08 omp first-party host ADR and the "claude and codex are configurable; omo keeps built-in defaults" clause of the 2026-10-08 role-model settings ADR. Existing settings files stay valid; older builds reject a file that contains an omo or omp section as an unknown field, so the installed binary must be updated before such a file is written.
