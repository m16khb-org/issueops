#!/usr/bin/env python3
"""Render `issueops list --json` as one self-contained HTML cycle board.

Reads the list JSON from stdin (UTF-8) and writes a single HTML file with
inline CSS to --out. No JavaScript, no external URL reference, standard
library only. The board only displays data and never runs the `next` commands
it prints. Input from `issueops view --view timeline|graph --json` (it has a
`view` key) is rendered as that view's page instead.
"""
from __future__ import annotations

import argparse
import html
import json
import re
import shlex
import sys
import urllib.parse
from pathlib import Path

# Every color of the board lives in this dict. The CSS below is generated from
# it, so no other place in this file writes a color value.
# Pairs are (background, text).
_BLUE = ("#ddf4ff", "#0a3069")
_YELLOW = ("#fff8c5", "#4d2d00")
_PURPLE = ("#fbefff", "#4a1d73")
_GREEN = ("#dafbe1", "#0f4d24")
_RED = ("#ffebe9", "#82071e")
_ORANGE = ("#fff1e5", "#7a2e00")
_GRAY = ("#e6ebf1", "#1f2328")

COLORS = {
    "page": {"background": "#f6f7f9", "surface": "#ffffff", "text": "#1f2328", "muted": "#59636e", "border": "#d1d9e0", "head": "#eaeef2", "link": "#0550ae", "code": "#eef1f4"},
    "phase": {
        "problem": ("#ddf4ff", "#0a3069"),
        "grill": ("#ddf4ff", "#0a3069"),
        "plan": ("#fff8c5", "#4d2d00"),
        "compatibility-review": ("#fff8c5", "#4d2d00"),
        "implement": ("#fbefff", "#4a1d73"),
        "ai-slop-clean": ("#fbefff", "#4a1d73"),
        "feedback": ("#ffeff7", "#6e1146"),
        "pr": ("#dafbe1", "#0f4d24"),
        "done": ("#e6ebf1", "#1f2328"),
        "cleanup": ("#e6ebf1", "#1f2328"),
        "default": ("#eaeef2", "#1f2328"),
    },
    "badge": {
        "pending": ("#fff8c5", "#4d2d00"),
        "failed": ("#ffebe9", "#82071e"),
        "cleanup-failed": ("#ffebe9", "#82071e"),
        "issue-create": ("#ffebe9", "#82071e"),
        "claimable": ("#ddf4ff", "#0a3069"),
        "cleanup": ("#e6ebf1", "#1f2328"),
        "unreflected": ("#fff1e5", "#7a2e00"),
        "invalid": ("#82071e", "#ffffff"),
    },
    # Timeline event kinds, grouped by meaning; any other kind uses "default".
    "event": {
        **dict.fromkeys(("cycle_started", "intent_recorded", "delegated", "issue_create_started"), _BLUE),
        **dict.fromkeys(("phase_entered", "phase_completed", "regressed", "skill_routed"), _YELLOW),
        **dict.fromkeys(("decision", "review", "ai_slop_cleaned"), _PURPLE),
        **dict.fromkeys(("pr_verified", "body_synced", "completed", "merged", "remote_reflected", "issue_closed", "child_validated"), _GREEN),
        **dict.fromkeys(("execution_failed", "cleanup_failed"), _RED),
        **dict.fromkeys(("completion_reopened", "base_synced", "child_created"), _ORANGE),
        **dict.fromkeys(("lease_claimed", "lease_released", "lease_replaced"), _GRAY),
        "default": ("#eaeef2", "#1f2328"),
    },
    # Graph edge states. not_reported is not a freshness proof, so it is not green.
    "state": {
        "stale": _RED,
        "not_reported": _GRAY,
        "unknown": _ORANGE,
        "default": ("#eaeef2", "#1f2328"),
    },
    # (minimum idle days, (background, text)), checked from the largest bound.
    "idle": (
        (30, ("#ffebe9", "#82071e")),
        (14, ("#fff1e5", "#7a2e00")),
        (7, ("#fff8c5", "#4d2d00")),
        (0, ("#dafbe1", "#0f4d24")),
    ),
}

NEXT_COMMAND = "issueops next --id {id} --json"
PATH_SPLIT = re.compile(r"[\\/]+")
URL_FORBIDDEN = re.compile(r"[\s\x00-\x1f\x7f]")


def esc(value: object) -> str:
    """Escape any value for HTML text and attribute positions."""
    return html.escape(str(value), quote=True)


def text_field(entry: dict, key: str) -> str:
    """Return a non-empty string field, or '' when absent or not a string."""
    value = entry.get(key)
    return value if isinstance(value, str) else ""


def idle_days(entry: dict) -> int | None:
    """Return idle_days only when it is a non-negative integer (bool excluded)."""
    value = entry.get("idle_days")
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        return None
    return value


