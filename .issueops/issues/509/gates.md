# Gates: 509

- [x] G1: 자기 보고서 생성
  CHECK: test -s .issueops/issues/509/dogfood-report.md
  EVIDENCE: (no output)
- [x] G2: diff 공백 오류 없음
  CHECK: git diff f441a2aa432a51868c855741a8d0ad54f6ff3385 --check
  EVIDENCE: (no output)
- [x] G3: 실제 병렬 claim 관측
  CHECK: python3 -c 'import json; d=json.load(open("/tmp/issueops-dogfood-fixes/cycle/parallel-observed.json")); c=d["cycles"]; assert d["ok"] and d["schema_version"]==1 and len(c)==2; assert all(len({x["status"][k] for x in c})==2 for k in ["id","branch","worktree_path"]); assert len({x["status"]["execution"]["lease"]["holder"]["session_id"] for x in c})==2; assert len({x["status"]["execution"]["lease"]["holder"]["session_process"]["pid"] for x in c})==2; assert all(x["status"]["execution"]["lease"]["status"]=="active" and x["status"]["execution"].get("pending") is None and x["status"]["execution"]["lease"]["generation"]>x["source_generation"] and x["status"]["execution"]["lease"]["holder"]["session_id"]!=x["source_actor"]["session_id"] and x["status"]["worktree_path"]==x["status"]["execution"]["workspace"]["root"] and x["status"]["execution"]["lease"]["holder"]["session_process"]["pid"]!=x["source_actor"]["session_process"]["pid"] and x["process_verified"] and x["observed_at"] and x["stale_owner"]["exit_code"]!=0 and "requires the current write lease holder" in x["stale_owner"]["error"] and len(x["stale_owner"]["before_sha256"])==64 and x["stale_owner"]["before_sha256"]==x["stale_owner"]["after_sha256"] for x in c); print("parallel owners verified")'
  EXPECT: parallel owners verified
  EVIDENCE: parallel owners verified
- [x] G4: 최종 변경 경계
  CHECK: python3 -c 'import subprocess; base="f441a2aa432a51868c855741a8d0ad54f6ff3385"; tracked=set(subprocess.check_output(["git","diff","--name-only",base],text=True).splitlines()); untracked=set(subprocess.check_output(["git","ls-files","--others","--exclude-standard"],text=True).splitlines()); actual=tracked.union(untracked); assert actual=={".issueops/issues/509/dogfood-report.md",".issueops/issues/509/gates.md"},actual; print("scope verified")'
  EXPECT: scope verified
  EVIDENCE: scope verified
