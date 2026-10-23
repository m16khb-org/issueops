#!/usr/bin/env python3
"""
Validate issueops skill metadata without relying on host-managed system
skill copies or optional PyYAML installation.
"""

from __future__ import annotations

import ast
import math
import re
import sys
from pathlib import Path


MAX_SKILL_NAME_LENGTH = 64
MAX_DESCRIPTION_LENGTH = 1024
# Agent Skills specification (https://agentskills.io/specification), progressive
# disclosure: the SKILL.md body is loaded whole on activation, "< 5000 tokens
# recommended" and "under 500 lines"; detail belongs in referenced files.
MAX_BODY_TOKENS = 5000
MAX_BODY_LINES = 500
ALLOWED_PROPERTIES = {"name", "description", "license", "allowed-tools", "metadata"}
HANGUL_SYLLABLE = re.compile(r"[\uac00-\ud7a3]")


def estimate_tokens(text: str) -> int:
    """Return a tokenizer-free estimate: one token per Hangul syllable and one
    per four other characters. It is a budget gate, not a billing count."""
    hangul = len(HANGUL_SYLLABLE.findall(text))
    return hangul + math.ceil((len(text) - hangul) / 4)


class FrontmatterError(ValueError):
    pass


def _parse_scalar(raw: str):
    value = raw.strip()
    if value == "":
        return ""
    if value in {"true", "True"}:
        return True
    if value in {"false", "False"}:
        return False
    if value in {"null", "Null", "NULL", "~"}:
        return None
    if value[0] in {"'", '"'}:
        try:
            return ast.literal_eval(value)
        except (SyntaxError, ValueError) as exc:
            raise FrontmatterError(f"invalid quoted scalar: {exc}") from exc
    # Hosts parse frontmatter as strict YAML: a plain scalar may not contain
    # ": " (mapping value) or " #" (comment), so such a value must be quoted.
    if ": " in value or value.endswith(":") or " #" in value:
        raise FrontmatterError("plain scalar contains ': ' or ' #'; quote the value")
    return value


def parse_frontmatter(text: str) -> dict[str, object]:
    data: dict[str, object] = {}
    current_key = ""
    current_block: list[str] = []

    def flush_block() -> None:
        nonlocal current_key, current_block
        if current_key:
            data[current_key] = "\n".join(current_block).rstrip("\n")
            current_key = ""
            current_block = []

    for line_no, line in enumerate(text.splitlines(), start=1):
        if current_key:
            if line.startswith((" ", "\t")) or line.strip() == "":
                current_block.append(line)
                continue
            flush_block()

        if line.lstrip().startswith("#") or line.strip() == "":
            continue
        if ":" not in line:
            raise FrontmatterError(f"line {line_no}: expected key: value")
        key, raw_value = line.split(":", 1)
        key = key.strip()
        if not key:
            raise FrontmatterError(f"line {line_no}: empty key")
        if key in data:
            raise FrontmatterError(f"line {line_no}: duplicate key {key!r}")
        if raw_value.strip() == "":
            current_key = key
            current_block = []
            continue
        data[key] = _parse_scalar(raw_value)

    flush_block()
    return data


def validate_skill(skill_path: Path) -> tuple[bool, str]:
    skill_md = skill_path / "SKILL.md"
    if not skill_md.exists():
        return False, "SKILL.md not found"

    content = skill_md.read_text(encoding="utf-8")
    if not content.startswith("---"):
        return False, "No YAML frontmatter found"

    match = re.match(r"^---\n(.*?)\n---", content, re.DOTALL)
    if not match:
        return False, "Invalid frontmatter format"

    try:
        frontmatter = parse_frontmatter(match.group(1))
    except FrontmatterError as exc:
        return False, f"Invalid frontmatter: {exc}"

    unexpected_keys = set(frontmatter) - ALLOWED_PROPERTIES
    if unexpected_keys:
        allowed = ", ".join(sorted(ALLOWED_PROPERTIES))
        unexpected = ", ".join(sorted(unexpected_keys))
        return (
            False,
            f"Unexpected key(s) in SKILL.md frontmatter: {unexpected}. "
            f"Allowed properties are: {allowed}",
        )

    if "name" not in frontmatter:
        return False, "Missing 'name' in frontmatter"
    if "description" not in frontmatter:
        return False, "Missing 'description' in frontmatter"

    name = frontmatter.get("name", "")
    if not isinstance(name, str):
        return False, f"Name must be a string, got {type(name).__name__}"
    name = name.strip()
    if name:
        if not re.match(r"^[a-z0-9-]+$", name):
            return (
                False,
                f"Name '{name}' should be hyphen-case "
                "(lowercase letters, digits, and hyphens only)",
            )
        if name.startswith("-") or name.endswith("-") or "--" in name:
            return (
                False,
                f"Name '{name}' cannot start/end with hyphen or contain consecutive hyphens",
            )
        if len(name) > MAX_SKILL_NAME_LENGTH:
            return (
                False,
                f"Name is too long ({len(name)} characters). "
                f"Maximum is {MAX_SKILL_NAME_LENGTH} characters.",
            )

    description = frontmatter.get("description", "")
    if not isinstance(description, str):
        return False, f"Description must be a string, got {type(description).__name__}"
    if not description.strip():
        return False, "Description cannot be empty"
    if len(description.strip()) > MAX_DESCRIPTION_LENGTH:
        return (
            False,
            f"Description is too long ({len(description.strip())} characters). "
            f"Maximum is {MAX_DESCRIPTION_LENGTH} characters.",
        )

    body = content[match.end():].removeprefix("\n")
    body_lines = body.count("\n")
    if body_lines >= MAX_BODY_LINES:
        return (
            False,
            f"SKILL.md body has {body_lines} lines; keep it under {MAX_BODY_LINES} "
            "and move detail into references/",
        )
    body_tokens = estimate_tokens(body)
    if body_tokens > MAX_BODY_TOKENS:
        return (
            False,
            f"SKILL.md body is about {body_tokens} tokens; keep it at or under "
            f"{MAX_BODY_TOKENS} and move detail into references/",
        )

    return True, "Skill is valid!"


def main(argv: list[str]) -> int:
    if len(argv) < 2:
        print("Usage: validate-skill.py <skill-path> [<skill-path>...]", file=sys.stderr)
        return 2

    failed = False
    for arg in argv[1:]:
        skill_path = Path(arg)
        ok, message = validate_skill(skill_path)
        if len(argv) == 2:
            print(message)
        else:
            status = "ok" if ok else "error"
            print(f"{skill_path}: {status}: {message}")
        failed = failed or not ok
    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
