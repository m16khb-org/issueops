# Gates: 548

- [x] G1: 모든 Go 파일이 gofmt 형식이다
  CHECK: python3 -c "import subprocess as s,sys;f=s.run(['git','ls-files','*.go'],capture_output=True,text=True).stdout.split();p=s.run(['gofmt','-l']+f,capture_output=True,text=True);print('GOFMT_CLEAN' if p.returncode==0 and not p.stdout.strip() else 'DIRTY'+chr(10)+''+p.stdout+p.stderr)"
  EXPECT: GOFMT_CLEAN
  EVIDENCE: GOFMT_CLEAN
- [x] G2: go vet이 전체 패키지에서 통과한다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['go','vet','./...'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'+chr(10)+''%p.returncode+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G3: golangci-lint v2.12.2가 go.mod 툴체인으로 0건을 보고한다
  CHECK: python3 -c "import subprocess as s,sys;import os;v=s.run(['go','list','-m','-f','{{.GoVersion}}'],capture_output=True,text=True).stdout.strip();e=dict(os.environ,GOTOOLCHAIN='go'+v);p=s.run(['go','run','github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2','run','./...'],capture_output=True,text=True,env=e);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'+chr(10)+''%p.returncode+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G4: race 전체 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['go','test','-race','./...','-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'+chr(10)+''%p.returncode+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G5: govulncheck가 호출되는 취약점 0건을 보고한다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['go','run','golang.org/x/vuln/cmd/govulncheck@latest','./...'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'+chr(10)+''%p.returncode+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G6: go.mod가 tidy 상태이고 모듈 검증이 통과한다
  CHECK: python3 -c "import subprocess as s,sys;a=s.run(['go','mod','tidy','-diff'],capture_output=True,text=True);b=s.run(['go','mod','verify'],capture_output=True,text=True);print('ALL_PASS' if a.returncode==0 and not a.stdout.strip() and b.returncode==0 else 'FAIL'+chr(10)+''+a.stdout+a.stderr+b.stdout+b.stderr)"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G7: architecture ratchet 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['go','test','./internal/architecture','-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'+chr(10)+''%p.returncode+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G8: MCP 동시성 테스트가 race로 20회 연속 통과한다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['go','test','-race','-run','TestConcurrentMCPStreamsShareImmutableConfiguration','-count=20','./cmd/issueops/issueopsapp'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'+chr(10)+''%p.returncode+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G9: quality inspect의 collection_status가 error가 아니다
  CHECK: python3 -c "import subprocess as s,sys;import json;p=s.run(['go','run','./cmd/issueops','quality','inspect','--repo','.','--json'],capture_output=True,text=True);d=json.loads(p.stdout);print('COLLECTION_STATUS='+str(d.get('collection_status')))"
  EXPECT: COLLECTION_STATUS=ok
  EVIDENCE: COLLECTION_STATUS=ok
- [x] G10: CI workflow 테스트가 통과하고 shellcheck 지적이 0건이다
  CHECK: python3 -c "import subprocess as s,sys;import glob;a=s.run(['python3','scripts/ci_workflow_test.py'],capture_output=True,text=True);b=s.run(['shellcheck','install.sh']+sorted(glob.glob('scripts/*.sh')),capture_output=True,text=True);print('ALL_PASS' if a.returncode==0 and b.returncode==0 else 'FAIL'+chr(10)+''+(a.stdout+a.stderr)[-1500:]+(b.stdout+b.stderr)[-1500:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G11: --help가 B1의 하위 명령과 플래그를 모두 보여 준다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['go','run','./cmd/issueops','--help'],capture_output=True,text=True);o=p.stdout+p.stderr;need=['state maintain','contract conformance','project append','project commit-suggest','project lint-diagnose','--base-ref','--collect-all-steps','benchmark reliability','benchmark consensus'];m=[n for n in need if n not in o];print('HELP_COMPLETE' if not m else 'MISSING '+','.join(m))"
  EXPECT: HELP_COMPLETE
  EVIDENCE: HELP_COMPLETE
- [x] G12: 현행 문서(날짜 기록 제외)에 없는 심볼과 internal/core 경로가 현재형으로 남지 않는다. 없음·제거됨·역사 기록으로 명시한 줄은 의도적 언급으로 본다
  CHECK: python3 -c "import subprocess as s;p=s.run(['git','grep','-nF','-e','IssueOpsLifecycleTools','-e','TestProductionGraphHasNoLegacyAdapterEdges','-e','internal/core/','--','*.md',':!.issueops/cautions/20*',':!.issueops/adr/20*',':!.issueops/research/**',':!.issueops/plans/**',':!.issueops/archive/**',':!.issueops/issues/**',':!.issueops/verified-execution/**',':!.issueops/prompt-engineering/**',':!docs/superpowers/**'],capture_output=True,text=True);marks=['does not exist','존재하지 않','predate','제거됨','removed','historical','역사 기록','당시 경로','경로 이동'];left=[l for l in p.stdout.splitlines() if not any(m in l for m in marks)];print('NO_STALE_REFS' if not left else 'STALE'+chr(10)+''+chr(10).join(left)[-3000:])"
  EXPECT: NO_STALE_REFS
  EVIDENCE: NO_STALE_REFS
- [x] G13: self-verify의 모든 단계가 통과한다
  CHECK: python3 -c "import subprocess as s,sys;import json;p=s.run(['go','run','./cmd/issueops','self-verify','--json'],capture_output=True,text=True);d=json.loads(p.stdout);print('SELF_VERIFY_OK='+str(d.get('ok')))"
  EXPECT: SELF_VERIFY_OK=True
  EVIDENCE: SELF_VERIFY_OK=True
- [x] G14: core 회귀 가드와 guard 조건 파일이 base와 같다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['git','diff','--quiet','93649a264fd6f972dbb42989113914afc9444f0b','--','internal/architecture/dependency_test.go','internal/architecture/ownership_manifest_test.go','internal/domain/guard/paths.go'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'+chr(10)+''%p.returncode+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G15: benchmark 하위 명령이 IssueOps 카탈로그 원본에 있다
  CHECK: grep -n "benchmark reliability" internal/contract/cli/issueops_catalog.go
  EXPECT: /benchmark reliability/
  EVIDENCE: 97:  issueops benchmark reliability [--outcomes PATH] [--alpha N] [--json]
- [x] G16: artifact_verification이 context.Background를 쓰지 않는다
  CHECK: python3 -c "import subprocess as s,sys;t=open('internal/application/issueopsremote/artifact_verification.go').read();print('NO_BACKGROUND_CTX' if 'context.Background()' not in t else 'FOUND')"
  EXPECT: NO_BACKGROUND_CTX
  EVIDENCE: NO_BACKGROUND_CTX
