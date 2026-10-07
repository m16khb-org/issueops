---
name: 2026-10-07-review-revise-and-regress-rounds-are-capped-at-five
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Review revise and regress rounds are capped at five

- Date: 2026-10-07
- Kind: `adr`
- Source: issueops-implement #550
- Summary: Unwaived devils-advocate revise verdicts per plan phase and stop-to-replan regressions per cycle are both capped at five instead of three; review effort still steps up from the third round.
- Context: The 2026-07-02 regress round cap and Decision (4) of the 2026-09-08 adversarial review throughput record both set the cap at three. Two recent cycles, #550 among them, passed plan review only on the third round, so a single extra defect would have forced a stop or a waiver. The user asked on 2026-10-07 to raise both caps to five ("5회까지로 늘려주면 좋겠어 3회는 너무 아슬아슬해") and chose to apply it to review and re-plan alike inside #550.
- Decision: reviseRoundCap (internal/domain/issueopsreview/devils_advocate.go) and regressCap (internal/domain/issueopsreview/regress.go) are 5: the fifth unwaived revise in a plan phase and the fifth regress in a cycle are accepted, the sixth is refused with the same error and exits as before. The issueops-review skill allows up to five review rounds and keeps the effort step-up from round three, so rounds three to five run one effort level higher. This record supersedes only the cap number in the 2026-07-02 regress round cap and in Decision (4) of the 2026-09-08 record; the RegressEvents audit trail, the human-decision escalation, the stop→reflect→regress exit, and the waiver path stay as decided there.
- Consequences: A cycle that stopped at three revises or three regressions can continue up to five; records and output schemas are unchanged, and the cap errors still report the actual count. The 2026-09-24 Claude role-model record's round-three effort step-up is unchanged and now covers rounds three to five. The skill text and README.en.md state five rounds; dated records keep the old number as history.
- Evidence:
  - internal/domain/issueopsreview/devils_advocate.go
  - internal/domain/issueopsreview/regress.go
  - internal/adapter/issueops/devilsadvocate/revise_round_cap_test.go
  - internal/adapter/issueops/issueops_regress_cap_test.go
  - skills/issueops-review/SKILL.md
  - README.en.md
  - https://github.com/m16khb-org/issueops/issues/550
- Alternatives / rejected options:
  - Raise only the revise cap: rejected, the user chose both caps
  - Make the cap configurable per repository: rejected, no second caller needs a different value
  - Escalate effort only from round four: rejected, the round-three step-up is what made the recent third rounds converge
