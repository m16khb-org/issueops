# Gates: 536

- [x] G1: 현재 hook surface
  CHECK: go test -v ./cmd/issueops/hookcli -run ^TestRetiredHookSubcommandsAreRejected$ -count=1
  EXPECT: PASS
  EVIDENCE: PASS | ok  	issueops/cmd/issueops/hookcli	0.638s
- [x] G2: in-process MCP
  CHECK: go test -v ./cmd/issueops/mcpcli -run ^TestRunMCPServesInProcessWithoutADaemon$ -count=1
  EXPECT: PASS
  EVIDENCE: PASS | ok  	issueops/cmd/issueops/mcpcli	0.400s
- [x] G3: 활성 지침과 loop metadata 회귀
  CHECK: go test -v ./internal/adapter/skillcontract ./internal/application/selfverify -count=1
  EXPECT: PASS
  EVIDENCE: PASS | ok  	issueops/internal/application/selfverify	1.209s
- [x] G4: skill 구조
  CHECK: python3 scripts/validate-skill.py skills/verified-execution
  EXPECT: Skill is valid!
  EVIDENCE: Skill is valid!
