# Gates: 517

- [x] G1: 리뷰 스킬 검증
  CHECK: python3 scripts/validate-skill.py skills/issueops-review
  EVIDENCE: Skill is valid!
- [x] G2: 리뷰 스킬 셸 블록 검증
  CHECK: python3 scripts/verify-skill-shell.py skills/issueops-review
  EVIDENCE: skill shell verification passed: 1 file(s)
- [x] G3: 3라운드 규칙에 Fable 5 제외와 claude -p 실행 방법이 있다
  CHECK: python3 -c "import sys; s=open('skills/issueops-review/SKILL.md',encoding='utf-8').read(); i=s.find('같은 대상의 수정·재리뷰는 최대 3라운드다'); block=s[i:i+1200] if i>=0 else ''; need=['Fable 5','claude -p --model','--effort','next.review.effort','codex']; miss=[n for n in need if n not in block]; print('present' if not miss else 'missing: '+','.join(miss)); sys.exit(1 if miss else 0)"
  EXPECT: present
  EVIDENCE: present
- [x] G4: 나쁜 예에 Fable 3라운드 항목이 있다
  CHECK: python3 -c "import sys; s=open('skills/issueops-review/SKILL.md',encoding='utf-8').read(); i=s.find('## 나쁜 예'); ok=i>=0 and 'Fable 5' in s[i:]; print('present' if ok else 'missing'); sys.exit(0 if ok else 1)"
  EXPECT: present
  EVIDENCE: present
- [x] G5: 새 ADR이 옛 Claude 기본값을 대체한다고 밝힌다
  CHECK: python3 -c "import glob,sys; fs=[f for f in glob.glob('.issueops/adr/2026-09-2*.md') if 'Fable 5' in open(f,encoding='utf-8').read() and '2026-07-24' in open(f,encoding='utf-8').read()]; print('adr_present' if fs else 'adr_missing'); sys.exit(0 if fs else 1)"
  EXPECT: adr_present
  EVIDENCE: adr_present
- [x] G6: 2026-07-24 ADR 파일은 그대로다
  CHECK: git diff --exit-code 92bbbddabb9bbee9c7a1e050fc6fe061d7d45201 -- .issueops/adr/decisions/2026-07-24-issueops-planner-implementer-dual-structure.md
  EVIDENCE: (no output)
- [x] G7: README 대체 목록과 ADR 색인이 새 ADR 파일을 가리킨다
  CHECK: python3 -c "import glob,os,sys; fs=[f for f in glob.glob('.issueops/adr/2026-09-2*.md') if 'Fable 5' in open(f,encoding='utf-8').read() and '2026-07-24' in open(f,encoding='utf-8').read()]; idx=open('.issueops/ADR.md',encoding='utf-8').read(); r=open('.issueops/adr/README.md',encoding='utf-8').read(); ok=bool(fs) and any('adr/'+os.path.basename(f) in idx for f in fs) and '2026-07-24' in r and 'Fable 5' in r; print('linked' if ok else 'not_linked'); sys.exit(0 if ok else 1)"
  EXPECT: linked
  EVIDENCE: linked
- [x] G8: 코드의 모델 기본값은 그대로다
  CHECK: git diff --exit-code 92bbbddabb9bbee9c7a1e050fc6fe061d7d45201 -- internal/port/orca.go internal/contract/issueopspreparation/prepare.go
  EVIDENCE: (no output)
- [x] G9: 응답 계약 golden
  CHECK: go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
  EVIDENCE: ok  	issueops/cmd/issueops/issueopsapp	11.061s
- [x] G10: Go 전체 테스트
  CHECK: go test ./... -count=1
  EVIDENCE: <redacted>
- [x] G11: 바이너리 빌드
  CHECK: go build -o bin/issueops ./cmd/issueops
  EVIDENCE: (no output)
- [x] G12: docs·inspect 기본 게이트
  CHECK: python3 -c "import subprocess,sys; rs=[subprocess.run(['./bin/issueops',c,'--json'],capture_output=True).returncode for c in ('docs','inspect')]; print('ok' if rs==[0,0] else 'rc='+str(rs)); sys.exit(0 if rs==[0,0] else 1)"
  EXPECT: ok
  EVIDENCE: ok
- [x] G13: self-verify 기본 게이트
  CHECK: ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
  EVIDENCE: <redacted>
