---
name: 2026-10-08-tracked-intent-copies-quote-the-raw-request-and-local-self-v
description: Caution record for a solved false case or recurring risk.
---

# Tracked intent copies quote the raw request, and local self-verify cannot see the tracked-file scan

- Date: 2026-10-08
- Kind: `caution`
- Source: issueops-docs #555
- Summary: The intent.md tracked copy repeats the user's original request verbatim, so a quoted name of another private repository reached a committed file and failed the meeting-notes identified-fixture scan in CI; the local self-verify passed because that scan reads git ls-files and the copy was still untracked (#555).
- Context: In #555 the original request quoted a SessionStart hint from another repository, including that repository's name. phase --to implement wrote .issueops/issues/555/intent.md from the sealed intent, the change was committed, and the CI self-verify failed scripts/meeting_notes_skill_contract_test.py test_synthetic_fixture_family_has_no_identified_meeting_data. The local self-verify run before the commit had passed the same suite.
- Resolution: Before committing .issueops/issues/<n>/, read intent.md for names of other private repositories, people, or workspaces quoted from the request and replace them with a placeholder in the tracked copy (the sealed original stays ignored; only implement and ai-slop-clean transitions rewrite the copy). To reproduce the CI result before pushing, run scripts/python_suite_runner.py in a temporary detached worktree where the cycle materials are committed.
