#!/usr/bin/env python3
from __future__ import annotations

import json
import re
import subprocess
import sys
import tempfile
import unittest
from html.parser import HTMLParser
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
if str(SCRIPT_DIR) not in sys.path:
    sys.path.insert(0, str(SCRIPT_DIR))

import render_board  # noqa: E402

SCRIPT = SCRIPT_DIR / "render_board.py"
STRING_FIELDS = (
    "id", "repo", "branch", "phase", "lease_status", "holder_host", "pending_kind", "failure_code",
    "cleanup_failure_step", "issue_create_status", "issue_url", "remote_artifact_url",
)


def doc(*entries: dict, **extra) -> dict:
    data = {"ok": True, "generated_at": "2026-11-11T12:16:04Z", "scanned_records": len(entries), "read_errors": 0, "entries": list(entries)}
    data.update(extra)
    return data


def entry(**fields) -> dict:
    base = {"id": "io-aaa", "repo": "C:\\work\\acme-repo", "phase": "pr"}
    base.update(fields)
    return base


def all_fields(value: str) -> dict:
    return {key: value for key in STRING_FIELDS}


class RowParser(HTMLParser):
    """Collect the text of each body-table cell, row by row."""

    def __init__(self) -> None:
        super().__init__()
        self.rows: list[list[str]] = []
        self._cell: list[str] | None = None
        self._in_body = False

    def handle_starttag(self, tag, attrs):
        if tag == "tbody":
            self._in_body = True
        elif tag == "tr" and self._in_body:
            self.rows.append([])
        elif tag == "td" and self._in_body:
            self._cell = []

    def handle_endtag(self, tag):
        if tag == "td" and self._cell is not None:
            self.rows[-1].append("".join(self._cell))
            self._cell = None

    def handle_data(self, data):
        if self._cell is not None:
            self._cell.append(data)


def rows_of(page: str) -> list[list[str]]:
    parser = RowParser()
    parser.feed(page)
    return parser.rows


def run_script(stdin: bytes, out: Path | None) -> subprocess.CompletedProcess:
    command = [sys.executable, str(SCRIPT)]
    if out is not None:
        command += ["--out", str(out)]
    return subprocess.run(command, input=stdin, capture_output=True, check=False)


