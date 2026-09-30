# Gates: 522

- [x] G1: 실제 owner 렌더링과 봉인 보존
  CHECK: go test ./internal/adapter/issueops -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/issueops	285.226s
- [x] G2: 정책 및 runtime tier
  CHECK: go test ./internal/domain/agentmodel ./internal/application/issueopsnext ./internal/application/issueopsowner -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/application/issueopsnext	0.254s | ok  	issueops/internal/application/issueopsowner	0.375s
- [x] G3: review skill 계약
  CHECK: python3.12 scripts/validate-skill.py skills/issueops-review
  EXPECT: Skill is valid!
  EVIDENCE: Skill is valid!
- [x] G4: 최종 단일 self-verify
  CHECK: python3.12 -c 'import subprocess,json,pathlib,sys; p=subprocess.run(["./bin/issueops","self-verify","--seed=100","--target-score=95","--llm-eval=false","--json"],capture_output=True,text=True); pathlib.Path(".issueops/issues/522/artifact/self-verify.json").write_text(p.stdout); pathlib.Path(".issueops/issues/522/artifact/self-verify.stderr").write_text(p.stderr); r=json.loads(p.stdout); assert p.returncode==0 and r["ok"] and r["termination_eligible"]; assert r["summary"]["failed_steps"]==0 and r["summary"]["goal_scores"]; assert all(g["passed"] and g["score"]>95 for g in r["summary"]["goal_scores"]); assert all(s["ok"] for run in r["runs"] for s in run["steps"]); print("SELF_VERIFY_PASS")'
  EXPECT: SELF_VERIFY_PASS
  EVIDENCE: SELF_VERIFY_PASS
