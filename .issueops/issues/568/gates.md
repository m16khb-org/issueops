# Gates: 568

- [x] G1: opencode actor는 `serve --stdio` receipt로만 허용되고, 부모가 살아 있는 공유 서비스·host를 바꾼 플래그·오래 사는 조상 receipt는 core verifier에서 거부된다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','-run','Opencode','./internal/domain/nativehost','./internal/domain/issueops','./internal/application/authority','./internal/adapter/issueops','./cmd/issueops/issueopscli/executioncmd'],capture_output=True,text=True);o=p.stdout+p.stderr;need=['TestOpencodeStandaloneServerRequiresServeAndStdio','TestNormalizeNativeActorBindsOpencodeHostToOpencodeReceipt','TestVerifyNativeRequiresStandaloneOpencodeServer','TestNativeProcessInspectorOpencodeArgumentsMatchStartIdentity','TestNativeSessionIdentityFromEnvSupportsOpencode','TestNativeSessionIdentityFromEnvRejectsOmpAndOpencodeTogether'];m=[n for n in need if '--- PASS: '+n not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m))"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G2: 생성 plugin이 mock opencode 런타임에서 세션당 한 번 catalog를 읽어 매 호출 system에 넣고 실패를 삼키며, opencode argv가 설치된 플래그만 쓴다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','-run','Opencode','./internal/adapter/hostprotocol','./internal/adapter/gitworktree'],capture_output=True,text=True);o=p.stdout+p.stderr;need=['TestGeneratedOpencodeLifecyclePluginExecutesMockContextHook','TestGeneratedOpencodeLifecyclePluginProofRejectsRuntimeMutations','TestBuildInteractiveArgvOpencodeStandalone','TestBuildPrintArgvOpencodeRun','TestWorkspaceRelaunchCommandOpencode'];m=[n for n in need if '--- PASS: '+n not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m))"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G3: opencode 설치 adapter와 활성화·inspect·native 검증 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','./internal/adapter/opencode','./internal/adapter/installutil','./internal/adapter','./internal/adapter/inspect','./internal/adapter/verification/...','./internal/domain/nativeactivation','./internal/application/nativeactivation','./cmd/issueops/issueopsapp','./cmd/issueops/installcli'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL'+chr(10)+(p.stdout+p.stderr)[-2500:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G4: 새로 빌드한 바이너리의 임시 HOME install dry-run이 opencode 설정·plugin·스킬 링크를 계획하고 임시 HOME에 파일·디렉터리·링크를 하나도 만들지 않는다
  CHECK: python3 -c "import subprocess as s,tempfile,os,json;b=s.run(['go','build','-o','bin/issueops','./cmd/issueops'],capture_output=True,text=True);h=tempfile.mkdtemp();st=tempfile.mkdtemp();e=dict(os.environ,HOME=h,ISSUEOPS_STATE_DIR=st);e.pop('XDG_CONFIG_HOME',None);p=s.run(['./bin/issueops','install','--dry-run','--json','--mcp-transport=stdio'],capture_output=True,text=True,env=e);o=p.stdout;k=['opencode_user_mcp_config','opencode_user_lifecycle_plugin','opencode_project_mcp_template','opencode_lifecycle_extension_template','/.config/opencode/skills/'];m=[x for x in k if x not in o];w=os.listdir(h);print('DRY_RUN_OK' if b.returncode==0 and p.returncode==0 and not m and not w else 'FAIL build=%d rc=%d missing=%s written=%s'%(b.returncode,p.returncode,m,w[:5]))"
  EXPECT: DRY_RUN_OK
  EVIDENCE: DRY_RUN_OK