class RenderFunctionTest(unittest.TestCase):
    def test_a_no_external_references_without_links(self) -> None:
        page = render_board.render_board(doc(entry(branch="main", lease_status="active", holder_host="h1", pending_kind="push")))
        for needle in ("http://", "https://", "<script", "<link", "src=", "@import", "url("):
            self.assertEqual(page.count(needle), 0, needle)

    def test_b_html_in_every_string_field_is_escaped(self) -> None:
        page = render_board.render_board(doc(all_fields("<img onerror=x>")))
        self.assertEqual(page.count("<img"), 0)

    def test_c_quote_cannot_break_out_of_an_attribute(self) -> None:
        page = render_board.render_board(doc(all_fields('x" onmouseover="a')))
        self.assertEqual(page.count('" onmouseover="'), 0)
        self.assertIn("&quot;", page)

    def test_d_unsafe_urls_never_become_links(self) -> None:
        for url in ("javascript:alert(1)", "JAVASCRIPT:alert(1)", " javascript:alert(1)", "data:text/html,x", "http://example.com/x", "https://", "//example.com/x", "https://github.com@evil.example/x", "https://a:b@evil.example/", "https://@/x", "https://:443"):
            for key in ("issue_url", "remote_artifact_url"):
                with self.subTest(url=url, key=key):
                    page = render_board.render_board(doc(entry(**{key: url})))
                    self.assertEqual(page.count("<a "), 0)

    def test_e_https_url_becomes_one_link(self) -> None:
        url = "https://github.com/acme/repo/issues/21"
        page = render_board.render_board(doc(entry(issue_url=url)))
        self.assertEqual(page.count(f'href="{url}"'), 1)

    def test_e_url_with_quote_stays_inside_href(self) -> None:
        page = render_board.render_board(doc(entry(issue_url='https://github.com/a"b')))
        self.assertEqual(page.count('" onmouseover="'), 0)
        self.assertNotIn('a"b', page)

    def test_f_idle_cell(self) -> None:
        def idle_of(**fields) -> str:
            return rows_of(render_board.render_board(doc(entry(**fields))))[0][5]

        self.assertEqual(idle_of(idle_days=3), "3")
        self.assertEqual(idle_of(idle_days=0), "0")
        self.assertEqual(idle_of(), "")
        for bad in ("3", -1, 2.5, True, None, [1], {"a": 1}):
            with self.subTest(bad=bad):
                self.assertEqual(idle_of(idle_days=bad), "")

    def test_g_sort_by_idle_desc_then_missing_last_then_id(self) -> None:
        page = render_board.render_board(doc(entry(id="io-a", idle_days=5), entry(id="io-b"), entry(id="io-c", idle_days=9)))
        self.assertEqual([row[0] for row in rows_of(page)], ["io-c", "io-a", "io-b"])

    def test_g_ties_and_missing_sort_by_id(self) -> None:
        page = render_board.render_board(doc(entry(id="io-z", idle_days=2), entry(id="io-y"), entry(id="io-b", idle_days=2), entry(id="io-a")))
        self.assertEqual([row[0] for row in rows_of(page)], ["io-b", "io-z", "io-a", "io-y"])

    def test_h_summary_line(self) -> None:
        page = render_board.render_board(doc(entry(id="io-a"), entry(id="io-b"), scanned_records=7, read_errors=1))
        self.assertIn("표시 2 / 전체 7", page)
        self.assertIn("읽기 실패 1", page)
        self.assertIn("생성 2026-11-11T12:16:04Z", page)

    def test_i_empty_entries_shows_empty_notice(self) -> None:
        page = render_board.render_board(doc())
        self.assertIn("사이클 없음", page)
        self.assertNotIn("<table", page)

    def test_l_next_command_per_row(self) -> None:
        page = render_board.render_board(doc(entry(id="io-a"), entry(id="io-b")))
        for ident in ("io-a", "io-b"):
            self.assertEqual(page.count(f"<code>issueops next --id {ident} --json</code>"), 1)

    def test_l_missing_or_non_string_id_gets_no_command(self) -> None:
        for bad in (None, 7, ["io-a"]):
            with self.subTest(id=bad):
                page = render_board.render_board(doc({"id": bad, "repo": "r", "phase": "plan"}))
                self.assertEqual(page.count("issueops next --id"), 0)
                self.assertNotIn("None", page)

    def test_e_uppercase_https_scheme_is_a_link(self) -> None:
        page = render_board.render_board(doc(entry(issue_url="HTTPS://github.com/acme/repo/issues/21")))
        self.assertEqual(page.count("<a "), 1)

    def test_m_hangul_is_kept(self) -> None:
        page = render_board.render_board(doc(entry(branch="기능/보드")))
        self.assertIn("기능/보드", page)
        self.assertIn('<meta charset="utf-8">', page)

    def test_repo_shows_last_path_element_with_full_path_in_title(self) -> None:
        for repo, short in (("C:\\work\\acme-repo", "acme-repo"), ("/home/u/acme-repo/", "acme-repo")):
            with self.subTest(repo=repo):
                page = render_board.render_board(doc(entry(repo=repo)))
                self.assertEqual(rows_of(page)[0][1], short)
                self.assertIn(f'title="{repo}"', page)

    def test_badges_follow_the_mapping(self) -> None:
        page = render_board.render_board(doc(entry(
            pending_kind="push", failure_code="E1", cleanup_failure_step="worktree", issue_create_status="unknown",
            claimable=True, cleanup_candidate=True, completion_unreflected=True, invalid=True,
        )))
        cell = rows_of(page)[0][6]
        for label in ("pending:push", "failed:E1", "cleanup-failed:worktree", "issue-create:unknown", "claimable", "cleanup", "unreflected", "invalid"):
            self.assertIn(label, cell)

    def test_tolerates_malformed_entries_and_fields(self) -> None:
        page = render_board.render_board(doc(entry(branch=5, claimable="yes", lease_status=None), "junk", 3))
        self.assertEqual(len(rows_of(page)), 1)

    def test_output_is_deterministic(self) -> None:
        data = doc(entry(id="io-a", idle_days=1), entry(id="io-b", idle_days=1))
        self.assertEqual(render_board.render_board(data), render_board.render_board(data))


