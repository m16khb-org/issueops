#!/usr/bin/env python3

from __future__ import annotations

import importlib.util
import tempfile
import unittest
from pathlib import Path


SCRIPT = Path(__file__).with_name("validate-skill.py")
SPEC = importlib.util.spec_from_file_location("validate_skill", SCRIPT)
assert SPEC is not None
validate_skill = importlib.util.module_from_spec(SPEC)
assert SPEC.loader is not None
SPEC.loader.exec_module(validate_skill)


class ValidateSkillTest(unittest.TestCase):
    def write_skill(self, body: str) -> Path:
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name) / "sample-skill"
        root.mkdir()
        (root / "SKILL.md").write_text(body, encoding="utf-8")
        return root

    def test_validates_quoted_description_without_pyyaml(self) -> None:
        skill = self.write_skill(
            '---\n'
            'name: sample-skill\n'
            'description: "Validate a description with: colon and \\"quotes\\"."\n'
            '---\n'
        )

        ok, message = validate_skill.validate_skill(skill)

        self.assertTrue(ok)
        self.assertEqual(message, "Skill is valid!")

    def test_rejects_unexpected_key(self) -> None:
        skill = self.write_skill(
            "---\n"
            "name: sample-skill\n"
            "description: sample\n"
            "unexpected: value\n"
            "---\n"
        )

        ok, message = validate_skill.validate_skill(skill)

        self.assertFalse(ok)
        self.assertIn("Unexpected key", message)

    def test_rejects_bad_name(self) -> None:
        skill = self.write_skill(
            "---\n"
            "name: Bad_Name\n"
            "description: sample\n"
            "---\n"
        )

        ok, message = validate_skill.validate_skill(skill)

        self.assertFalse(ok)
        self.assertIn("hyphen-case", message)

    def test_rejects_plain_description_that_strict_yaml_rejects(self) -> None:
        # Hosts load frontmatter with a strict YAML parser; "Triggers: x" in a
        # plain scalar is a mapping-value error there and drops the skill.
        for description in ("Use it. Triggers: a, b.", "Use it #now"):
            ok, message = validate_skill.validate_skill(self.write_skill(self.frontmatter(description)))
            self.assertFalse(ok, description)
            self.assertIn("quote the value", message)
        ok, _ = validate_skill.validate_skill(self.write_skill(self.frontmatter("'Use it. Triggers: a, b.'")))
        self.assertTrue(ok)

    def frontmatter(self, description: str = "sample") -> str:
        return f"---\nname: sample-skill\ndescription: {description}\n---\n"

    def test_description_limit_is_inclusive(self) -> None:
        ok, _ = validate_skill.validate_skill(self.write_skill(self.frontmatter("d" * 1024)))
        self.assertTrue(ok)

        ok, message = validate_skill.validate_skill(self.write_skill(self.frontmatter("d" * 1025)))
        self.assertFalse(ok)
        self.assertIn("Description is too long", message)

    def test_body_token_budget_counts_hangul_per_syllable(self) -> None:
        # 5000 Hangul syllables sit exactly on the budget; one more exceeds it.
        ok, _ = validate_skill.validate_skill(self.write_skill(self.frontmatter() + "가" * 5000))
        self.assertTrue(ok)

        ok, message = validate_skill.validate_skill(self.write_skill(self.frontmatter() + "가" * 5001))
        self.assertFalse(ok)
        self.assertIn("about 5001 tokens", message)

    def test_body_token_budget_counts_four_other_characters_per_token(self) -> None:
        ok, _ = validate_skill.validate_skill(self.write_skill(self.frontmatter() + "a" * 20000))
        self.assertTrue(ok)

        ok, message = validate_skill.validate_skill(self.write_skill(self.frontmatter() + "a" * 20001))
        self.assertFalse(ok)
        self.assertIn("about 5001 tokens", message)

    def test_body_line_budget_excludes_frontmatter(self) -> None:
        ok, _ = validate_skill.validate_skill(self.write_skill(self.frontmatter() + "x\n" * 499))
        self.assertTrue(ok)

        ok, message = validate_skill.validate_skill(self.write_skill(self.frontmatter() + "x\n" * 500))
        self.assertFalse(ok)
        self.assertIn("500 lines", message)


if __name__ == "__main__":
    unittest.main()
