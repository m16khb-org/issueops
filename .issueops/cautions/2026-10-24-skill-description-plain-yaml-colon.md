---
name: 2026-10-24-skill-description-plain-yaml-colon
description: Caution record for a solved false case or recurring risk.
---

# A skill description with ": " in a plain YAML scalar is dropped by strict hosts

- Date: 2026-10-24
- Kind: `caution`
- Source: autoresearch 2026-10-23 usability session
- Summary: `description: Use it. Triggers: a, b.` is not valid YAML (a plain scalar may not contain `: ` or ` #`). Codex parses SKILL.md frontmatter strictly and skips the skill ("Skipped N skill(s) due to invalid SKILL.md files" in the vendor binary); Claude Code loads it with all frontmatter fields dropped. A description rewrite that adds "Triggers:" or "Merged cycles:" silently removes the skill from routing on those hosts.
- Context: The 2026-10-23 description diet introduced this in diagram-design, issueops-abandon, and issueops-cleanup. `scripts/validate-skill.py` used a lenient line parser and reported them valid; a strict check (`yaml.safe_load` on the frontmatter) failed all three.
- Resolution: Quote such descriptions with single quotes (they contain double quotes). `scripts/validate-skill.py` now rejects a plain scalar containing `: `, ` #`, or a trailing `:` with "quote the value" (`scripts/validate_skill_test.py`). Re-check all skills with `python3 scripts/validate-skill.py skills/*`.
