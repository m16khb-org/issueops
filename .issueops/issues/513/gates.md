# Gates: 513

- [ ] G1: 스펙 문서가 브랜치에 있다
  CHECK: test -f docs/superpowers/specs/2026-09-24-readable-issue-pr-design.md
  EVIDENCE: pending
- [ ] G2: Go 전체 테스트
  CHECK: go test ./... -count=1
  EVIDENCE: pending
- [ ] G3: race 테스트
  CHECK: go test -race ./... -count=1
  EVIDENCE: pending
- [ ] G4: go vet
  CHECK: go vet ./...
  EVIDENCE: pending
- [ ] G5: gofmt 깨끗함(커밋 전 파일 포함)
  CHECK: python3 -c "import subprocess,sys; fs=subprocess.run(['git','ls-files','--cached','--others','--exclude-standard','*.go'],capture_output=True,text=True).stdout.split(); out=subprocess.run(['gofmt','-l']+fs,capture_output=True,text=True).stdout.strip() if fs else ''; print(out or 'clean'); sys.exit(1 if out else 0)"
  EXPECT: clean
  EVIDENCE: pending
- [ ] G6: 계약 golden
  CHECK: go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
  EVIDENCE: pending
- [ ] G7: PR 계약 필수 절 4개
  CHECK: go test ./internal/domain/artifacttemplate -run TestPullRequestContractRequiresFourSections -count=1
  EVIDENCE: pending
- [ ] G8: 구현 이슈 계약 필수 절 5개
  CHECK: go test ./internal/domain/artifacttemplate -run TestImplementationIssueContractRequiresFiveSections -count=1
  EVIDENCE: pending
- [ ] G9: 가독성 검사 규칙
  CHECK: go test ./internal/domain/artifactreadability -count=1
  EVIDENCE: pending
- [ ] G10: create-issue confirm이 critical과 템플릿 누락을 intent 기록 전에 거부
  CHECK: go test ./cmd/issueops/issueopscli/remotecmd -run TestRemoteCreateIssueRefusesCriticalReadability -count=1
  EVIDENCE: pending
- [ ] G11: create-child·create-pr confirm이 critical을 거부하고 기본 템플릿을 쓴다
  CHECK: go test ./cmd/issueops/issueopscli/remotecmd -run TestRemoteCreateChildAndPRRefuseCriticalReadability -count=1
  EVIDENCE: pending
- [ ] G12: sync-issue·sync-pr confirm이 critical을 거부하고 원격 본문 warning을 보고
  CHECK: go test ./internal/adapter/issueops ./cmd/issueops/issueopscli/remotecmd -run TestRemoteSyncRefusesCriticalAndReportsLiveReadability -count=1
  EVIDENCE: pending
- [ ] G13: reflect-completion이 원고를 요구하고 SHA·로컬 경로·과다 길이를 거부
  CHECK: go test ./cmd/issueops/issueopscli/remotecmd ./internal/adapter/issueops -run TestReflectCompletionRequiresReadableResult -count=1
  EVIDENCE: pending
- [ ] G14: 진행 결과 구간에 하네스 값이 없음
  CHECK: go test ./internal/adapter/provider/issuebody -run TestCompletionSectionIsHumanReadable -count=1
  EVIDENCE: pending
- [ ] G15: 정리 경로가 이슈 본문을 쓰지 않음
  CHECK: go test ./internal/adapter/issueops -run TestCleanupPathsKeepAuditOutOfIssueBody -count=1
  EVIDENCE: pending
- [ ] G16: 계획 검토 한 줄 요약, 중단 지적 가림, 짧은 중단 판정의 반영·regress
  CHECK: python3 -c "import subprocess,sys; pat='TestPlanReviewSectionSummarizesRounds'+chr(124)+'TestPlanReviewMasksHashesAndPaths'+chr(124)+'TestShortKoreanStopReflectsAndRegresses'; sys.exit(subprocess.run(['go','test','./internal/adapter/provider/issuebody','./internal/adapter/issueops','-run',pat,'-count=1']).returncode)"
  EVIDENCE: pending