def sort_key(entry: dict) -> tuple[int, int, str]:
    """idle_days descending (missing last), then id ascending."""
    idle = idle_days(entry)
    ident = str(entry.get("id", ""))
    return (1, 0, ident) if idle is None else (0, -idle, ident)


def build_css() -> str:
    page = COLORS["page"]
    rules = [
        "body{margin:0;padding:16px;font-family:system-ui,'Malgun Gothic',sans-serif;font-size:14px;"
        f"background:{page['background']};color:{page['text']}}}",
        "h1{font-size:18px;margin:0 0 8px}",
        f".summary{{margin:0 0 12px;color:{page['muted']}}}",
        f"table{{border-collapse:collapse;width:100%;background:{page['surface']};border:1px solid {page['border']}}}",
        f"th,td{{padding:6px 8px;border-bottom:1px solid {page['border']};text-align:left;vertical-align:top}}",
        f"th{{background:{page['head']};white-space:nowrap}}",
        f"code{{background:{page['code']};padding:1px 4px;border-radius:3px;white-space:nowrap}}",
        f"a{{color:{page['link']}}}",
        f".muted{{color:{page['muted']}}}",
        ".pill{display:inline-block;padding:1px 8px;border-radius:10px;margin:0 4px 2px 0;white-space:nowrap}",
        ".idle{text-align:right}",
        "ul.tree{margin:4px 0;padding-left:20px}",
    ]
    for group in ("phase", "badge", "event", "state"):
        for name, (bg, fg) in COLORS[group].items():
            rules.append(f".{group}-{name}{{background:{bg};color:{fg}}}")
    for index, (_, (bg, fg)) in enumerate(COLORS["idle"]):
        rules.append(f".idle-band-{index}{{background:{bg};color:{fg}}}")
    return "\n".join(rules)


def repo_cell(repo: str) -> str:
    if not repo:
        return "<td></td>"
    parts = [part for part in PATH_SPLIT.split(repo) if part]
    short = parts[-1] if parts else repo
    return f'<td title="{esc(repo)}">{esc(short)}</td>'


def phase_cell(phase: str) -> str:
    name = phase if phase in COLORS["phase"] else "default"
    return f'<td><span class="pill phase-{name}">{esc(phase)}</span></td>'


def lease_cell(entry: dict) -> str:
    status = text_field(entry, "lease_status")
    host = text_field(entry, "holder_host")
    pieces = [esc(status)] if status else []
    if host:
        pieces.append(f'<span class="muted">@{esc(host)}</span>')
    return f"<td>{' '.join(pieces)}</td>"


def idle_cell(entry: dict) -> str:
    idle = idle_days(entry)
    if idle is None:
        return '<td class="idle"></td>'
    for index, (bound, _) in enumerate(COLORS["idle"]):
        if idle >= bound:
            return f'<td class="idle idle-band-{index}">{idle}</td>'
    return f'<td class="idle">{idle}</td>'


def badges(entry: dict) -> list[tuple[str, str]]:
    """Return (kind, label) pairs in a fixed order."""
    found: list[tuple[str, str]] = []
    for kind, key, prefix in (
        ("pending", "pending_kind", "pending:"),
        ("failed", "failure_code", "failed:"),
        ("cleanup-failed", "cleanup_failure_step", "cleanup-failed:"),
        ("issue-create", "issue_create_status", "issue-create:"),
    ):
        value = text_field(entry, key)
        if value:
            found.append((kind, prefix + value))
    for kind, key, label in (
        ("claimable", "claimable", "claimable"),
        ("cleanup", "cleanup_candidate", "cleanup"),
        ("unreflected", "completion_unreflected", "unreflected"),
        ("invalid", "invalid", "invalid"),
    ):
        if entry.get(key) is True:
            found.append((kind, label))
    return found


def badge_cell(entry: dict) -> str:
    pills = [f'<span class="pill badge-{kind}">{esc(label)}</span>' for kind, label in badges(entry)]
    return f"<td>{''.join(pills)}</td>"


def safe_https(url: str) -> bool:
    """True only for an https URL with a host, no userinfo, and no whitespace or control characters."""
    if not url or URL_FORBIDDEN.search(url):
        return False
    try:
        parsed = urllib.parse.urlparse(url)
    except ValueError:
        return False
    # Userinfo (https://github.com@evil.example/) hides the real host behind a fixed link label.
    return parsed.scheme == "https" and bool(parsed.hostname) and "@" not in parsed.netloc


def link_html(url: str, label: str) -> str:
    if not url:
        return ""
    if safe_https(url):
        return f'<a href="{esc(url)}">{esc(label)}</a>'
    return f'<span class="muted">{esc(url)}</span>'


