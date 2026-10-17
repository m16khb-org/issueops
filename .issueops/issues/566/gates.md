# Gates: 566

- [x] G1: 결함 유형 전체 나열 규칙이 있다
  CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "같은 유형의 다른 발생 위치를 대상 전체에서 찾아 같은 결함 안에 나열한다"
  EXPECT: FOUND
  EVIDENCE: FOUND
- [x] G2: delta 리뷰어가 직전 지적 상태를 판정한다
  CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "직전 지적마다 해결됨 또는 남음만 판정하고"
  EXPECT: FOUND
  EVIDENCE: FOUND
- [x] G3: 기존 결함 표시 규칙이 있다
  CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "기존 결함으로 표시해 비차단 메모에 둔다"
  EXPECT: FOUND
  EVIDENCE: FOUND
- [x] G4: 성공 기준과 관련된 결함은 기존 결함에서 빠진다
  CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "성공 기준이 고치라고 한 결함, 결함 수정 사이클에서 그 결함의 근본 원인"
  EXPECT: FOUND
  EVIDENCE: FOUND
- [x] G5: 근본 원인 렌즈가 intent 기준으로 걸린다
  CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "intent가 결함·회귀 수정이면 diff에 근본 원인 렌즈를 더한다"
  EXPECT: FOUND
  EVIDENCE: FOUND
- [x] G6: 근본 원인 렌즈의 허용 조건이 CONSTITUTION 원문과 같다
  CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-review/SKILL.md "근본 원인이 이미 제거됐거나 현재 범위에서 물리적으로 제거할 수 없다는 근거"
  EXPECT: FOUND
  EVIDENCE: FOUND
- [x] G7: create-pr이 위험이 큰 tier에서 위험과 되돌리기를 요구한다
  CHECK: python3 -c "import sys;t=' '.join(open(sys.argv[1],encoding='utf-8').read().replace(chr(96),'').split());print('FOUND' if sys.argv[2] in t else 'MISSING')" skills/issueops-create-pr/SKILL.md "review.tier가 schema-auth나 contract면 위험과 되돌리기를 반드시 두고"
  EXPECT: FOUND
  EVIDENCE: FOUND
- [x] G8: sync-pr 표에 템플릿에 없는 옛 제목이 없다
  CHECK: awk 'BEGIN{n=0} $0 ~ "## 변경 사항" {n++} $0 ~ "## 검증" {n++} $0 ~ "## 리뷰어 초점" {n++} $0 ~ "## 위험.rollback" {n++} $0 ~ "## Breaking Changes" {n++} $0 ~ "릴리즈 노트" {n++} $0 ~ "## 문서.마이그레이션" {n++} END{print "stale_headings=" n}' skills/issueops-sync-pr/SKILL.md
  EXPECT: stale_headings=0
  EVIDENCE: stale_headings=0
- [x] G9: 바뀐 세 스킬이 유효하다
  CHECK: python3 scripts/validate-skill.py skills/issueops-review skills/issueops-create-pr skills/issueops-sync-pr
  EXPECT: skills/issueops-sync-pr: ok: Skill is valid!
  EVIDENCE: skills/issueops-create-pr: ok: Skill is valid! | skills/issueops-sync-pr: ok: Skill is valid!
- [x] G10: 바뀐 세 스킬의 shell 코드 블록이 유효하다
  CHECK: python3 scripts/verify-skill-shell.py skills/issueops-review skills/issueops-create-pr skills/issueops-sync-pr
  EXPECT: skill shell verification passed: 3 file(s)
  EVIDENCE: skill shell verification passed: 3 file(s)
- [x] G11: 스킬 변경이 response-contract golden을 바꾸지 않는다
  CHECK: go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/cmd/issueops/issueopsapp	3.224s