class ScriptTest(unittest.TestCase):
    def setUp(self) -> None:
        tmp = tempfile.TemporaryDirectory()
        self.addCleanup(tmp.cleanup)
        self.out = Path(tmp.name) / "board.html"

    def test_ok_input_writes_utf8_file(self) -> None:
        payload = json.dumps(doc(entry(branch="기능/보드")), ensure_ascii=False).encode("utf-8")
        result = run_script(payload, self.out)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("기능/보드", self.out.read_text(encoding="utf-8"))

    def test_i_empty_entries_exit_zero_and_file_written(self) -> None:
        result = run_script(json.dumps(doc()).encode(), self.out)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertIn("사이클 없음", self.out.read_text(encoding="utf-8"))

    def test_missing_entries_key_counts_as_empty(self) -> None:
        result = run_script(b'{"ok": true, "scanned_records": 0, "read_errors": 0}', self.out)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue(self.out.exists())

    def test_j_bad_input_exits_one_without_file(self) -> None:
        for payload in (b'{"ok": false}', b"{", b"[]", b'{"ok": true, "entries": "x"}', b"\xff\xfe", b"", b'{"ok": true, "entries": ' + b"[" * 100000):
            with self.subTest(payload=payload):
                result = run_script(payload, self.out)
                self.assertEqual(result.returncode, 1)
                self.assertEqual(len(result.stderr.decode("utf-8").strip().splitlines()), 1)
                self.assertFalse(self.out.exists())

    def test_k_missing_out_is_an_argparse_error(self) -> None:
        result = run_script(json.dumps(doc()).encode(), None)
        self.assertEqual(result.returncode, 2)

    def test_hex_colors_live_only_in_the_colors_dict(self) -> None:
        source = SCRIPT.read_text(encoding="utf-8")
        end = source.index("NEXT_COMMAND =")
        self.assertEqual(re.findall(r"#[0-9a-fA-F]{3,8}\b", source[end:]), [])


GOLDEN = Path(__file__).resolve().parents[3] / "cmd/issueops/testdata/response_contracts.golden.json"
UNSAFE_URLS = ("javascript:alert(1)", "JAVASCRIPT:alert(1)", " javascript:alert(1)", "data:text/html,x", "http://example.com/x", "https://", "//example.com/x", "https://github.com@evil.example/x", "https://a:b@evil.example/", "https://@/x", "https://:443")
HREF_VALUE = re.compile(r'href="[^"]*"')


def golden(key: str) -> dict:
    return json.loads(GOLDEN.read_text(encoding="utf-8"))["cli"][key]


def timeline_doc(**timeline) -> dict:
    base = {"events_total": 0, "events_shown": 0, "undated": 0, "limits": [], "events": []}
    base.update(timeline)
    return {"ok": True, "view": "timeline", "id": "io-aaa", "source": "record", "generated_at": "2026-11-11T12:16:04Z", "timeline": base}


def graph_doc(nodes=(), edges=(), **graph) -> dict:
    base = {"phase": "plan", "judged_by": {"stage": "plan", "stale_keys": [], "unmapped_stale_keys": []}, "limits": [], "nodes": list(nodes), "edges": list(edges), "children_total": 0, "children_shown": 0}
    base.update(graph)
    return {"ok": True, "view": "graph", "id": "io-aaa", "generated_at": "2026-11-11T12:16:04Z", "graph": base}


def render_view(data: dict) -> str:
    return render_board.VIEWS[data["view"]](data)


