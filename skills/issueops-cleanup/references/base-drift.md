# Base drift at finish

Read from `SKILL.md` precondition 7 when preview reports `base_branch_drifted`,
`retargeted_base`, or `merged_base_remote_unobserved`.

Prepared and provider-observed merged base must match (`base_branch_drifted`).
Exemption: the prepared base is observed **absent from origin** AND the merged
base is the repository default. Preview reports `retargeted_base` with both
bases, default branch and absence evidence. Failed observation blocks with
`merged_base_remote_unobserved`; no flag may assert a base.
A deliberate retarget must be recorded **before** finish with
`issueops branch retarget --id ID --base-branch REF --reason TEXT`: it accepts
only a base the provider currently shows as the PR/MR target and that exists on
`origin`, then appends `branch_prepare.retargets[]` and moves the prepared base,
so finish compares against that recorded decision.
Check absence with `git ls-remote --heads origin
refs/heads/<prepared-base>` (empty output means gone); do not reach for
`ls-remote --symref`, which the command policy rejects.
