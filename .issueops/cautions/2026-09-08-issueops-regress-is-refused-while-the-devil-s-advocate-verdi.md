---
name: 2026-09-08-issueops-regress-is-refused-while-the-devil-s-advocate-verdi
description: Caution record for a solved false case or recurring risk.
---

# issueops regress is refused while the devil's-advocate verdict is revise

- Date: 2026-09-08
- Kind: `caution`
- Source: cli
- Summary: The escape from a capped revise loop is stop, then remote reflection, then regress; calling regress directly fails.
- Context: issueops_regress.go rejects a regress when the current verdict is revise and additionally requires a recorded stop plus a non-empty IssueReflectedAt. The unwaived revise round cap fires exactly when the current verdict is revise, so an error message that pointed straight at regress would name a closed door.
- Resolution: Record a stop verdict, run issueops remote reflect-devils-advocate --confirm, then issueops regress --reason. Or take the round with --waive --waiver-rationale, which the implement gate accepts. A regress clears the review, so the cap counts from zero after re-planning.
