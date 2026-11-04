#!/usr/bin/env python3

from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

import route_docs_eval as ev

PLAN = """# 계획 제목

## 목적

`.issueops/cautions/outside.md` 는 이 절 밖의 인용이다.

## 적용되는 결정과 주의사항

| 문서 | 항목 | 제약 |
|---|---|---|
| `.issueops/cautions/a.md` | §1 | x |
| `.issueops/ADR.md` → `adr/b.md` | y | z |
| `cautions/lessons/c.md` | | |
| `.issueops/cautions/a.md` | 중복 | |
| `.issueops/adr/README.md` | 색인 | |
| `.issueops/cautions/missing.md` | 없는 파일 | |
| `.issueops/CAUTIONS.md` | 모듈이 아닌 문서 | |
| `docs/cautions/other.md` | 접두 경로 | |

## 다음 절

`.issueops/adr/after.md`
"""


def tasks(*items: tuple[str, list[str]]) -> list[dict]:
    return [{"issue": i, "title": title, "cited": cited} for i, (title, cited) in enumerate(items, 1)]


class FixtureParsingTest(unittest.TestCase):
    def test_plan_title_takes_first_level_one_heading(self) -> None:
        self.assertEqual(ev.plan_title("intro\n# 계획 제목  \n## 절\n# 둘째"), "계획 제목")
        self.assertIsNone(ev.plan_title("## only second level\n"))

    def test_decision_section_stops_at_next_heading(self) -> None:
        section = ev.decision_section(PLAN)
        self.assertIn("a.md", section)
        self.assertNotIn("after.md", section)
        self.assertNotIn("outside.md", section)
        self.assertIsNone(ev.decision_section("# t\n\n## 목적\n"))

    def test_cited_records_normalizes_dedupes_and_filters(self) -> None:
        existing = {
            ".issueops/cautions/a.md",
            ".issueops/adr/b.md",
            ".issueops/cautions/lessons/c.md",
            ".issueops/adr/README.md",
            "docs/cautions/other.md",
        }
        cited = ev.cited_records(ev.decision_section(PLAN), existing.__contains__)
        self.assertEqual(
            cited,
            [".issueops/adr/b.md", ".issueops/cautions/a.md", ".issueops/cautions/lessons/c.md"],
        )

    def test_build_fixture_reads_plans_in_numeric_order_and_skips_uncited(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp)
            for rel in (".issueops/cautions/a.md", ".issueops/adr/b.md"):
                (repo / rel).parent.mkdir(parents=True, exist_ok=True)
                (repo / rel).write_text("x", encoding="utf-8")
            plans = {
                "10": PLAN,
                "9": "# 아홉\n\n## 적용되는 결정과 주의사항\n\n- `.issueops/adr/b.md`\n",
                "11": "# 인용 없음\n\n## 적용되는 결정과 주의사항\n\n- 없음\n",
                "12": "# 절 없음\n\n## 목적\n",
            }
            for name, text in plans.items():
                path = repo / ".issueops" / "issues" / name / "plan.md"
                path.parent.mkdir(parents=True)
                path.write_text(text, encoding="utf-8")
            fixture = ev.build_fixture(repo)
        self.assertEqual(
            fixture["tasks"],
            [
                {"issue": 9, "title": "아홉", "cited": [".issueops/adr/b.md"]},
                {"issue": 10, "title": "계획 제목", "cited": [".issueops/adr/b.md", ".issueops/cautions/a.md"]},
            ],
        )

    def test_committed_fixture_is_well_formed(self) -> None:
        loaded = ev.load_fixture(ev.DEFAULT_FIXTURE)
        for task in loaded:
            self.assertTrue(all(path.startswith(ev.RECORD_PREFIXES) for path in task["cited"]))

    def test_load_fixture_rejects_malformed_data(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "f.json"
            for bad in ({"tasks": []}, {"tasks": [{"title": "t", "cited": []}]}, {"tasks": [{"cited": ["x"]}]}, []):
                path.write_text(json.dumps(bad), encoding="utf-8")
                with self.assertRaises(ValueError):
                    ev.load_fixture(path)


class ScoringTest(unittest.TestCase):
    def fake(self, routes: dict[str, list[str]]) -> ev.Runner:
        return lambda title: routes[title]

    def test_recall_and_precision_math(self) -> None:
        data = tasks(("t1", ["a", "b"]), ("t2", ["c"]), ("t3", ["d", "e"]))
        runner = self.fake({"t1": ["a", "x"], "t2": ["y"], "t3": []})
        result = ev.score(data, runner)
        self.assertEqual((result["cited"], result["returned"], result["hits"]), (5, 3, 1))
        self.assertAlmostEqual(result["recall_at_k"], 1 / 5)
        self.assertAlmostEqual(result["precision"], 1 / 3)
        self.assertAlmostEqual(result["mean_task_recall"], (0.5 + 0 + 0) / 3)
        self.assertEqual([item["misses"] for item in result["per_task"]], [["b"], ["c"], ["d", "e"]])

    def test_only_top_k_results_count(self) -> None:
        data = tasks(("t", ["late", "early"]))
        routed = ["n1", "n2", "n3", "n4", "early", "late"]
        result = ev.score(data, self.fake({"t": routed}))
        self.assertEqual((result["returned"], result["hits"]), (5, 1))
        self.assertEqual(result["per_task"][0]["misses"], ["late"])
        self.assertAlmostEqual(result["precision"], 1 / 5)

    def test_perfect_and_empty_cases(self) -> None:
        perfect = ev.score(tasks(("t", ["a", "b"])), self.fake({"t": ["b", "a"]}), jobs=4)
        self.assertEqual((perfect["recall_at_k"], perfect["precision"]), (1.0, 1.0))
        self.assertEqual(perfect["per_task"][0]["misses"], [])
        nothing = ev.score(tasks(("t", ["a"])), self.fake({"t": []}))
        self.assertEqual((nothing["recall_at_k"], nothing["precision"]), (0.0, 0.0))

    def test_parallel_run_keeps_fixture_order(self) -> None:
        data = tasks(*[(f"t{i}", [f"r{i}"]) for i in range(20)])
        result = ev.score(data, lambda title: ["r" + title[1:]], jobs=8)
        self.assertEqual([item["title"] for item in result["per_task"]], [f"t{i}" for i in range(20)])
        self.assertEqual(result["recall_at_k"], 1.0)

    def test_record_paths_keeps_only_records_in_router_order(self) -> None:
        docs = [
            {"rel_path": "AGENTS.md"},
            {"rel_path": ".issueops/cautions/x.md"},
            {"rel_path": ".issueops/CAUTIONS.md"},
            {"rel_path": ".issueops/adr/y.md"},
        ]
        self.assertEqual(ev.record_paths(docs), [".issueops/cautions/x.md", ".issueops/adr/y.md"])

    def test_human_report_lists_misses(self) -> None:
        result = ev.score(tasks(("t1", ["a", "b"])), self.fake({"t1": ["a"]}))
        text = ev.render_human(result)
        self.assertIn("recall@5=0.500 precision=1.000", text)
        self.assertIn("missed b", text)
        self.assertNotIn("missed a", text)


if __name__ == "__main__":
    unittest.main()
