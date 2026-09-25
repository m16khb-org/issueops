# Gates: 514

- [x] G1: 절단 함수의 경계와 최대성
  CHECK: go test ./internal/domain/policy -count=1
  EVIDENCE: ok  	issueops/internal/domain/policy	0.958s
- [x] G2: 새 회귀 테스트 24개가 모두 있고 통과
  CHECK: python3 -c "import subprocess,sys; names=['TestTruncateBytesUTF8SafeMaximalPrefix','TestTailBytesUTF8SafeMaximalSuffix','TestTruncateRunesUTF8SafeCountsCharacters','TestTrimIncompleteRuneUTF8Safe','TestDryRunPreviewUTF8Safe','TestBoundedCommandStderrUTF8Safe','TestBoundedDiagnosticUTF8Safe','TestBoundedIssueOpsTextUTF8Safe','TestBoundedOutputTextUTF8Safe','TestBoundedProcessFieldUTF8Safe','TestOrcaBoundedDiagnosticUTF8Safe','TestBudgetOutputUTF8Safe','TestBoundedVersionUTF8Safe','TestEllipsizeMiddleUTF8Safe','TestDurableIssueCreateFailureUTF8Safe','TestRemoteVerifyStderrUTF8Safe','TestCommandOutputErrorUTF8Safe','TestBoundedExecutionRemoteDiagnosticUTF8Safe','TestPreparationBoundedDiagnosticUTF8Safe','TestBoundedGitFailureUTF8Safe','TestEvidenceTailUTF8Safe','TestThrowDetailUTF8Safe','TestTailWithBudgetUTF8Safe','TestTruncateContentUTF8SafeCountsCharacters']; pkgs=['./internal/domain/policy','./internal/domain/issueopsremote','./internal/domain/judgement','./internal/domain/webfetch','./internal/domain/gates','./internal/adapter/provider/providerutil','./internal/adapter/audit','./internal/adapter/orca','./internal/adapter/policy','./internal/adapter/hostprobe','./cmd/issueops/issueopscli/remotecmd','./cmd/issueops/issueopscli/remoteverify','./internal/adapter/issueops','./internal/adapter/outbound/issueopspreparation','./internal/adapter/issueops/orphancleanup','./cmd/issueops/apidoc/reviewfiles','./cmd/issueops/commandstep']; r=subprocess.run(['go','test']+pkgs+['-run','UTF8Safe','-count=1','-v'],capture_output=True,text=True); missing=[n for n in names if ('--- PASS: '+n+' (') not in r.stdout]; ok=r.returncode==0 and not missing; print('ALL_PASS' if ok else 'missing: '+','.join(missing)+' rc='+str(r.returncode)); sys.exit(0 if ok else 1)"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G3: 기존 테스트 파일을 고치지 않았다
  CHECK: python3 -c "import subprocess,sys; out=subprocess.run(['git','diff','--name-only','--diff-filter=M','92bbbddabb9bbee9c7a1e050fc6fe061d7d45201'],capture_output=True,text=True).stdout.split(); mod=[p for p in out if p.endswith('_test.go')]; print('unchanged' if not mod else ' '.join(mod)); sys.exit(1 if mod else 0)"
  EXPECT: unchanged
  EVIDENCE: unchanged