class ViewRenderTest(unittest.TestCase):
    def test_view_timeline_from_golden(self) -> None:
        data = golden("issueops_view_timeline")
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "timeline.html"
            result = run_script(json.dumps(data).encode(), out)
            self.assertEqual(result.returncode, 0, result.stderr)
            page = out.read_text(encoding="utf-8")
        self.assertIn("표시 9 / 전체 9, 시각 없음 0", page)
        self.assertEqual(len(rows_of(page)), len(data["timeline"]["events"]))
        self.assertIn("phase_ledger_first_entry_last_completion", page)

    def test_view_graph_from_golden(self) -> None:
        data = golden("issueops_view_graph")
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "graph.html"
            result = run_script(json.dumps(data).encode(), out)
            self.assertEqual(result.returncode, 0, result.stderr)
            page = out.read_text(encoding="utf-8")
        self.assertEqual(page.count("<table"), 2)
        for needle in ("stage_unjudgeable", "architecture_doc", "next_reports_first_blocking_gate_only", "prepare"):
            self.assertIn(needle, page)
        self.assertEqual([row[0] for row in rows_of(page) if row][:len(data["graph"]["nodes"])], [n["id"] for n in data["graph"]["nodes"]])

    def test_view_no_external_references(self) -> None:
        pages = [render_view(golden("issueops_view_timeline")), render_view(golden("issueops_view_graph"))]
        pages.append(render_board.render_board(doc(entry(issue_url="https://github.com/acme/repo/issues/21"))))
        for page in pages:
            bare = HREF_VALUE.sub('href=""', page)
            for needle in ("http://", "https://", "<script", "<link", "<svg", "src=", "@import", "url("):
                self.assertEqual(bare.count(needle), 0, needle)

    def test_view_escapes_strings(self) -> None:
        for bad in ("<img onerror=x>", 'x" onmouseover="a'):
            timeline = render_view(timeline_doc(events_total=1, events_shown=1, events=[{"at": bad, "kind": bad, "phase": bad, "subject": bad, "ref": bad, "verdict": bad}]))
            node = {"id": bad, "kind": bad, "present": True, "at": bad, "verdict": bad, "url": bad}
            graph = render_view(graph_doc([node, {"id": f"child:{bad}", "kind": "child", "present": True, "verdict": bad, "url": bad}], [{"from": bad, "to": bad, "binding": bad, "gate": bad, "state": bad}], limits=[bad]))
            for page in (timeline, graph):
                self.assertEqual(page.count("<img"), 0)
                self.assertEqual(page.count('" onmouseover="'), 0)

    def test_view_unsafe_urls_never_become_links(self) -> None:
        for url in UNSAFE_URLS:
            with self.subTest(url=url):
                nodes = [{"id": "plan", "kind": "plan", "present": True, "url": url}, {"id": "child:c1", "kind": "child", "present": True, "url": url}]
                self.assertEqual(render_view(graph_doc(nodes)).count("<a "), 0)

    def test_view_every_href_is_https(self) -> None:
        url = "https://github.com/acme/repo/issues/21"
        nodes = [{"id": "plan", "kind": "plan", "present": True, "url": url}, {"id": "child:c1", "kind": "child", "present": True, "url": url}]
        pages = [render_view(golden("issueops_view_graph")), render_view(graph_doc(nodes))]
        for page in pages:
            for href in re.findall(r'href="([^"]*)"', page):
                self.assertTrue(href.startswith("https://"), href)
        self.assertEqual(pages[1].count("<a "), 3)  # plan and child in the node table, child in the tree

    def test_view_edge_states_have_colors(self) -> None:
        for state in ("stale", "not_reported", "unknown"):
            self.assertIn(state, render_board.COLORS["state"])
            page = render_view(graph_doc(edges=[{"from": "a", "to": "b", "binding": "digest", "gate": "g", "state": state}]))
            self.assertIn(f'class="pill state-{state}"', page)

    def test_view_empty_timeline(self) -> None:
        page = render_view(timeline_doc())
        self.assertIn("이벤트 없음", page)
        self.assertNotIn("<table", page)

    def test_view_retained_without_timeline(self) -> None:
        data = timeline_doc(limits=["retained_without_timeline"])
        data.update(source="retained", outcome="merged", retained_at="2026-11-01T00:00:00Z")
        page = render_view(data)
        for needle in ("타임라인이 생기기 전에 보존된 기록", "retained_without_timeline", "merged", "2026-11-01T00:00:00Z"):
            self.assertIn(needle, page)

    def test_view_counts_pair(self) -> None:
        page = render_view(timeline_doc(events_total=205, events_shown=200, undated=3))
        self.assertIn("표시 200 / 전체 205, 시각 없음 3", page)

    def test_view_children_counts(self) -> None:
        page = render_view(graph_doc(children_total=55, children_shown=50))
        self.assertIn("자식 표시 50 / 전체 55", page)

    def test_view_unknown_view_exits_one(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "out.html"
            for payload in (b'{"ok": true, "view": "bogus"}', b'{"ok": true, "view": ["graph"]}', b'{"ok": true, "view": null}'):
                with self.subTest(payload=payload):
                    result = run_script(payload, out)
                    self.assertEqual(result.returncode, 1)
                    self.assertFalse(out.exists())

    def test_view_bad_input_exits_one(self) -> None:
        bad_ok = dict(timeline_doc(), ok=False)
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "out.html"
            for payload in (b"{", json.dumps(bad_ok).encode(), json.dumps(dict(graph_doc(), ok=False)).encode()):
                with self.subTest(payload=payload):
                    result = run_script(payload, out)
                    self.assertEqual(result.returncode, 1)
                    self.assertFalse(out.exists())


if __name__ == "__main__":
    unittest.main()