def links_cell(entry: dict) -> str:
    pieces = [
        link_html(text_field(entry, "issue_url"), "issue"),
        link_html(text_field(entry, "remote_artifact_url"), "artifact"),
    ]
    return f"<td>{' '.join(piece for piece in pieces if piece)}</td>"


def row_html(entry: dict) -> str:
    ident = text_field(entry, "id")
    command = NEXT_COMMAND.format(id=shlex.quote(ident)) if ident else ""
    cells = [
        f'<td class="id">{esc(ident)}</td>',
        repo_cell(text_field(entry, "repo")),
        f"<td>{esc(text_field(entry, 'branch'))}</td>",
        phase_cell(text_field(entry, "phase")),
        lease_cell(entry),
        idle_cell(entry),
        badge_cell(entry),
        links_cell(entry),
        f"<td><code>{esc(command)}</code></td>",
    ]
    return "<tr>" + "".join(cells) + "</tr>"


def count_text(value: object) -> str:
    return str(value) if isinstance(value, int) and not isinstance(value, bool) else "-"


def dicts(value: object) -> list[dict]:
    """Return the dict items of a list, or [] when value is not a list."""
    return [item for item in value if isinstance(item, dict)] if isinstance(value, list) else []


def page(title: str, summary: str, body: str) -> str:
    """Wrap an already-escaped summary line and body in the shared HTML document."""
    return (
        "<!DOCTYPE html>\n"
        '<html lang="ko"><head><meta charset="utf-8">\n'
        '<meta name="viewport" content="width=device-width, initial-scale=1">\n'
        f"<title>{esc(title)}</title>\n"
        f"<style>\n{build_css()}\n</style></head>\n"
        f"<body><h1>{esc(title)}</h1>\n"
        f'<p class="summary">{summary}</p>\n'
        f"{body}\n</body></html>\n"
    )


def render_board(data: dict) -> str:
    """Return the full HTML document for a parsed `issueops list --json` object."""
    entries = dicts(data.get("entries"))
    entries.sort(key=sort_key)
    generated = data.get("generated_at")
    summary = (
        f"표시 {len(entries)} / 전체 {count_text(data.get('scanned_records'))}, "
        f"읽기 실패 {count_text(data.get('read_errors'))}, "
        f"생성 {esc(generated) if isinstance(generated, str) and generated else '-'}"
    )
    if entries:
        header = "".join(f"<th>{name}</th>" for name in ("ID", "repo", "branch", "phase", "lease", "idle", "상태", "링크", "다음 명령"))
        body = f"<table><thead><tr>{header}</tr></thead><tbody>\n" + "\n".join(row_html(e) for e in entries) + "\n</tbody></table>"
    else:
        body = '<p class="muted">사이클 없음</p>'
    return page("issueops 사이클 보드", summary, body)


# What each limit code means for a reader; an unlisted code is shown as the code alone.
LIMIT_TEXT = {
    "phase_ledger_first_entry_last_completion": "phase 기록은 각 phase의 첫 진입과 마지막 완료만 남아 있다.",
    "lease_last_value_only": "lease 기록은 마지막 claim, release, replace만 남아 있다.",
    "single_review_latest_only": "구현·문서·스키마 리뷰는 가장 최근 기록만 남아 있다.",
    "retained_without_timeline": "타임라인이 생기기 전에 보존된 기록이라 이벤트가 없다.",
    "next_reports_first_blocking_gate_only": "`next`는 막는 첫 게이트 하나만 보고한다. not_reported는 신선하다는 뜻이 아니다.",
}


def pill(group: str, name: str, label: str) -> str:
    """A colored label; a name missing from COLORS[group] uses the group's default color."""
    return f'<span class="pill {group}-{name if name in COLORS[group] else "default"}">{esc(label)}</span>'


def table_html(headers: tuple[str, ...], rows: list[list[str]]) -> str:
    """Return a table; each row is a list of already-escaped cell contents."""
    head = "".join(f"<th>{name}</th>" for name in headers)
    body = "\n".join("<tr>" + "".join(f"<td>{cell}</td>" for cell in row) + "</tr>" for row in rows)
    return f"<table><thead><tr>{head}</tr></thead><tbody>\n{body}\n</tbody></table>"


def limits_html(limits: object) -> str:
    codes = [code for code in limits if isinstance(code, str)] if isinstance(limits, list) else []
    items = "".join(f"<li>{esc(LIMIT_TEXT.get(code, ''))} <code>{esc(code)}</code></li>" for code in codes)
    return f'<ul class="muted">{items}</ul>' if items else ""


def keys_text(keys: object) -> str:
    names = [key for key in keys if isinstance(key, str)] if isinstance(keys, list) else []
    return esc(", ".join(names)) if names else "없음"


