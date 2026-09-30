# Gates: 528

- [x] G1: Python 순서·점수·coverage·후보·역사 계약을 지킨다
  CHECK: go test ./internal/application/selfverify ./internal/domain/selfverify ./internal/application/selfaugment ./cmd/issueops/selfworkflow/... -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/cmd/issueops/selfworkflow/verifycmd	0.315s | ok  	issueops/cmd/issueops/selfworkflow/verifyloop	0.377s
- [x] G2: 최종 selfverify에서 CI와 같은 전체 Python discovery가 성공한다
  CHECK: python3 -c 'import json; r=json.load(open(".issueops/issues/528/artifact/session/selfverify.stdout")); s=next(s for s in r["runs"][0]["steps"] if s["label"]=="Python script tests"); assert s["ok"] and "Ran 53 tests" in s["stderr"] and "OK" in s["stderr"] and "unittest discover -s scripts -p *_test.py" in s["command"]; print("PYTHON_PASS")'
  EXPECT: PYTHON_PASS
  EVIDENCE: PYTHON_PASS
- [x] G3: 실제 Python fail/pass와 missing/unsupported 조기 실패를 확인한다
  CHECK: go test ./internal/application/selfverify -run TestPythonDiscoveryRuntimeAndEarlyFailure -v -count=1
  EXPECT: PASS
  EVIDENCE: PASS | ok  	issueops/internal/application/selfverify	0.971s
- [x] G4: 같은 실행의 format·lint·selfverify·vet·race battery가 모두 성공한다
  CHECK: python3 -c 'import json; r=json.load(open(".issueops/issues/528/artifact/session/battery.json")); assert r["ok"] and r["steps"]==27 and r["minimum_score"]>95 and all(x["returncode"]==0 for x in r["commands"]); print("BATTERY_PASS")'
  EXPECT: BATTERY_PASS
  EVIDENCE: BATTERY_PASS
