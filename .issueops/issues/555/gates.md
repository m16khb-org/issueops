# Gates: 555

- [x] G1: compact 카탈로그가 .issueops 경로 줄 목록을 내고 두 호스트 모델 텍스트가 같다(단위 테스트)
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test']+['./internal/adapter/projectdoc/...', './internal/adapter/hostprotocol/...', './internal/application/hookprompt/...']+['-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G2: 실제 바이너리의 codex·claude session-start additionalContext가 같고 .issueops/ADR.md 줄을 포함하며 .issueops 없는 디렉터리는 {}를 낸다
  CHECK: python3 -c "import subprocess as s,json,os,tempfile;d=tempfile.mkdtemp();b=d+'/issueops';s.run(['go','build','-o',b,'./cmd/issueops'],check=True);w=os.getcwd();r=lambda a,i,e=None:s.run([b,'hook']+a,input=json.dumps(i),capture_output=True,text=True,env=dict(os.environ,**(e or {}))).stdout;c=json.loads(r(['session-start','--host','codex'],{'cwd':w,'source':'startup'}))['hookSpecificOutput']['additionalContext'];l=json.loads(r(['session-start','--host','claude'],{'cwd':w,'source':'startup'}))['hookSpecificOutput']['additionalContext'];e=r(['session-start','--host','claude'],{'cwd':d,'source':'startup'}).strip();ok=c==l and any(x.startswith('- .issueops/ADR.md: ') for x in c.splitlines()) and e=='{}';print('SAME_CATALOG' if ok else 'DIFF'+chr(10)+c+chr(10)+'---'+chr(10)+l+chr(10)+e)"
  EXPECT: SAME_CATALOG
  EVIDENCE: SAME_CATALOG
- [x] G3: user view가 레포 이름이 든 한 줄이 된다(단위 테스트)
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test']+['./internal/adapter/hookprompt/...', './cmd/issueops/issueopsapp/...']+['-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G3b: worktree에서도 systemMessage가 '📚 issueops · project docs '로 시작하는 한 줄이고 .git 없는 디렉터리는 basename을 쓴다
  CHECK: python3 -c "import subprocess as s,json,os,tempfile;d=tempfile.mkdtemp();b=d+'/issueops';s.run(['go','build','-o',b,'./cmd/issueops'],check=True);w=os.getcwd();r=lambda a,i,e=None:s.run([b,'hook']+a,input=json.dumps(i),capture_output=True,text=True,env=dict(os.environ,**(e or {}))).stdout;m=json.loads(r(['session-start','--host','claude'],{'cwd':w,'source':'startup'})).get('systemMessage','');t=tempfile.mkdtemp();os.makedirs(t+'/.issueops');open(t+'/.issueops/ADR.md','w').write('# ADR'+chr(10));n=json.loads(r(['session-start','--host','claude'],{'cwd':t,'source':'startup'})).get('systemMessage','');ok=m.startswith('📚 issueops · project docs ') and len(m.splitlines())==1 and n.startswith('📚 '+os.path.basename(t)+' · project docs 1');print('REPO_NAME_LINE' if ok else 'BAD'+chr(10)+m+chr(10)+n)"
  EXPECT: REPO_NAME_LINE
  EVIDENCE: REPO_NAME_LINE
- [x] G4: 표준 설명 개정과 subagent-start 서브커맨드 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test']+['./internal/domain/projectdoc/...', './cmd/issueops/hookcli/...', './internal/adapter/inbound/catalog/cli/...', './cmd/issueops/']+['-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G5: subagent-start가 SessionStart와 같은 카탈로그를 systemMessage 없이 SubagentStart 이벤트로 낸다(claude·codex)
  CHECK: python3 -c "import subprocess as s,json,os,tempfile;d=tempfile.mkdtemp();b=d+'/issueops';s.run(['go','build','-o',b,'./cmd/issueops'],check=True);w=os.getcwd();r=lambda a,i,e=None:s.run([b,'hook']+a,input=json.dumps(i),capture_output=True,text=True,env=dict(os.environ,**(e or {}))).stdout;c=json.loads(r(['session-start','--host','claude'],{'cwd':w,'source':'startup'}))['hookSpecificOutput']['additionalContext'];a=json.loads(r(['subagent-start','--host','claude'],{'cwd':w,'agent_type':'general-purpose'}));x=json.loads(r(['subagent-start','--host','codex'],{'cwd':w,'agent_type':'worker'}));h=a.get('hookSpecificOutput',{});ok=h.get('hookEventName')=='SubagentStart' and 'systemMessage' not in a and h.get('additionalContext')==c and x.get('hookSpecificOutput',{}).get('additionalContext')==c and 'systemMessage' not in x;print('SUBAGENT_CATALOG' if ok else 'BAD'+chr(10)+json.dumps(a)+chr(10)+json.dumps(x))"
  EXPECT: SUBAGENT_CATALOG
  EVIDENCE: SUBAGENT_CATALOG
