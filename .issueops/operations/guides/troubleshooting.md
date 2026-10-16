---
name: troubleshooting
description: Health gate, diagnosis, and one-time reconciliation procedures.
---

# Troubleshooting and Reconciliation

This guide owns the cross-system health gate and the one-time reconciliation
procedure. Canonical index: [../../OPERATIONS.md](../../OPERATIONS.md).

## Operational Health and One-Time Reconciliation

`issueops doctor` is the sole public cross-system health gate for canonical Git state, all IssueOps records/bindings, and unexpected user-state artifacts. Invocation-only preservation never writes state:

```bash
issueops doctor --repo . --preserve-cycle EXACT_CYCLE_ID --json
```

- An active IssueOps execution is live only with a complete generation, native process receipt, and canonical worktree. Process absence alone never authorizes interrupt, deletion, or lease replacement; use the previewed generation-CAS replacement sequence and prove quiescence.
- The preserve flag is a repeatable exact value for one doctor invocation. It does not create a persistent exception or cure incomplete/duplicate identity.
- The stability audit builds the binary, then delegates operational judgement to `doctor`.

The approved one-time full reconciliation uses an external mode-`0700` bundle at `~/.local/state/issueops-backups/<repo-fingerprint>/<UTC-timestamp>/`, not a product cleanup command. Git and SQLite backups are restore-tested. Stop before reset if the final full digest drifts. After a reset or crash seam, resume the sealed append-only journal and complete idempotently forward; do not infer a partial rollback.

## Related references

- [operations/guides/stability-baseline.md](stability-baseline.md): stability audit
  baseline that builds the binary then delegates to `doctor`.
- [issueops-execution.md](issueops-execution.md): generation-CAS replacement and
  recovery for failed execution holders.