def render_timeline(data: dict) -> str:
    """Return the HTML page of `issueops view --view timeline --json`."""
    timeline = data["timeline"] if isinstance(data.get("timeline"), dict) else {}
    events = dicts(timeline.get("events"))
    source = text_field(data, "source")
    summary = (
        f"표시 {count_text(timeline.get('events_shown'))} / 전체 {count_text(timeline.get('events_total'))}, "
        f"시각 없음 {count_text(timeline.get('undated'))}, source {esc(source or '-')}"
    )
    if source == "retained":
        summary += f", outcome {esc(text_field(data, 'outcome') or '-')}, retained_at {esc(text_field(data, 'retained_at') or '-')}"
    if events:
        rows = [
            [esc(e.get("at", "")), pill("event", str(e.get("kind", "")), str(e.get("kind", "")))]
            + [esc(e.get(key, "")) for key in ("phase", "subject", "ref", "verdict", "round", "generation")]
            for e in events
        ]
        body = table_html(("at", "kind", "phase", "subject", "ref", "verdict", "round", "generation"), rows)
    else:
        body = '<p class="muted">이벤트 없음</p>'
    return page(f"issueops 타임라인 {text_field(data, 'id')}", summary, limits_html(timeline.get("limits")) + body)


def render_graph(data: dict) -> str:
    """Return the HTML page of `issueops view --view graph --json`."""
    graph = data["graph"] if isinstance(data.get("graph"), dict) else {}
    judged = graph["judged_by"] if isinstance(graph.get("judged_by"), dict) else {}
    nodes, edges = dicts(graph.get("nodes")), dicts(graph.get("edges"))
    summary = (
        f"judged_by stage {esc(text_field(judged, 'stage') or '-')}, stale_keys {keys_text(judged.get('stale_keys'))}, "
        f"unmapped_stale_keys {keys_text(judged.get('unmapped_stale_keys'))}, "
        f"unknown_reason {esc(text_field(graph, 'unknown_reason') or '없음')}"
    )
    node_rows = [
        [esc(n.get("id", "")), esc(n.get("kind", "")), "있음" if n.get("present") is True else "없음", esc(n.get("at", "")), esc(n.get("verdict", "")), link_html(text_field(n, "url"), "link")]
        for n in nodes
    ]
    edge_rows = [
        [f"{esc(e.get('from', ''))} &rarr; {esc(e.get('to', ''))}", esc(e.get("binding", "")), esc(e.get("gate", "")), pill("state", str(e["state"]), str(e["state"])) if e.get("state") else ""]
        for e in edges
    ]
    children = "".join(f"<li>{esc(n.get('id', ''))} {esc(n.get('verdict', ''))} {link_html(text_field(n, 'url'), 'issue')}</li>" for n in nodes if n.get("kind") == "child")
    tree = f'<li>cycle<ul class="tree">{children}</ul></li>'
    if any(n.get("kind") == "parent" and n.get("present") is True for n in nodes):
        tree = f'<li>parent<ul class="tree">{tree}</ul></li>'
    body = (
        limits_html(graph.get("limits"))
        + "<h2>노드</h2>\n" + table_html(("id", "kind", "present", "at", "verdict", "링크"), node_rows)
        + "\n<h2>연결</h2>\n" + table_html(("from &rarr; to", "binding", "gate", "상태"), edge_rows)
        + f'\n<h2>부모·자식</h2>\n<p class="muted">자식 표시 {count_text(graph.get("children_shown"))} / 전체 {count_text(graph.get("children_total"))}</p>\n<ul class="tree">{tree}</ul>'
    )
    return page(f"issueops 그래프 {text_field(data, 'id')}", summary, body)


VIEWS = {"timeline": render_timeline, "graph": render_graph}


def fail(message: str) -> int:
    print(f"render_board: {message}", file=sys.stderr)
    return 1


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description="Render `issueops list --json` (stdin) as a self-contained HTML board.")
    parser.add_argument("--out", required=True, help="path of the HTML file to write")
    args = parser.parse_args(argv)
    try:
        data = json.loads(sys.stdin.buffer.read().decode("utf-8-sig"))
    except (UnicodeDecodeError, ValueError, RecursionError) as exc:
        return fail(f"input is not valid UTF-8 JSON: {exc}")
    if not isinstance(data, dict):
        return fail("top-level JSON value must be an object")
    if data.get("ok") is not True:
        return fail("input has ok != true")
    if "view" in data:
        render = VIEWS.get(data["view"]) if isinstance(data["view"], str) else None
        if render is None:
            return fail(f"unknown view: {data['view']!r}")
    elif data.get("entries") is not None and not isinstance(data["entries"], list):
        return fail("entries must be an array")
    else:
        render = render_board
    document = render(data)
    out = Path(args.out)
    try:
        out.parent.mkdir(parents=True, exist_ok=True)
        with open(out, "w", encoding="utf-8", newline="\n") as handle:
            handle.write(document)
    except OSError as exc:
        return fail(f"cannot write {out}: {exc}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
