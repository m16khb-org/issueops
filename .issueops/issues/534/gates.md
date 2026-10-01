# Gates: 534

- [x] G1: 실제 fallback 및 drift 회귀
  CHECK: go test ./internal/application/selfverify ./cmd/issueops/selfworkflow/steps ./cmd/issueops/selfworkflow/verifyloop ./internal/architecture -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/cmd/issueops/selfworkflow/verifyloop	0.188s | ok  	issueops/internal/architecture	23.495s
- [x] G2: 실제 네 golden 테스트 실행
  CHECK: python3 -c 'import subprocess,json; p="issueops/cmd/issueops/"; required=[(p+"contractgolden",n) for n in ["TestCLIUsageGolden","TestMCPToolsGolden","TestMCPResourcesGolden"]]+[(p+"issueopsapp","TestResponseContractsGolden")]; pattern="^("+chr(124).join(n for _,n in required)+")$"; r=subprocess.run(["go","test","-json","./cmd/issueops/contractgolden","./cmd/issueops/issueopsapp","-run",pattern,"-count=1"],capture_output=True,text=True); print(r.stdout); print(r.stderr); assert r.returncode==0; e=[json.loads(l) for l in r.stdout.splitlines() if l.strip()]; seen={(x.get("Package"),x.get("Test"),x.get("Action")) for x in e}; assert all((pkg,n,a) in seen for pkg,n in required for a in ["run","pass"]); assert all((p+pkg,None,"pass") in seen for pkg in ["contractgolden","issueopsapp"]); assert not any(x.get("Action") in ["skip","fail"] for x in e); print("all four golden tests executed and passed")'
  EXPECT: all four golden tests executed and passed
  EVIDENCE: {"Time":"2026-10-01T10:06:58.983416+09:00","Action":"pass","Package":"issueops/cmd/issueops/issueopsapp","Elapsed":8.126} | all four golden tests executed and passed
