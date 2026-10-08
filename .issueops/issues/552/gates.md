# Gates: 552

- [x] G1: 모든 Go 파일이 gofmt 형식이다
  CHECK: python3 -c "import subprocess as s;f=s.run(['git','ls-files','*.go'],capture_output=True,text=True).stdout.split();p=s.run(['gofmt','-l']+f,capture_output=True,text=True);print('GOFMT_CLEAN' if p.returncode==0 and not p.stdout.strip() else 'DIRTY'+chr(10)+p.stdout+p.stderr)"
  EXPECT: GOFMT_CLEAN
  EVIDENCE: GOFMT_CLEAN
- [x] G2: go vet이 전체 패키지에서 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','vet','./...'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G3: golangci-lint v2.12.2가 go.mod 툴체인으로 darwin과 linux 모두 0건을 보고한다
  CHECK: python3 -c "import subprocess as s,os,tempfile;v=s.run(['go','list','-m','-f','{{.GoVersion}}'],capture_output=True,text=True).stdout.strip();d=tempfile.mkdtemp();e=dict(os.environ,GOTOOLCHAIN='go'+v,GOBIN=d);i=s.run(['go','install','github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2'],capture_output=True,text=True,env=e);c=[d+'/golangci-lint','run','./...'];a=s.run(c,capture_output=True,text=True,env=e);b=s.run(c,capture_output=True,text=True,env=dict(e,GOOS='linux'));print('ALL_PASS' if i.returncode==0 and a.returncode==0 and b.returncode==0 else 'FAIL'+chr(10)+(i.stderr+a.stdout+a.stderr)[-1500:]+(b.stdout+b.stderr)[-1500:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G4: supervisor 사전 감지 테스트 8종(조회 실패·HOME 부재·셸 이스케이프 값을 나눔)이 race로 실행되고 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-race','-count=1','-v','./internal/adapter/mcpservice/...'],capture_output=True,text=True);o=p.stdout;n=o.count('--- PASS: TestPrepareChecksTheUserManagerUnitSearchPath/');print('ALL_PASS' if p.returncode==0 and n==8 else 'FAIL rc=%d n=%d'%(p.returncode,n)+chr(10)+(o+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G5: pr-review Python 스위트가 skip 없이 통과하고 S2·S3 테스트가 실행된다
  CHECK: python3 -c "import subprocess as s;p=s.run(['python3','-m','unittest','discover','-v','-s','skills/pr-review/tests'],capture_output=True,text=True);e=p.stderr;w=['test_source_line_names_the_tool_that_ran','test_source_line_lists_codegraph_and_its_fallback','test_git_grep_skips_ignored_files','test_git_grep_finds_definitions_under_lock_named_directories','test_grep_fallback_when_rg_is_not_installed'];m=[x for x in w if x+' ' not in e];print('ALL_PASS' if p.returncode==0 and '(skipped=' not in e and not m else 'FAIL missing=%s'%m+chr(10)+e[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G6: race 전체 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-race','./...','-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G7: self-verify의 모든 단계가 통과한다
  CHECK: python3 -c "import subprocess as s,json;p=s.run(['go','run','./cmd/issueops','self-verify','--json'],capture_output=True,text=True);d=json.loads(p.stdout);print('SELF_VERIFY_OK='+str(d.get('ok')))"
  EXPECT: SELF_VERIFY_OK=True
  EVIDENCE: SELF_VERIFY_OK=True
- [ ] G8: 현재 HEAD의 PR CI verify job이 통과한다
  CHECK: python3 -c "import subprocess as s,json;h=s.run(['git','rev-parse','HEAD'],capture_output=True,text=True).stdout.strip();p=s.run(['gh','run','list','--workflow','ci.yml','--commit',h,'--json','databaseId,status,conclusion,url'],capture_output=True,text=True);r=json.loads(p.stdout or '[]');ok=[x for x in r if x['status']=='completed' and x['conclusion']=='success'];print('CI_GREEN' if ok else 'NOT_GREEN '+json.dumps(r))"
  EXPECT: CI_GREEN
  EVIDENCE: pending
- [ ] G9: CI의 임시 HOME HTTP 설치 회귀 단계가 HOME 불일치 오류를 확인하고 통과한다
  CHECK: python3 -c "import subprocess as s,json;h=s.run(['git','rev-parse','HEAD'],capture_output=True,text=True).stdout.strip();p=s.run(['gh','run','list','--workflow','ci.yml','--commit',h,'--json','databaseId,conclusion'],capture_output=True,text=True);r=[x for x in json.loads(p.stdout or '[]') if x['conclusion']=='success'];j=s.run(['gh','run','view',str(r[0]['databaseId']),'--json','jobs'],capture_output=True,text=True).stdout if r else '{}';st=[x for y in json.loads(j).get('jobs',[]) for x in y['steps'] if 'HOME the user systemd manager' in x['name']];print('HTTP_STEP_GREEN' if st and all(x['conclusion']=='success' for x in st) else 'NOT_GREEN')"
  EXPECT: HTTP_STEP_GREEN
  EVIDENCE: pending
- [x] G10: CI workflow 테스트가 HTTP 회귀 블록의 실패 전파까지 통과한다
  CHECK: python3 -c "import subprocess as s;a=s.run(['python3','-m','unittest','-v','scripts/ci_workflow_test.py'],capture_output=True,text=True);e=a.stderr;print('ALL_PASS' if a.returncode==0 and 'test_http_home_mismatch_block_requires_refusal_before_writes' in e else 'FAIL'+chr(10)+(a.stdout+e)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
ABANDON: G8 원장 게이트로는 채울 수 없다. 첫 실행에서 CI_GREEN으로 충족된 것은 HEAD가 아직 base 커밋 3a538328이라 main의 CI run을 읽은 결과이며 이 변경의 증거가 아니다. CHECK가 보는 HEAD의 CI run은 원장을 커밋해 push해야 생기고, 그 결과를 EVIDENCE로 쓰면 gates.md가 바뀌어 HEAD가 다시 바뀐다. PR CI verify job 통과는 8단계 push 뒤 PR 발행 전에 run URL로 확인하고 PR 본문과 완료 기록에 남긴다. 실패하면 4단계로 돌아가 원인을 고친다.
ABANDON: G9 G8과 같은 이유로 원장 게이트로는 채울 수 없다. 임시 HOME HTTP 설치 회귀 단계의 통과(실제 runner의 user manager 환경에 HOME이 있고 거부 오류가 나는지)는 push 뒤 PR CI run의 해당 step 결과로 확인하고 run URL을 PR 본문과 완료 기록에 남긴다. 실패하면 4단계로 돌아가 원인을 고친다.
