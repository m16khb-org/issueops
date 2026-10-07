# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Validate issueops operating-document ownership and navigation."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from dataclasses import asdict
from pathlib import Path
from typing import Literal, cast
from urllib.parse import unquote, urlparse

from .project_docs_optimize.documentation_contract import (
    SCHEMA_VERSION,
    Manifest,
    Report,
    Violation,
    load_manifest,
    repo_path,
)

MARKDOWN_LINK = re.compile(r"!?\[[^\]]*]\(([^)]+)\)")
INLINE_CODE = re.compile(r"`[^`]*`")
DATED_RECORD = re.compile(r"^\d{4}-\d{2}-\d{2}-.+\.md$")
# Directories the issueops runtime writes into any target repository. They hold
# per-cycle artifacts, not operating documents, so no manifest entry is needed.
RUNTIME_DIRECTORIES = frozenset(
    {"issues", "gates", "verified-execution", "state", "evidence", "tmp"},
)
# Required and optional root documents that issueops project bootstrap may
# create (internal/domain/projectdoc/constants.go).
STANDARD_ROOT_DOCUMENTS = frozenset(
    {
        "ARCHITECTURE.md", "CAUTIONS.md", "COMMIT_POLICY.md", "CONSTITUTION.md",
        "CONVENTIONS.md", "TECH_STACK.md", "TESTING.md", "OPEN_API_SPEC.md",
        "ADR.md", "OPERATIONS.md", "AGENT_WORKFLOW.md", "VCS.md", "DESIGN.md",
    },
)

def parse_args(argv: list[str]) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Validate modular issueops operating documentation.",
    )
    _ = parser.add_argument("--root", type=Path, default=Path.cwd())
    _ = parser.add_argument(
        "--mode",
        choices=("report", "check"),
        default="check",
    )
    _ = parser.add_argument("--json", action="store_true", dest="json_output")
    return parser.parse_args(argv)


def line_count(path: Path) -> int:
    return len(path.read_text(encoding="utf-8").splitlines())


def markdown_targets(path: Path) -> tuple[str, ...]:
    targets: list[str] = []
    fenced = False
    for line in path.read_text(encoding="utf-8").splitlines():
        if line.lstrip().startswith("```"):
            fenced = not fenced
            continue
        if fenced:
            continue
        prose = INLINE_CODE.sub("", line)
        targets.extend(
            match.group(1).strip() for match in MARKDOWN_LINK.finditer(prose)
        )
    return tuple(targets)


def local_target(source: Path, raw_target: str) -> Path | None:
    target = raw_target.strip("<>").split(maxsplit=1)[0]
    parsed = urlparse(target)
    if parsed.scheme or target.startswith("#"):
        return None
    relative = unquote(target.split("#", maxsplit=1)[0])
    if not relative:
        return None
    return (source.parent / relative).resolve()


def visible_markdown(root: Path, docs_root: Path) -> tuple[list[Path], bool]:
    """Return .issueops Markdown files git does not ignore.

    The flag reports a tree whose every document is ignored, which would
    otherwise pass with nothing checked.
    """
    candidates = sorted(docs_root.rglob("*.md"))
    try:
        listed = subprocess.run(
            ["git", "-C", str(root), "ls-files", "--cached", "--others",
             "--exclude-standard", "-z", "--", ".issueops"],
            check=True,
            capture_output=True,
            text=True,
        ).stdout
    except (OSError, subprocess.CalledProcessError):
        return candidates, False
    visible = {(root / item).resolve() for item in listed.split("\0") if item}
    kept = [path for path in candidates if path.resolve() in visible]
    return kept, bool(candidates) and not kept


def placement_violations(
    root: Path,
    docs_root: Path,
    manifest: Manifest,
    markdown_files: list[Path],
) -> list[Violation]:
    """Check that every directory and document under .issueops has an owner."""
    violations: list[Violation] = []
    module_dirs = [repo_path(root, family.module_dir) for family in manifest.families]
    declared_dirs = [repo_path(root, path) for path in manifest.directories]
    open_dirs = [docs_root / name for name in ("documentation", *RUNTIME_DIRECTORIES)]
    # Parents of nested module dirs (operations/ for operations/guides) may hold
    # only documents listed in fixed_documents.
    family_parents = {
        module.relative_to(docs_root).parts[0]
        for module in module_dirs
        if len(module.relative_to(docs_root).parts) > 1
    }
    allowed_roots = {
        repo_path(root, path)
        for path in [
            *(family.root for family in manifest.families),
            *manifest.single_owner_topics.values(),
            *manifest.root_documents,
        ]
    }
    fixed = {repo_path(root, path) for path in manifest.fixed_documents}
    for declared in [*declared_dirs, *fixed,
                     *(repo_path(root, path) for path in manifest.root_documents)]:
        if not declared.exists():
            violations.append(
                Violation(
                    "missing_declared_path",
                    str(declared.relative_to(root)),
                    "manifest declares a path that does not exist",
                ),
            )
    reported_dirs: set[str] = set()
    for document in markdown_files:
        if not document.is_relative_to(docs_root):
            continue
        resolved = document.resolve()
        relative = document.relative_to(docs_root)
        path = str(document.relative_to(root))
        if len(relative.parts) == 1:
            if document.name not in STANDARD_ROOT_DOCUMENTS and resolved not in allowed_roots:
                violations.append(
                    Violation(
                        "undeclared_root_document",
                        path,
                        "root document is not a standard project doc, family root, "
                        + "single-owner topic, or manifest root_documents entry",
                    ),
                )
            continue
        module = next((m for m in module_dirs if document.is_relative_to(m)), None)
        if module is not None:
            if document.parent != module and DATED_RECORD.match(document.name):
                violations.append(
                    Violation(
                        "nested_record",
                        path,
                        f"dated record belongs directly under {module.relative_to(root)}",
                    ),
                )
            continue
        if resolved in fixed or any(
            document.is_relative_to(d) for d in [*open_dirs, *declared_dirs]
        ):
            continue
        top = relative.parts[0]
        if top in family_parents:
            violations.append(
                Violation(
                    "undeclared_document",
                    path,
                    "document sits beside a family module; move it into the module "
                    + "or list it in manifest fixed_documents",
                ),
            )
        elif top not in reported_dirs:
            reported_dirs.add(top)
            violations.append(
                Violation(
                    "undeclared_directory",
                    str((docs_root / top).relative_to(root)),
                    "directory is not a family module, runtime directory, or "
                    + "manifest directories entry",
                ),
            )
    return violations


