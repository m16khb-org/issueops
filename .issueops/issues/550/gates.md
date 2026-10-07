# Gates: 550

- [x] G1: 모든 Go 파일이 gofmt 형식이다
  CHECK: python3 -c "import subprocess as s,sys;f=s.run(['git','ls-files','*.go'],capture_output=True,text=True).stdout.split();p=s.run(['gofmt','-l']+f,capture_output=True,text=True);print('GOFMT_CLEAN' if p.returncode==0 and not p.stdout.strip() else 'DIRTY'+chr(10)+p.stdout+p.stderr)"
  EXPECT: GOFMT_CLEAN
  EVIDENCE: GOFMT_CLEAN
- [x] G2: go vet이 전체 패키지에서 통과한다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['go','vet','./...'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G3: golangci-lint v2.12.2가 go.mod 툴체인으로 darwin과 linux 모두 0건을 보고한다
  CHECK: python3 -c "import subprocess as s,sys,os,tempfile;v=s.run(['go','list','-m','-f','{{.GoVersion}}'],capture_output=True,text=True).stdout.strip();d=tempfile.mkdtemp();e=dict(os.environ,GOTOOLCHAIN='go'+v,GOBIN=d);i=s.run(['go','install','github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2'],capture_output=True,text=True,env=e);c=[d+'/golangci-lint','run','./...'];a=s.run(c,capture_output=True,text=True,env=e);b=s.run(c,capture_output=True,text=True,env=dict(e,GOOS='linux'));print('ALL_PASS' if i.returncode==0 and a.returncode==0 and b.returncode==0 else 'FAIL'+chr(10)+(i.stderr+a.stdout+a.stderr)[-1500:]+(b.stdout+b.stderr)[-1500:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G4: race 전체 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['go','test','-race','./...','-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G5: CI workflow 테스트가 통과하고 install 호출이 stdio transport를 명시한다
  CHECK: python3 -c "import subprocess as s,sys;a=s.run(['python3','scripts/ci_workflow_test.py'],capture_output=True,text=True);t=open('.github/workflows/ci.yml').read();print('ALL_PASS' if a.returncode==0 and '--mcp-transport=stdio' in t else 'FAIL'+chr(10)+(a.stdout+a.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G6: shellcheck 지적이 0건이다
  CHECK: python3 -c "import subprocess as s,sys,glob;b=s.run(['shellcheck','install.sh']+sorted(glob.glob('scripts/*.sh')),capture_output=True,text=True);print('ALL_PASS' if b.returncode==0 else 'FAIL'+chr(10)+(b.stdout+b.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G7: production Go 코드에 os.IsNotExist 호출이 남지 않는다
  CHECK: python3 -c "import subprocess as s;p=s.run(['git','grep','-nF','os.IsNotExist(','--','*.go',':!*_test.go'],capture_output=True,text=True);print('NO_OS_ISNOTEXIST' if not p.stdout.strip() else 'FOUND'+chr(10)+p.stdout[-3000:])"
  EXPECT: NO_OS_ISNOTEXIST
  EVIDENCE: NO_OS_ISNOTEXIST
- [x] G8: 직접 테스트가 없던 세 패키지에 테스트가 있고 통과한다
  CHECK: python3 -c "import subprocess as s,sys,glob;ps=['internal/domain/issueopsorphancleanup','internal/application/issueopsbodysync','internal/domain/nativehost'];m=[x for x in ps if not glob.glob(x+'/*_test.go')];p=s.run(['go','test','-count=1']+['./'+x for x in ps],capture_output=True,text=True);print('ALL_PASS' if not m and p.returncode==0 else 'FAIL missing=%s'%m+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G9: benchmark 예시 문구 변경 전후 fixture별 차원 점수가 같다(기준 18 fixture, 342 차원, digest 7405bbec3a6764a7)
  CHECK: python3 -c "import subprocess as s,json,hashlib;p=s.run(['go','run','./cmd/issueops','benchmark','run','--fixtures','testdata/issueops/fixtures','--judge','none','--json'],capture_output=True,text=True);d=json.loads(p.stdout);t=sorted((x['fixture_id'],y['dimension'],y['score']) for x in d['scores'] for y in x['dimension_scores']);print('BENCH=%d/%s/%s/%s'%(len(t),hashlib.sha256(json.dumps(t).encode()).hexdigest()[:16],d['average_score'],d['minimum_score']))"
  EXPECT: BENCH=342/7405bbec3a6764a7/100/100
  EVIDENCE: BENCH=342/7405bbec3a6764a7/100/100
- [x] G10: self-verify의 모든 단계가 통과한다
  CHECK: python3 -c "import subprocess as s,sys,json;p=s.run(['go','run','./cmd/issueops','self-verify','--json'],capture_output=True,text=True);d=json.loads(p.stdout);print('SELF_VERIFY_OK='+str(d.get('ok')))"
  EXPECT: SELF_VERIFY_OK=True
  EVIDENCE: SELF_VERIFY_OK=True
- [ ] G11: 현재 HEAD의 PR CI verify job이 install과 self-verify까지 통과한다
  CHECK: python3 -c "import subprocess as s,json;h=s.run(['git','rev-parse','HEAD'],capture_output=True,text=True).stdout.strip();p=s.run(['gh','run','list','--workflow','ci.yml','--commit',h,'--json','databaseId,status,conclusion,url'],capture_output=True,text=True);r=json.loads(p.stdout or '[]');ok=[x for x in r if x['status']=='completed' and x['conclusion']=='success'];print('CI_GREEN' if ok else 'NOT_GREEN '+json.dumps(r))"
  EXPECT: CI_GREEN
  EVIDENCE: pending
- [x] G12: supervisor load 실패 에러가 실패한 명령과 출력을 담는다
  CHECK: python3 -c "import subprocess as s,sys;p=s.run(['go','test','-count=1','-run','Load','./internal/adapter/mcpservice'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G13: PR CI에서 확인한 supervisor 실패 원인 판정이 기록돼 있다
  CHECK: python3 -c "t=open('.issueops/issues/550/ci-diagnosis.md').read();print('DIAGNOSIS_RECORDED' if ('판정: (a)' in t or '판정: (b)' in t) and 'actions/runs/' in t else 'MISSING')"
  EXPECT: DIAGNOSIS_RECORDED
  EVIDENCE: DIAGNOSIS_RECORDED
- [x] G14: revise·regress 상한 5의 경계 테스트가 실제로 실행되고 통과한다
  CHECK: python3 -c "import subprocess as s;r=chr(124).join(['Cap','Revise','Regress']);p=s.run(['go','test','-count=1','-v','-run',r,'./internal/domain/issueopsreview','./internal/adapter/issueops/devilsadvocate','./internal/adapter/issueops'],capture_output=True,text=True);o=p.stdout;a=open('internal/domain/issueopsreview/devils_advocate.go').read();b=open('internal/domain/issueopsreview/regress.go').read();print('ALL_PASS' if p.returncode==0 and 'no tests to run' not in o and 'TestRecordRejectsSixthUnwaivedReviseRound' in o and 'reviseRoundCap = 5' in a and 'regressCap = 5' in b else 'FAIL'+chr(10)+(o+p.stderr)[-2000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G15: 리뷰 스킬이 최대 5라운드와 여섯 번째 거부를 말하고 현행 스킬과 README에 상한 3 문구가 남지 않는다
  CHECK: python3 -c "import glob;k=open('skills/issueops-review/SKILL.md').read();bad=['최대 3라운드','세 번까지','네 번째는','at most three unwaived','The fourth is refused'];fs=sorted(glob.glob('skills/*/SKILL.md'))+['README.md','README.en.md'];left=[f+':'+b for f in fs for b in bad if b in open(f).read()];print('ALL_PASS' if '최대 5라운드' in k and '여섯 번째' in k and not left else 'FAIL '+str(left))"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G16: 상한 숫자를 부분 대체하는 결정 기록이 있고 ADR 색인의 두 기존 행이 superseded in part를 표시한다
  CHECK: python3 -c "import os;p=chr(124)+' ';t=open('.issueops/ADR.md').read().splitlines();a=[l for l in t if l.startswith(p+'2026-07-02') and 'adr/2026-07-02-issueops-regress-round-cap.md' in l];b=[l for l in t if l.startswith(p+'2026-09-08') and 'adr/2026-09-08-adversarial-review-throughput-executable-findings-change-tie.md' in l];n=[l for l in t if 'adr/2026-10-07-review-revise-and-regress-rounds-are-capped-at-five.md' in l and l.startswith(p+'2026-10-07')];f=os.path.exists('.issueops/adr/2026-10-07-review-revise-and-regress-rounds-are-capped-at-five.md');print('MARKED' if f and len(n)==1 and len(a)==1 and len(b)==1 and 'superseded in part' in a[0] and 'superseded in part' in b[0] else 'UNMARKED')"
  EXPECT: MARKED
  EVIDENCE: MARKED
- [x] G17: rg가 없는 PATH에서도 pr-review DefsFallbackTest가 통과한다
  CHECK: python3 -c "import subprocess as s,shutil,os,tempfile,sys;d=tempfile.mkdtemp();[os.symlink(shutil.which(b),os.path.join(d,b)) for b in ('grep','git')];os.symlink(sys.executable,os.path.join(d,'python3'));p=s.run(['python3','-m','unittest','tests.test_context_pack.DefsFallbackTest'],cwd='skills/pr-review',capture_output=True,text=True,env=dict(os.environ,PATH=d));print('ALL_PASS' if p.returncode==0 and 'Ran 3 tests' in p.stderr else 'FAIL'+chr(10)+p.stderr[-2000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
ABANDON: G11 원장 게이트로는 채울 수 없다. CHECK가 보는 'HEAD의 CI run'은 원장을 커밋해 push해야 생기고, 그 결과를 EVIDENCE로 쓰면 gates.md가 바뀌어 HEAD가 다시 바뀐다. CI verify job의 install·self-verify 통과는 8단계 push 뒤 PR 발행 전에 run URL로 확인하고, PR 본문과 완료 기록에 남긴다. 실패하면 4단계로 돌아가 원인을 고친다.