- [x] G5b: Explore·explorer·FORK 에이전트는 {}를 받고 ISSUEOPS_DISABLE_HOOKS=1이면 출력이 없다
  CHECK: python3 -c "import subprocess as s,json,os,tempfile;d=tempfile.mkdtemp();b=d+'/issueops';s.run(['go','build','-o',b,'./cmd/issueops'],check=True);w=os.getcwd();r=lambda a,i,e=None:s.run([b,'hook']+a,input=json.dumps(i),capture_output=True,text=True,env=dict(os.environ,**(e or {}))).stdout;o=[r(['subagent-start','--host','claude'],{'cwd':w,'agent_type':t}).strip() for t in ['Explore','explorer','FORK']];z=r(['subagent-start','--host','claude'],{'cwd':w,'agent_type':'general-purpose'},{'ISSUEOPS_DISABLE_HOOKS':'1'});print('EXCLUDED' if all(json.loads(v)=={} for v in o) and z.strip()=='' else 'BAD'+chr(10)+repr(o)+chr(10)+repr(z))"
  EXPECT: EXCLUDED
  EVIDENCE: EXCLUDED
- [x] G6: 두 설치기가 SubagentStart를 등록하고 golden·병합 보존·native integration 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test']+['./internal/adapter/claude/...', './internal/adapter/codex/...', './internal/adapter/', './internal/adapter/verification/...']+['-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G9: 추적 템플릿에 SubagentStart 명령이 호스트별로 있다
  CHECK: python3 -c "import json,re;c=json.load(open('configs/codex/hooks.json'));l=json.load(open('configs/claude/hooks.settings.json'));f=lambda d,h:bool(re.search('hook subagent-start --host '+h+'$',d['hooks']['SubagentStart'][0]['hooks'][0]['command']));print('TEMPLATES_OK' if f(c,'codex') and f(l,'claude') else 'BAD')"
  EXPECT: TEMPLATES_OK
  EVIDENCE: TEMPLATES_OK
- [x] G10: 'SessionStart 하나' 같은 낡은 서술이 코드·문서에 남지 않는다
  CHECK: python3 -c "import subprocess as s;p=s.run(['rg','-n',chr(124).join(['session-start.post-compact .', '둘뿐이다', 'turns both into a no-op', 'register only .SessionStart', 'Hooks provide only .SessionStart', 'owns exactly .SessionStart', 'SessionStart. 하나', 'SessionStart.만 등록', 'SessionStart. context hook만', 'catalog 주입뿐', 'only adds SessionStart', 'SessionStart alone carries', 'context-only ..SessionStart. project-doc catalog.', '정확히 .SessionStart. 하나'])]+['internal', 'cmd', 'configs', '.issueops/CONVENTIONS.md', '.issueops/conventions', '.issueops/operations', '.issueops/CAUTIONS.md', '.issueops/cautions/issueops-lifecycle.md', '.issueops/architecture', '.issueops/ARCHITECTURE.md', '.issueops/testing', 'scripts', 'README.md', 'README.en.md', 'skills'],capture_output=True,text=True);print('NO_STALE_CLAIMS' if p.returncode==1 else 'FOUND rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: NO_STALE_CLAIMS
  EVIDENCE: NO_STALE_CLAIMS
- [x] G11: child-host smoke 스크립트가 SubagentStart 계약 두 개를 검사하고 스크립트 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s,re;t=open('scripts/verify-child-host-smoke.sh').read();n=len(re.findall('hook subagent-start --host (codex' + chr(124) + 'claude)'+chr(34),t));p=s.run(['go','test','./internal/adapter/hostprobe/','-run','ChildHostSmoke','-count=1'],capture_output=True,text=True);ok=n==2 and 'subcommand = '+chr(34)+'session-start'+chr(34) not in t and p.returncode==0;print('SMOKE_OK' if ok else 'BAD n=%d rc=%d'%(n,p.returncode)+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: SMOKE_OK
  EVIDENCE: SMOKE_OK
- [x] G7: 전체 Go 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test']+['./...']+['-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G8: project-docs-optimize 검사에서 기존 roadmap 예산 초과 외 위반이 없다
  CHECK: python3 -c "import subprocess as s,json,os;p=s.run(['uv','run','--directory','skills/project-docs-optimize','python','-m','scripts.check','--root',os.getcwd(),'--mode','report','--json'],capture_output=True,text=True);v=[x for x in json.loads(p.stdout).get('violations',[]) if x.get('path')!='.issueops/adr/roadmap.md'];print('DOCS_OK' if not v else 'VIOLATIONS'+chr(10)+json.dumps(v)[-3000:])"
  EXPECT: DOCS_OK
  EVIDENCE: DOCS_OK
