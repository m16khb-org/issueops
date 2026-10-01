# Gates: 543

- [x] G1: 커밋 범위와 dirty union 및 invalid scope 판정
  EVIDENCE: ok  	issueops/internal/application/riskqa	0.659s | ok  	issueops/internal/domain/riskqa	0.658s
  CHECK: go test ./internal/adapter/verification/riskqa ./internal/application/riskqa ./internal/domain/riskqa -count=1
  EXPECT: /^ok\s/m
- [x] G2: CLI MCP 기준 전달과 selfverify 연결
  EVIDENCE: ok  	issueops/internal/application/selfverify	0.472s | ok  	issueops/cmd/issueops/issueopsapp	2.165s
  CHECK: go test ./cmd/issueops/selfworkflow/verifycmd ./cmd/issueops/mcpcli ./internal/application/selfverify ./cmd/issueops/issueopsapp -run Scope -count=1
  EXPECT: /^ok\s/m
- [x] G3: issueops-verify 스킬 형식
  EVIDENCE: Skill is valid!
  CHECK: python3 scripts/validate-skill.py skills/issueops-verify
  EXPECT: /Skill is valid/
- [x] G4: self-verify 스킬 형식
  EVIDENCE: Skill is valid!
  CHECK: python3 scripts/validate-skill.py skills/self-verify
  EXPECT: /Skill is valid/

- [x] G5: 최종 문서와 exact cycle 기준 전달 계약
  EVIDENCE: ok  	issueops/internal/adapter/skillcontract	0.121s
  CHECK: go test ./internal/adapter/skillcontract -run TestFinalVerificationBatteryPinsSingleOwnerContract -count=1
  EXPECT: /^ok\s/m
