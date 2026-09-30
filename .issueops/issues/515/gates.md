# Gates: 515

- [x] G1: 고정 표본
  CHECK: python3 docs/research/2026-09-30-readability/verify.py snapshot
  EXPECT: snapshot verified: 10
  EVIDENCE: snapshot verified: 10
- [x] G2: 지표 재계산
  CHECK: python3 docs/research/2026-09-30-readability/verify.py metrics
  EXPECT: metrics reproduced: 10
  EVIDENCE: metrics reproduced: 10
- [x] G3: reader 증거
  CHECK: python3 docs/research/2026-09-30-readability/verify.py evidence
  EXPECT: reader evidence verified: 10
  EVIDENCE: reader evidence verified: 10
- [x] G4: ADR 연결
  CHECK: python3 docs/research/2026-09-30-readability/verify.py decision
  EXPECT: decision linked
  EVIDENCE: decision linked
- [x] G5: 원격 비교
  CHECK: python3 docs/research/2026-09-30-readability/verify.py remote
  EXPECT: remote comparison recorded: 10
  EVIDENCE: remote comparison recorded: 10
- [x] G6: 단일 문서 배터리
  CHECK: python3 docs/research/2026-09-30-readability/battery.py
  EXPECT: document battery passed
  EVIDENCE: logs: /tmp/issueops-515-battery-78lwbwsi | document battery passed