- [ ] G17: implement·ai-slop-clean 전이가 두 모드에서 추적 사본을 쓴다
  CHECK: go test ./internal/adapter/issueops -run TestPhaseTransitionWritesTrackedMaterials -count=1
  EVIDENCE: pending
- [ ] G18: #513 추적 사본이 있고 무시되지 않는다
  CHECK: python3 -c "import os,subprocess,sys; ps=['.issueops/issues/513/plan.md','.issueops/issues/513/intent.md','.issueops/issues/513/plan-review.md']; bad=[p for p in ps if not os.path.exists(p) or subprocess.run(['git','check-ignore','-q',p]).returncode==0]; print('tracked' if not bad else 'bad: '+','.join(bad)); sys.exit(1 if bad else 0)"
  EXPECT: tracked
  EVIDENCE: pending
- [ ] G19: 저장소 템플릿이 계약과 일치
  CHECK: go test ./internal/domain/artifacttemplate -run TestRepositoryTemplatesMatchContract -count=1
  EVIDENCE: pending
- [ ] G20: 바뀐 스킬 검증
  CHECK: python3 -c "import subprocess,sys; s=['issueops-remote-write','issueops-create-issue','issueops-create-pr','issueops-sync-issue','issueops-sync-pr','issueops-cleanup','issueops-implement','issueops-review','issueops-plan','issueops','gitlab-usecase','gates-ledger']; bad=[x for x in s if subprocess.run([sys.executable,'scripts/validate-skill.py','skills/'+x],capture_output=True).returncode or subprocess.run([sys.executable,'scripts/verify-skill-shell.py','skills/'+x],capture_output=True).returncode]; print('skills ok' if not bad else 'failed: '+','.join(bad)); sys.exit(1 if bad else 0)"
  EXPECT: skills ok
  EVIDENCE: pending
- [ ] G21: Python 한국어 게이트 제거
  CHECK: python3 -c "import os,sys; p='skills/issueops-remote-write/scripts/remote_artifact_gate.py'; gone=not os.path.exists(p); print('removed' if gone else 'present'); sys.exit(0 if gone else 1)"
  EXPECT: removed
  EVIDENCE: pending
- [ ] G22: 프로젝트 문서 검사기
  CHECK: python3 -c "import os,subprocess,sys; sys.exit(subprocess.run(['uv','run','--directory','skills/project-docs-optimize','python','-m','scripts.check','--root',os.getcwd(),'--mode','check','--json'],capture_output=True).returncode)"
  EVIDENCE: pending
- [ ] G23: 바이너리 빌드
  CHECK: go build -o bin/issueops ./cmd/issueops
  EVIDENCE: pending
- [ ] G24: self-verify 기본 게이트
  CHECK: ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
  EVIDENCE: pending
- [ ] G25: 진행 결과 반영은 머지 증거를 계속 요구
  CHECK: rg -o "cannot reflect completion without provider-verified merge evidence" internal/adapter/issueops/issueops_completion_remote.go
  EXPECT: cannot reflect completion without provider-verified merge evidence
  EVIDENCE: pending
- [ ] G26: regress는 중단 판정 반영을 계속 요구
  CHECK: rg -o "reflect the devil's-advocate findings to the issue before regressing" internal/adapter/issueops/issueops_regress.go
  EXPECT: reflect the devil's-advocate findings to the issue before regressing
  EVIDENCE: pending
- [ ] G27: 봉인 디렉터리는 계속 무시된다
  CHECK: git check-ignore -q .issueops/issues/513/artifact/plan.md
  EVIDENCE: pending
- [ ] G28: 사본이 없는 기존 사이클에 warning
  CHECK: go test ./internal/adapter/issueops -run TestTrackedMaterialsMissingWarning -count=1
  EVIDENCE: pending