- [x] G5: opencode role model이 설정 파일·builtin을 따르고 docs-only·escalation이 variant를 바꾸지 않는다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','-run','Opencode','./internal/domain/agentmodel','./internal/application/agentmodel','./cmd/issueops/modelcli'],capture_output=True,text=True);o=p.stdout+p.stderr;need=['TestResolveOpencodeKeepsModelDefinedEffort'];m=[n for n in need if '--- PASS: '+n not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m))"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G6: cmux launcher와 인계 스킬이 opencode를 standalone·auto로 연다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','./internal/adapter/cmux','./internal/adapter/issueops','./cmd/issueops/issueopscli/executioncmd'],capture_output=True,text=True);t=open('skills/issueops/references/session-choice.md').read();ok=p.returncode==0 and 'opencode --standalone --auto' in t;print('ALL_PASS' if ok else 'FAIL rc=%d'%p.returncode)"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G7: trace opencode-json이 step_finish 토큰을 합산한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','-run','Opencode','./internal/adapter/trace','./internal/application/trace','./cmd/issueops/basiccli'],capture_output=True,text=True);o=p.stdout+p.stderr;print('ALL_PASS' if p.returncode==0 and '--- PASS: TestOpencodeHostUsage' in o else 'FAIL rc=%d'%p.returncode)"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G8: hostprobe opencode runner가 사용자 세션 ID 없는 격리 env와 warm-up한 private serve로 episode를 구성하고, catalog 주입 없이는 guard가 probe 도구를 막으며, conformance가 opt-in일 때만 opencode를 요청한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','-v','./internal/adapter/hostprobe','./cmd/issueops/contractcli','./cmd/issueops/issueopsapp'],capture_output=True,text=True);o=p.stdout+p.stderr;need=['TestOpencodeRunnerBuildsIsolatedEnvWithoutUserSessionIDs','TestOpencodeRunnerWarmsMCPBeforeRun','TestOpencodeGuardPluginBlocksProbeToolWithoutCatalog','TestOpencodeRunnerProjectsRunJSONToolUse','TestOpencodeRunnerRequiresExplicitModel','TestOpencodeRunnerNeverWritesUserHome'];m=[n for n in need if '--- PASS: '+n not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m))"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G9: build·vet·gofmt·전체 race 테스트가 통과하고 MCP enum이 opencode를 포함한다(golden·inventory·템플릿 일치는 이 테스트들이 확인한다)
  CHECK: python3 -c "import subprocess as s;r=[s.run(c,capture_output=True,text=True) for c in (['go','build','-o','bin/issueops','./cmd/issueops'],['go','vet','./...'],['go','test','-race','-count=1','./...'])];f=s.run(['gofmt','-l','cmd','internal'],capture_output=True,text=True).stdout.strip();t=open('internal/contract/mcp/issueops_catalog.go').read();ok=all(x.returncode==0 for x in r) and not f and '\"opencode\"' in t;print('ALL_PASS' if ok else 'FAIL gofmt=%r rcs=%s'%(f[:200],[x.returncode for x in r]))"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G10: 실제 opencode 2.0.32 standalone 세션이 catalog를 받고 issueops MCP 도구를 보며 whoami가 standalone receipt를 낸다
  CHECK: python3 -c "import subprocess as s,os;e=dict(os.environ,ISSUEOPS_OPENCODE_LIVE='1');p=s.run(['go','test','-count=1','-v','-run','TestOpencodeLiveStandaloneSession','./internal/adapter/opencode'],capture_output=True,text=True,env=e);o=p.stdout+p.stderr;print('OPENCODE_LIVE_PASS' if p.returncode==0 and '--- PASS: TestOpencodeLiveStandaloneSession' in o and '--- SKIP' not in o else 'FAIL rc=%d'%p.returncode+chr(10)+o[-2500:])"
  EXPECT: OPENCODE_LIVE_PASS
  EVIDENCE: OPENCODE_LIVE_PASS
- [ ] G11: CI와 같은 격리 환경(임시 clone, 임시 HOME·XDG·state, stdio 설치)에서 self-verify가 종료 조건을 만족한다
  CHECK: python3 -c "import subprocess as s,tempfile,os,shutil,json;t=tempfile.mkdtemp();r=os.path.join(t,'r');h=os.path.join(t,'h');os.makedirs(h);s.run(['git','clone','-q','.',r],check=True);L=lambda *a:s.run(['git','ls-files','-z',*a],capture_output=True,text=True).stdout.split(chr(0));D=lambda f:os.path.join(r,f);rm=lambda p:os.path.lexists(p) and os.remove(p);[(os.makedirs(os.path.dirname(D(f)),exist_ok=True),rm(D(f)),shutil.copy2(f,D(f),follow_symlinks=False)) for f in L('-co','--exclude-standard') if f and os.path.lexists(f)];[rm(D(f)) for f in L('-d') if f];e=dict(os.environ,HOME=h,CODEX_HOME=h+'/.codex',XDG_CONFIG_HOME=h+'/.config',ISSUEOPS_STATE_DIR=h+'/state');e.pop('ISSUEOPS_ROOT',None);b=s.run(['./scripts/install-native.sh','--path-mode=skip','--mcp-transport=stdio'],cwd=r,env=e,capture_output=True,text=True);v=s.run(['./bin/issueops','self-verify','--seed=100','--target-score=95','--llm-eval=false','--json'],cwd=r,env=e,capture_output=True,text=True);d=json.loads(v.stdout or '{}');print('ISOLATED_SELF_VERIFY_OK' if b.returncode==0 and d.get('ok') and d.get('termination_eligible') else 'FAIL install=%d verify=%d'%(b.returncode,v.returncode))"
  EXPECT: ISOLATED_SELF_VERIFY_OK
  EVIDENCE: pending
ABANDON: G11 CHECK cannot finish inside the gates check policy cap (timeout_exceeds_15m): the isolated self-verify alone takes about 868s, plus install. The same CHECK steps were run directly with LANG, the CI-equivalent Python venv, and the user Go caches: install exit 0, self-verify exit 0, ok=true, termination_eligible=true, minimum_goal_score=100 (.issueops/evidence/task-11-isolated-self-verify.txt; see cautions/2026-10-18-gate-checks-that-run-go-tests-or-self-verify-need-lang-and-t.md).
