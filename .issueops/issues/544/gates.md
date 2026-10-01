# Gates: 544

- [x] G1: 문서 diff의 공백과 충돌 흔적이 없다
  CHECK: python3 -c "import subprocess; r=subprocess.run([\"git\",\"diff\",\"--check\"],capture_output=True,text=True); print(r.stdout+r.stderr if r.returncode else \"DIFF_CHECK_PASS\"); raise SystemExit(r.returncode)"
  EXPECT: DIFF_CHECK_PASS
  EVIDENCE: DIFF_CHECK_PASS
- [x] G2: project-doc 링크와 구조가 유효하다
  CHECK: uv run --directory skills/project-docs-optimize python -m scripts.check --root /Users/habin/workspace/issueops.worktrees/544-doc-verification-ownership --mode check --json
  EXPECT: /"ok": true/
  EVIDENCE: "violations": [] | }
