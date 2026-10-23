---
name: cmux-handoff.md
description: Opt-in issueops execution handoff-cmux launch of a native host into an existing canonical worktree.
---

# Explicit cmux handoff

cmux is an opt-in terminal launcher for an existing canonical worktree. It is
never part of automatic host selection and is not installed, started, or
required by `issueops install`, `update`, or `self-verify`. Follow
[`skills/issueops/references/session-choice-cmux.md`](../../../skills/issueops/references/session-choice-cmux.md)
only after the user explicitly requests cmux and the exact direct generation is
released.

The supported command is `issueops execution handoff-cmux`. Supply the observed
absolute cmux executable, exact `cmux 0.64.10 (90) [fafa50702]` build identity,
absolute Unix socket, exact window UUID, canonical `--cwd`, native host
executable, model/effort, and sealed prompt/material digests. Do not
run bare `cmux <path>` because it may start the app, and do not run `cmux omo`;
the generic terminal command executes the absolute native Omo binary. The
IssueOps preflight is limited to version, ping, capabilities, and exact-target
identify. It never discovers a socket or changes its permissions.

The prompt leaf must be a regular mode-0600 file owned by the current effective
UID, no larger than 64 KiB, and stable in owner, mode, size, and identity across
the same-FD read. The 64 KiB bound is shared by prompt reading and launcher
preparation because the native host receives the prompt as one argv string.
Directory ownership is outside this leaf rule; no-follow directory handles and
namespace identity checks protect traversal.

The command records `call_staged` before `new-workspace`, creates one workspace
with `--focus false`, resolves one exact surface, and sends once. A create or
send timeout/response loss is terminal for that lineage: inspect the returned
IDs and recovery directory read-only, do not rerun the command, switch launcher,
or close the workspace automatically. Raw-input acceptance and a bootstrap
process receipt remain delivery evidence; only the normal IssueOps claim CAS
creates authority. A process receipt is attached only when the bootstrap PID's
observed executable matches the expected host executable. Wrapper descendants
that cannot be proved stay explicitly unverified. cmux 0.64.10 exposes no
runtime/machine/server identity, and the recorded socket endpoint incarnation
is an observation of one path entry rather than proof of peer identity or full
socket-race closure. Before each Preflight, CreateWorkspace, and Send call, the
handler rereads the durable record and rechecks released status, generation,
request and process cwd, durable worktree, Git top-level, and the initially
pinned canonical-root filesystem identity. CreateWorkspace receives the
validated canonical path. The adapter cannot bind cmux's own path lookup to the
pinned directory descriptor, so the immediate recheck narrows but cannot remove
the path-to-cmux race. It cannot certify a live cmux × host row.