def validate(root: Path, manifest: Manifest) -> Report:
    docs_root = root / ".issueops"
    violations: list[Violation] = []
    markdown_files, all_ignored = visible_markdown(root, docs_root)
    if all_ignored:
        violations.append(
            Violation(
                "docs_tree_ignored",
                ".issueops",
                "git ignores every .issueops document, so nothing would be checked",
            ),
        )
    visible = {path.resolve() for path in markdown_files}
    agents = root / "AGENTS.md"
    if agents.is_file():
        markdown_files.append(agents)

    for family in manifest.families:
        root_doc = repo_path(root, family.root)
        module_dir = repo_path(root, family.module_dir)
        if not root_doc.is_file():
            violations.append(
                Violation("missing_root", family.root, "required root index is missing"),
            )
            continue
        if line_count(root_doc) > manifest.max_root_lines:
            violations.append(
                Violation(
                    "line_budget_exceeded",
                    family.root,
                    f"root index exceeds {manifest.max_root_lines} lines",
                ),
            )
        if all_ignored:
            continue
        if not module_dir.is_dir():
            violations.append(
                Violation(
                    "missing_module_dir",
                    family.module_dir,
                    "module directory is missing",
                ),
            )
            continue
        modules = [m for m in sorted(module_dir.rglob("*.md")) if m.resolve() in visible]
        if not modules:
            violations.append(
                Violation(
                    "empty_module_dir",
                    family.module_dir,
                    "module directory has no Markdown documents",
                ),
            )
        for module in modules:
            if line_count(module) > manifest.max_module_lines:
                violations.append(
                    Violation(
                        "line_budget_exceeded",
                        str(module.relative_to(root)),
                        f"module exceeds {manifest.max_module_lines} lines",
                    ),
                )
        linked_paths = {
            target
            for raw in markdown_targets(root_doc)
            if (target := local_target(root_doc, raw)) is not None
        }
        if not any(
            target == module_dir or target.is_relative_to(module_dir)
            for target in linked_paths
        ):
            violations.append(
                Violation(
                    "module_dir_unlinked",
                    family.root,
                    f"root index does not link into {family.module_dir}",
                ),
            )

    violations.extend(
        placement_violations(root, docs_root, manifest, markdown_files),
    )

    for topic, owner in manifest.single_owner_topics.items():
        if not repo_path(root, owner).is_file():
            violations.append(
                Violation(
                    "missing_owner",
                    owner,
                    f"canonical owner for {topic!r} is missing",
                ),
            )

    for document in markdown_files:
        for raw in markdown_targets(document):
            target = local_target(document, raw)
            if target is not None and not target.exists():
                violations.append(
                    Violation(
                        "broken_link",
                        str(document.relative_to(root)),
                        f"target does not exist: {raw}",
                    ),
                )

    ordered = tuple(
        sorted(violations, key=lambda item: (item.path, item.code, item.message)),
    )
    return Report(
        ok=not ordered,
        schema_version=SCHEMA_VERSION,
        root=str(root),
        documents_checked=len(markdown_files),
        families_checked=len(manifest.families),
        violations=ordered,
    )


def render(report: Report, *, json_output: bool) -> None:
    if json_output:
        print(json.dumps(asdict(report), ensure_ascii=False, indent=2))
        return
    status = "ok" if report.ok else "failed"
    print(
        f"project docs check {status}: {report.documents_checked} documents, "
        + f"{len(report.violations)} violations",
    )
    for violation in report.violations:
        print(f"- [{violation.code}] {violation.path}: {violation.message}")


def main(argv: list[str]) -> int:
    args = parse_args(argv)
    root = cast("Path", args.root).resolve()
    mode = cast("Literal['report', 'check']", args.mode)
    json_output = cast("bool", args.json_output)
    try:
        manifest = load_manifest(
            root / ".issueops" / "documentation" / "manifest.json",
        )
        report = validate(root, manifest)
    except (OSError, TypeError, ValueError, json.JSONDecodeError) as error:
        report = Report(
            ok=False,
            schema_version=SCHEMA_VERSION,
            root=str(root),
            documents_checked=0,
            families_checked=0,
            violations=(Violation("invalid_input", str(root), str(error)),),
        )
    render(report, json_output=json_output)
    return 1 if mode == "check" and not report.ok else 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