- [x] G4: 기존 절단 테스트가 통과
  CHECK: python3 -c "import subprocess,sys; cases=[('./internal/domain/policy','TestBoundedDiagnosticRedactsAndCapsExternalText'),('./internal/domain/issueopsremote','TestBoundedIssueOpsText')]; rs=[subprocess.run(['go','test',p,'-run',n,'-count=1','-v'],capture_output=True,text=True) for p,n in cases]; ok=all(r.returncode==0 and ('--- PASS: '+n+' (') in r.stdout for r,(p,n) in zip(rs,cases)); print('ALL_PASS' if ok else 'FAIL'); sys.exit(0 if ok else 1)"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G5: 대상 18개 파일에 바이트 슬라이스 절단이 남지 않음
  CHECK: python3 -c "import sys; fs=['internal/adapter/provider/providerutil/bounded_command.go','internal/domain/policy/redaction.go','internal/domain/issueopsremote/issueops_text_bound.go','internal/domain/judgement/structured.go','internal/adapter/audit/process.go','internal/adapter/orca/runner.go','internal/adapter/policy/policy_run.go','internal/adapter/hostprobe/runner.go','internal/adapter/hostprobe/omo.go','cmd/issueops/issueopscli/remotecmd/remote.go','cmd/issueops/issueopscli/remoteverify/issueops_remote_helpers.go','internal/adapter/issueops/execution_remote.go','internal/adapter/outbound/issueopspreparation/repository.go','internal/adapter/issueops/orphancleanup/orphan_cleanup.go','internal/domain/gates/evaluate.go','cmd/issueops/apidoc/reviewfiles/evidence.go','cmd/issueops/commandstep/result.go','internal/domain/webfetch/validate.go']; toks=['value[:providerDiagnosticLimit]','value[:limit]','s[:400]','s[:1000]','value[:processDiagnosticLimit]','text[:limit]','value[:prefix]','value[len(value)-suffix:]','value[:256]','diagnostic[:maxBytes]','diagnostic[:maxRemoteVerifyDiagnosticBytes]','message[:4096]','stderr[:512]','joined[:max]','arg[:72]','s[originalBytes-tailBudget:]','content[:maxChars]']; hits=[f+':'+str(i+1) for f in fs for i,l in enumerate(open(f).read().splitlines()) if any(t in l for t in toks)]; print('clean' if not hits else ' '.join(hits)); sys.exit(1 if hits else 0)"
  EXPECT: clean
  EVIDENCE: clean
- [x] G6: #513 소유 파일이 그대로다
  CHECK: git diff --exit-code 92bbbddabb9bbee9c7a1e050fc6fe061d7d45201 -- internal/adapter/issueops/issueops_completion_remote.go
  EVIDENCE: (no output)
- [x] G7: gofmt 깨끗함(커밋 전 파일 포함)
  CHECK: python3 -c "import subprocess,sys; fs=subprocess.run(['git','ls-files','--cached','--others','--exclude-standard','*.go'],capture_output=True,text=True).stdout.split(); out=subprocess.run(['gofmt','-l']+fs,capture_output=True,text=True).stdout.strip() if fs else ''; print(out or 'clean'); sys.exit(1 if out else 0)"
  EXPECT: clean
  EVIDENCE: clean
- [x] G8: go vet
  CHECK: go vet ./...
  EVIDENCE: (no output)
- [x] G9: 변경 패키지 lint
  CHECK: golangci-lint run ./internal/domain/policy ./internal/domain/issueopsremote ./internal/domain/judgement ./internal/domain/webfetch ./internal/domain/gates ./internal/adapter/provider/providerutil ./internal/adapter/audit ./internal/adapter/orca ./internal/adapter/policy ./internal/adapter/hostprobe ./cmd/issueops/issueopscli/remotecmd ./cmd/issueops/issueopscli/remoteverify ./internal/adapter/issueops ./internal/adapter/outbound/issueopspreparation ./internal/adapter/issueops/orphancleanup ./cmd/issueops/apidoc/reviewfiles ./cmd/issueops/commandstep
  EVIDENCE: (no output)
- [x] G10: Go 전체 테스트(architecture 포함)
  CHECK: go test ./... -count=1
  EVIDENCE: <redacted>
- [x] G11: race 테스트
  CHECK: go test -race ./... -count=1
  EVIDENCE: <redacted>
- [x] G12: 바이너리 빌드
  CHECK: go build -o bin/issueops ./cmd/issueops
  EVIDENCE: (no output)
- [x] G13: docs·inspect 기본 게이트
  CHECK: python3 -c "import subprocess,sys; rs=[subprocess.run(['./bin/issueops',c,'--json'],capture_output=True).returncode for c in ('docs','inspect')]; print('ok' if rs==[0,0] else 'rc='+str(rs)); sys.exit(0 if rs==[0,0] else 1)"
  EXPECT: ok
  EVIDENCE: ok
- [x] G14: self-verify 기본 게이트
  CHECK: ./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
  EVIDENCE: <redacted>
