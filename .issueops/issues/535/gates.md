# Gates: 535

- [x] G1: Workflow 실행과 실패 전파
  CHECK: python3 scripts/ci_workflow_test.py
  EXPECT: OK
  EVIDENCE: Ran 2 tests in 2.969s | OK
- [x] G2: Standalone Python와 Go 실패 전파
  CHECK: go test ./internal/application/selfverify -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/application/selfverify	1.142s
- [x] G3: 임시 HOME 전체 self-verify 성공
  CHECK: python3 -c "import json; r=json.load(open(\".issueops/issues/535/artifact/final-selfverify.json\")); assert r[\"ok\"] and r[\"summary\"][\"termination_eligible\"] and not r[\"summary\"][\"coverage_gaps\"]; print(\"FINAL_BATTERY_PASS\")"
  EXPECT: FINAL_BATTERY_PASS
  EVIDENCE: FINAL_BATTERY_PASS
