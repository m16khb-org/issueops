# Historical Comparison Context

Optional background moved from SKILL.md. The table below contains inherited
baseline estimates and targets, not measurements from this refactor. Current
actor obligations and operational metric formulas remain in SKILL.md; these
comparison ratios do not replace per-criterion metrics or prove an outcome.

## Comparison with ulw-loop

Original comparison target: **20%+ improvement over ulw-loop** on every dimension.

| Metric | ulw-loop baseline | Verified Execution target | Measurement |
|--------|------------------|---------------|-------------|
| **Evidence Coverage** | ~70% (some criteria lack observable evidence) | ≥95% (every criterion has a channel artifact) | `criteria_with_evidence / total_criteria` |
| **Rework Rate** | ~30% (worker outputs rejected on integration) | ≤15% (better task specs reduce rework) | `respawned_tasks / total_tasks` |
| **Cycle Efficiency** | ~60% (blocked criteria waste cycles) | ≥80% (dependency ordering prevents blocks) | `completed_criteria / total_attempts` |
| **Parallelization Ratio** | ~2x (manual wave grouping) | ≥4x (dependency-matrix-driven waves) | `total_tasks / wave_count` |
| **Cleanup Compliance** | ~50% (cleanup receipts often missing) | 100% (no pass without receipt) | `cleanup_receipts / qa_scenarios` |
| **Cross-Session Survival** | None (filesystem-only, no state checkpoints) | 100% (issueops state survives compaction) | `resumed_sessions / total_sessions` |
| **Host Portability** | Codex-only host assumptions | 2 hosts (Codex and Claude unified skill) | Host-specific section translates available tools |
