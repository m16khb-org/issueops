# Gates: 553

- [x] G1: host x 역할 해석기가 flag>local>global>default 우선순위, 필드 병합, child 상속, docs-only 하향(기본값일 때만), round 3~5 상향과 최댓값 캡을 지키고, 잘못된 host·역할·effort를 거부하며 내장 기본값에 Fable이 없다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','./internal/domain/agentmodel/...'],capture_output=True,text=True);o=p.stdout+p.stderr;m=[n for n in ['TestResolve','TestValidateSetting','TestBuiltinNeverFable'] if '--- PASS: '+n+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+o[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G2: 설정 adapter가 연결 워크트리에서 메인 워크트리 local을 찾고, info/exclude 등록이 멱등이며, 알 수 없는 필드를 파일 경로와 함께 거부한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','./internal/adapter/outbound/agentmodelconfig/...'],capture_output=True,text=True);o=p.stdout+p.stderr;m=[n for n in ['TestLocalPathFromLinkedWorktree','TestEnsureExcludeIdempotent','TestReadRejectsUnknownField'] if '--- PASS: '+n+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+o[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G3: issueops model show·set·unset·resolve CLI와 usage 카탈로그 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','./cmd/issueops/modelcli/...','./internal/adapter/inbound/catalog/cli/...'],capture_output=True,text=True);o=p.stdout+p.stderr;m=[n for n in [] if '--- PASS: '+n+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+o[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G4: prepare·next·owner policy·owner 프롬프트가 해석기 값을 쓰고, 깨진 설정에서 next는 경고와 빈 review를, prepare는 상태 변경 전 에러를 낸다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','./internal/application/...','./internal/adapter/issueops/...','./cmd/issueops/issueopscli/...'],capture_output=True,text=True);o=p.stdout+p.stderr;m=[n for n in [] if '--- PASS: '+n+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+o[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G5: Orca·cmux owner 명령에 역할 에이전트 인자가 한 번만 인용돼 들어가고(8 KiB 미만), resume·reconcile 요청에도 실리며, 설정 해석 실패 시 intent가 not_invoked_proven으로 남거나 pending이 생기지 않는다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','./internal/adapter/orca/...','./internal/adapter/hostprotocol/...','./internal/application/issueopspreparation/...','./internal/application/issueopslease/...','./internal/adapter/outbound/issueopslease/...','./cmd/issueops/issueopsapp/...'],capture_output=True,text=True);o=p.stdout+p.stderr;m=[n for n in ['TestClaudeAgentsJSON','TestCodexRoleFile','TestBuildInteractiveArgvExtraArgs','TestOwnerAgentCommandRoleAgents','TestPrepareOrcaRoleAgentFailureKeepsNotInvoked','TestResumeRoleAgentFailureLeavesNoPendingIntent','TestReconcileRoleAgentUsesProbeHost'] if '--- PASS: '+n+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+o[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G6: io-model 스킬과 갱신한 스킬이 validate-skill과 shell 검사를 통과한다
  CHECK: python3 -c "import subprocess as s;a=s.run(['python3','scripts/validate-skill.py','skills/io-model','skills/issueops-review','skills/issueops-remote-write','skills/issueops'],capture_output=True,text=True);b=s.run(['python3','scripts/verify-skill-shell.py'],capture_output=True,text=True);print('ALL_PASS' if a.returncode==0 and b.returncode==0 else 'FAIL'+chr(10)+(a.stdout+a.stderr+b.stdout+b.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G7: 모든 Go 파일이 gofmt 형식이다
  CHECK: python3 -c "import subprocess as s;f=s.run(['git','ls-files','*.go'],capture_output=True,text=True).stdout.split();p=s.run(['gofmt','-l']+f,capture_output=True,text=True);print('GOFMT_CLEAN' if p.returncode==0 and not p.stdout.strip() else 'DIRTY'+chr(10)+p.stdout+p.stderr)"
  EXPECT: GOFMT_CLEAN
  EVIDENCE: GOFMT_CLEAN
- [x] G8: go vet이 전체 패키지에서 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','vet','./...'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G9: 전체 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','./...','-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G10: race 전체 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-race','./...','-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G11: golangci-lint v2.12.2가 go.mod 툴체인으로 darwin과 linux 모두 0건을 보고하고 bin/issueops가 빌드된다
  CHECK: python3 -c "import subprocess as s,sys,os,tempfile;v=s.run(['go','list','-m','-f','{{.GoVersion}}'],capture_output=True,text=True).stdout.strip();d=tempfile.mkdtemp();e=dict(os.environ,GOTOOLCHAIN='go'+v,GOBIN=d);g=s.run(['go','build','-o','bin/issueops','./cmd/issueops'],capture_output=True,text=True);i=s.run(['go','install','github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2'],capture_output=True,text=True,env=e);c=[d+'/golangci-lint','run','./...'];a=s.run(c,capture_output=True,text=True,env=e);b=s.run(c,capture_output=True,text=True,env=dict(e,GOOS='linux'));print('ALL_PASS' if g.returncode==0 and i.returncode==0 and a.returncode==0 and b.returncode==0 else 'FAIL'+chr(10)+(g.stderr+i.stderr+a.stdout+a.stderr)[-1500:]+(b.stdout+b.stderr)[-1500:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G12: worktree 내용을 독립 저장소로 복사해 임시 HOME에 설치한 뒤 self-verify가 seed 100, 목표 95로 통과한다(CI의 임시 HOME 절차)
  CHECK: python3 -c "import subprocess as s,os,tempfile,json,shutil;g={k:s.run(['go','env',k],capture_output=True,text=True).stdout.strip() for k in ['GOCACHE','GOMODCACHE','GOPATH']};t=tempfile.mkdtemp();r=t+'/repo';h=t+'/home';os.makedirs(h);f=[p for p in s.run(['git','ls-files','-z','-co','--exclude-standard'],capture_output=True,text=True).stdout.split(chr(0)) if p and os.path.isfile(p)];[(os.makedirs(os.path.dirname(r+'/'+p),exist_ok=True),shutil.copy2(p,r+'/'+p)) for p in f];q=lambda *a:s.run(['git','-C',r,'-c','user.name=gate','-c','user.email=gate@example.com',*a],capture_output=True,text=True);q('init','-q');q('add','-A');q('commit','-q','-m','snapshot');e=dict(os.environ,HOME=h,CODEX_HOME=h+'/.codex',**g);b=s.run(['go','build','-o','bin/issueops','./cmd/issueops'],cwd=r,capture_output=True,text=True,env=e);i=s.run(['./scripts/install-native.sh','--skip-build','--path-mode=skip','--mcp-transport=stdio'],cwd=r,capture_output=True,text=True,env=e);p=s.run(['./bin/issueops','self-verify','--seed=100','--target-score=95','--llm-eval=false','--json'],cwd=r,capture_output=True,text=True,env=e);d=json.JSONDecoder().raw_decode(p.stdout)[0] if p.stdout.strip() else {};s.run(['chmod','-R','u+w',t]);shutil.rmtree(t,ignore_errors=True);ok=b.returncode==0 and i.returncode==0 and d.get('ok')==True;print('SELF_VERIFY_OK='+str(ok)+('' if ok else chr(10)+(b.stderr+i.stderr)[-800:]+p.stderr[-800:]))"
  EXPECT: SELF_VERIFY_OK=True
  EVIDENCE: SELF_VERIFY_OK=True
- [x] G13: 제거한 agentmodel 기본값 함수 이름이 코드·golden·testdata·스킬·현행 문서·configs에 남지 않는다(과거 이슈 기록·계획·ADR 이력은 고치지 않는다)
  CHECK: python3 -c "import subprocess as s;p=s.run(['git','grep','-nwE',chr(124).join(['PlannerDefaults','ImplementerDefaults','ResearchDefaults','ReviewEffortForTier','ImplementerModelCodex','ImplementerEffortCodex','ImplementerModelClaude','ImplementerEffortClaude','ImplementerModelOmo','ImplementerEffortOmo']),'--',':!.issueops/issues/',':!.issueops/plans/',':!.issueops/adr/',':!internal/architecture/testdata/ddd_responsibility_inventory.json'],capture_output=True,text=True);print('NO_STALE_NAMES' if not p.stdout.strip() else 'FOUND'+chr(10)+p.stdout[-3000:])"
  EXPECT: NO_STALE_NAMES
  EVIDENCE: NO_STALE_NAMES
- [x] G14: 연결 워크트리에서 빌드한 issueops가 model set --scope local로 메인 워크트리 local을 쓰고, resolve 소스가 local이며, 두 체크아웃의 git status가 비어 있다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','./cmd/issueops/modelcli/...'],capture_output=True,text=True);o=p.stdout+p.stderr;m=[n for n in ['TestLocalScopeFromLinkedWorktreeBinary'] if '--- PASS: '+n+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+o[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G15: claude와 codex 실측 결과(서브에이전트 전체 model ID, Codex 자식 스레드 gpt-6-luna/low)와 canonical worktree의 local 해석 관측이 기록돼 있다
  CHECK: python3 -c "t=open('.issueops/issues/553/live-check.md').read();print('LIVE_RECORDED' if all(k in t for k in ['판정: claude pass','판정: codex pass','판정: local pass']) else 'MISSING')"
  EXPECT: LIVE_RECORDED
  EVIDENCE: LIVE_RECORDED
- [x] G16: project docs 검사가 위반 0이다
  CHECK: python3 -c "import subprocess as s,json,os;p=s.run(['uv','run','--directory','skills/project-docs-optimize','python','-m','scripts.check','--root',os.getcwd(),'--mode','check','--json'],capture_output=True,text=True);d=json.loads(p.stdout);print('DOCS_OK='+str(d.get('ok')))"
  EXPECT: DOCS_OK=True
  EVIDENCE: DOCS_OK=True
