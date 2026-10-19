# Gates: 570

- [x] G1: 실행 전 사이클에서 아키텍처가 계획보다 먼저 요구된다
  CHECK: go test ./internal/domain/issueopsnext -run TestClassifyPlanStagesArchitectureFirst -count=1 -v
  EXPECT: --- PASS: TestClassifyPlanStagesArchitectureFirst
  EVIDENCE: PASS | ok  	issueops/internal/domain/issueopsnext	0.081s
- [x] G2: 필수 절이 빠지거나 빈 아키텍처는 리뷰 기록이 거부되고 오류에 제목이 나온다
  CHECK: go test ./internal/application/issueopsreview -run TestRecordArchitectureReviewRejectsMissingSections -count=1 -v
  EXPECT: --- PASS: TestRecordArchitectureReviewRejectsMissingSections
  EVIDENCE: PASS | ok  	issueops/internal/application/issueopsreview	0.086s
- [x] G3: 승인 뒤 아키텍처를 바꾸면 stale로 plan.architecture에 머문다
  CHECK: go test ./internal/domain/issueopsnext -run TestClassifyArchitectureReviewStaleStaysInArchitectureStage -count=1 -v
  EXPECT: --- PASS: TestClassifyArchitectureReviewStaleStaysInArchitectureStage
  EVIDENCE: PASS | ok  	issueops/internal/domain/issueopsnext	0.082s
- [x] G4: 범위 안 옛 이름(내용·경로)이 0건이다
  CHECK: python3 -c "import subprocess,re;P=['plan.design','StagePlanDesign','design_review','DesignReview','proposed_design','proposed-design','proposeddesign','design_open_questions','design_approval','design_verification','issueops_review_design','design review','design-review','cleanDesignValues','runIssueOpsDesign','intentdesign','issueops design',chr(34)+'design'+chr(34)];X=('.issueops/adr/','.issueops/issues/','.issueops/plans/','.issueops/research/','.issueops/verified-execution/','.issueops/archive/','.issueops/agent-improvements-2026-10-02/','docs/superpowers/','.issueops/ADR.md:','.issueops/documentation/owner-design-review.md','internal/domain/projectdoc/route.go:','internal/application/projectbootstrap/plan_test.go:');o=subprocess.run(['git','grep','-n','-i','-F']+sum([['-e',p] for p in P],[]),capture_output=True,text=True).stdout.splitlines()+[f+':' for f in subprocess.run(['git','ls-files'],capture_output=True,text=True).stdout.splitlines() if 'design' in f.lower() and 'database-design' not in f and 'diagram-design' not in f];h=[l for l in o if not l.startswith(X) and not re.match(r'[.]issueops/cautions/20[0-9][0-9]-',l) and not re.match(r'[.]issueops/documentation/quality-[^:]*[.]json:',l) and not re.search(r'design-review [FHBM][0-9]+',l) and not re.search(r'design-review [0-9]+차',l) and 'owner-design-review' not in l and 'Parnas' not in l];print(len(h))"
  EXPECT: 0
  EVIDENCE: 0
- [x] G5: CLI 경로에서 start부터 handoff까지 아키텍처가 계획보다 먼저 진행된다
  CHECK: go test ./cmd/issueops/issueopscli -run TestIssueOpsNextWalksArchitectureBeforePlan -count=1 -v
  EXPECT: --- PASS: TestIssueOpsNextWalksArchitectureBeforePlan
  EVIDENCE: PASS | ok  	issueops/cmd/issueops/issueopscli	0.404s
- [x] G6: gofmt와 vet가 깨끗하다
  CHECK: python3 -c "import subprocess,sys;f=subprocess.run(['git','ls-files','*.go'],capture_output=True,text=True).stdout.split();g=subprocess.run(['gofmt','-l']+f,capture_output=True,text=True).stdout.strip();v=subprocess.run(['go','vet','./...']).returncode;print('fmt-vet-ok' if not g and v==0 else 'fmt-vet-fail');sys.exit(0 if not g and v==0 else 1)"
  EXPECT: fmt-vet-ok
  EVIDENCE: fmt-vet-ok
- [x] G7: 전체 테스트(golden·계약 포함)가 통과한다
  CHECK: python3 -c "import subprocess,sys;r=subprocess.run(['go','test','./...','-count=1'],capture_output=True);print('test-ok' if r.returncode==0 else 'test-fail');sys.exit(r.returncode)"
  EXPECT: test-ok
  EVIDENCE: test-ok
- [x] G8: race 검사가 통과한다
  CHECK: python3 -c "import subprocess,sys;r=subprocess.run(['go','test','-race','./...','-count=1'],capture_output=True);print('race-ok' if r.returncode==0 else 'race-fail');sys.exit(r.returncode)"
  EXPECT: race-ok
  EVIDENCE: race-ok
- [x] G9: 부모 리뷰를 승계한 위임 자식은 아키텍처 문서 없이도 plan.architecture에 갇히지 않는다
  CHECK: go test ./internal/domain/issueopsnext -run TestClassifyDelegatedChildSkipsArchitectureStage -count=1 -v
  EXPECT: --- PASS: TestClassifyDelegatedChildSkipsArchitectureStage
  EVIDENCE: PASS | ok  	issueops/internal/domain/issueopsnext	0.783s
- [x] G10: 위임 자식을 만들면 승계한 아키텍처 리뷰가 부모 승계 표시를 갖는다
  CHECK: go test ./internal/domain/issueops -run TestBuildDelegatedProfileMarksParentArchitectureReview -count=1 -v
  EXPECT: --- PASS: TestBuildDelegatedProfileMarksParentArchitectureReview
  EVIDENCE: PASS | ok  	issueops/internal/domain/issueops	0.277s
- [x] G11: 코드 주석과 날짜 없는 현행 문서에 design review 표기가 0건이다
  CHECK: python3 -c "import subprocess,re;o=subprocess.run(['git','grep','-n','-i','-E','design[-_ ]review','--','*.go','.issueops/ADR.md','.issueops/documentation'],capture_output=True,text=True).stdout.splitlines();h=[l for l in o if re.search(r'design[-_ ]review',l.replace('2026-06-29-issueops-phase-ledger-grill-gate-design-review',''),re.I)];print(len(h))"
  EXPECT: 0
  EVIDENCE: 0
