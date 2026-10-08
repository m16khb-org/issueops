# Gates: 558

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
- [x] G4: 최종 battery 문서에 toolchain을 고정한 golangci-lint 단계가 있다
  CHECK: python3 -c "t=open('.issueops/testing/self-verification.md').read();print('BATTERY_LINT' if 'golangci-lint run ./...' in t.split('## 최종 검증 battery')[1].split('관련 변경의 추가 확인')[0] and 'GOOS=linux' in t else 'MISSING')"
  EXPECT: BATTERY_LINT
  EVIDENCE: BATTERY_LINT
- [x] G5: 채널 범위 조회·커서 규칙·retention 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;w=['TestWalkExistingAfterStartsAfterResolvedCursor', 'TestDeleteBeforeRemovesOnlyOlderRowsInBucket', 'TestRangeStartPreservesMissingCursor', 'TestMessagesAfterUsesExactCursorAndRestartsOnMissingCursor', 'TestSendPrunesMessagesPastRetention', 'TestSendPrunesAtRetentionCutoffAfterWrite'];p=s.run(['go', 'test', '-count=1', '-v', '-run']+[chr(124).join('^'+x+'$' for x in w)]+['./internal/adapter/outbound/sqlstore', './internal/adapter/channel', './internal/application/channel', './internal/domain/channel'],capture_output=True,text=True);o=p.stdout;m=[x for x in w if '--- PASS: '+x+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+(o+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G6: channel recv 대기가 context 취소에 즉시 반환한다(application, MCP HTTP)
  CHECK: python3 -c "import subprocess as s;w=['TestRecvWaitReturnsWhenContextCancelled', 'TestChannelRecvWaitStopsWhenHTTPRequestIsCancelled'];p=s.run(['go', 'test', '-count=1', '-v', '-run']+[chr(124).join('^'+x+'$' for x in w)]+['./internal/application/channel', './cmd/issueops/mcpcli'],capture_output=True,text=True);o=p.stdout;m=[x for x in w if '--- PASS: '+x+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+(o+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G7: 다른 채널 메시지 500개가 있는 state에서 channel recv 중앙값이 50ms 이하다
  CHECK: python3 -c "import subprocess as s,os,tempfile,time,json,statistics;d=tempfile.mkdtemp();b=d+'/issueops';s.run(['go','build','-o',b,'./cmd/issueops'],check=True);e=dict(os.environ,ISSUEOPS_STATE_DIR=d+'/state');[s.run([b,'channel','send','--channel','noise','--from','x','--message','m%d'%i],env=e,capture_output=True,check=True) for i in range(500)];s.run([b,'channel','send','--channel','target','--from','x','--message','hit'],env=e,capture_output=True,check=True);f=lambda:(time.perf_counter(),s.run([b,'channel','recv','--channel','target','--json'],env=e,capture_output=True,text=True));x=[(lambda a:((time.perf_counter()-a[0])*1000,len(json.loads(a[1].stdout)['messages'])))(f()) for _ in range(5)];m=statistics.median([y[0] for y in x]);print(('RECV_OK' if m<=50 and all(y[1]==1 for y in x) else 'RECV_SLOW')+' median_ms=%.1f runs=%s'%(m,[round(y[0],1) for y in x]))"
  EXPECT: RECV_OK
  EVIDENCE: RECV_OK median_ms=10.9 runs=[11.3, 11.3, 10.9, 10.7, 10.2]
- [x] G8: loop gate와 health 수집이 bucket을 한 번에 scan한다
  CHECK: python3 -c "import subprocess as s;w=['TestReadAllExistingDecodesEveryLoopInOneScan', 'TestCollectIssueOpsScansRecordsAndReportsUnreadableRows'];p=s.run(['go', 'test', '-count=1', '-v', '-run']+[chr(124).join('^'+x+'$' for x in w)]+['./internal/adapter/looprun', './internal/application/looprun', './internal/adapter/operationalhealth'],capture_output=True,text=True);o=p.stdout;m=[x for x in w if '--- PASS: '+x+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+(o+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G9: MCP HTTP server.log가 상한을 넘으면 copytruncate된다
  CHECK: python3 -c "import subprocess as s;w=['TestCappedLogWriterCopyTruncatesPastLimit', 'TestServiceLogWriterWrapsOnlyTheServiceLog'];p=s.run(['go', 'test', '-count=1', '-v', '-run']+[chr(124).join('^'+x+'$' for x in w)]+['./internal/adapter/mcpservice', './cmd/issueops/mcpcli'],capture_output=True,text=True);o=p.stdout;m=[x for x in w if '--- PASS: '+x+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+(o+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G10: state doctor가 은퇴 경로를 retired_path로 보고하고 install/update가 commit 뒤 지운다
  CHECK: python3 -c "import subprocess as s;w=['TestDoctorClassifiesCurrentAndRetiredStateEntries', 'TestRunTransactionRemovesRetiredStateOnlyAfterFinalize', 'TestRemoveRetiredStateDeletesOnlyAllowlistedEntries', 'TestRemoveRetiredStateKeepsDaemonWhileLegacyProcessIsAlive'];p=s.run(['go', 'test', '-count=1', '-v', '-run']+[chr(124).join('^'+x+'$' for x in w)]+['./internal/domain/state', './internal/application/install', './internal/adapter/install'],capture_output=True,text=True);o=p.stdout;m=[x for x in w if '--- PASS: '+x+' ' not in o];print('ALL_PASS' if p.returncode==0 and not m else 'FAIL rc=%d missing=%s'%(p.returncode,m)+chr(10)+(o+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G11: omo/omp가 extensionhost 하나를 공유하고 adapter 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s,os;p=s.run(['go','test','-count=1','./internal/adapter/extensionhost','./internal/adapter','./internal/adapter/codex','./internal/adapter/claude','./internal/adapter/installutil'],capture_output=True,text=True);g=[x for x in ('internal/adapter/omo','internal/adapter/omp') if os.path.isdir(x)];print('ALL_PASS' if p.returncode==0 and not g else 'FAIL rc=%d left=%s'%(p.returncode,g)+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G12: DDD 책임 원장이 소스와 일치한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-count=1','./internal/architecture','-run','TestDDDResponsibilityInventoryMatchesSource','-v'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 and '--- PASS: TestDDDResponsibilityInventoryMatchesSource' in p.stdout else 'FAIL'+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G13: 병렬화한 issueops 테스트가 race·shuffle 3회에서 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-race','-shuffle=on','-count=3','./internal/adapter/issueops'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G14: internal/adapter/issueops 단독 테스트 wall time이 44.5초 이하다
  CHECK: python3 -c "import subprocess as s,time;t=time.perf_counter();p=s.run(['go','test','-count=1','./internal/adapter/issueops'],capture_output=True,text=True);w=time.perf_counter()-t;print(('WALL_OK' if p.returncode==0 and w<=44.5 else 'WALL_FAIL rc=%d'%p.returncode)+' wall_s=%.1f'%w)"
  EXPECT: WALL_OK
  EVIDENCE: WALL_OK wall_s=38.0
- [x] G15: race 전체 테스트가 통과한다
  CHECK: python3 -c "import subprocess as s;p=s.run(['go','test','-race','./...','-count=1'],capture_output=True,text=True);print('ALL_PASS' if p.returncode==0 else 'FAIL rc=%d'%p.returncode+chr(10)+(p.stdout+p.stderr)[-3000:])"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G16: self-verify의 모든 단계가 통과한다
  CHECK: python3 -c "import subprocess as s,json;b=s.run(['go','build','-o','bin/issueops','./cmd/issueops'],capture_output=True,text=True);p=s.run(['./bin/issueops','self-verify','--seed=100','--target-score=95','--llm-eval=false','--json'],capture_output=True,text=True);d=json.loads(p.stdout);print('SELF_VERIFY_OK='+str(b.returncode==0 and d.get('ok')))"
  EXPECT: SELF_VERIFY_OK=True
  EVIDENCE: SELF_VERIFY_OK=True
- [ ] G17: 현재 HEAD의 PR CI가 통과한다
  CHECK: python3 -c "import subprocess as s,json;h=s.run(['git','rev-parse','HEAD'],capture_output=True,text=True).stdout.strip();p=s.run(['gh','run','list','--workflow','ci.yml','--commit',h,'--json','databaseId,status,conclusion,url'],capture_output=True,text=True);r=json.loads(p.stdout or '[]');ok=[x for x in r if x['status']=='completed' and x['conclusion']=='success'];print('CI_GREEN' if ok else 'NOT_GREEN '+json.dumps(r))"
  EXPECT: CI_GREEN
  EVIDENCE: pending
ABANDON: G17 원장 게이트로는 채울 수 없다. CHECK가 보는 HEAD는 아직 base 커밋 9274cbe6이고, 그 main CI run은 이 이슈가 고치는 errcheck로 실패한 상태다(run 37777728644). 이 변경의 CI run은 커밋을 push해야 생기고, 그 결과를 EVIDENCE로 적으면 gates.md가 바뀌어 HEAD가 다시 바뀐다. PR CI 통과는 8단계 push 뒤 PR 발행 전에 run URL로 확인해 PR 본문과 완료 기록에 남긴다. 실패하면 4단계로 돌아가 원인을 고친다.
