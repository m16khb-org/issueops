# Gates: 560

- [x] G1: 추적되는 Go 파일 어디에도 Orca 언급이 없다
  CHECK: python3 -c "import subprocess; r=subprocess.run(['git','grep','--untracked','-il','orca','--','*.go'],capture_output=True,text=True); print('NO_ORCA_GO' if r.returncode==1 else 'FOUND '+str(len(r.stdout.split())))"
  EXPECT: NO_ORCA_GO
  EVIDENCE: NO_ORCA_GO
- [x] G2: skills 아래 Orca 언급이 없다
  CHECK: python3 -c "import subprocess; r=subprocess.run(['git','grep','--untracked','-il','orca','--','skills'],capture_output=True,text=True); print('NO_ORCA_SKILLS' if r.returncode==1 else 'FOUND '+str(len(r.stdout.split())))"
  EXPECT: NO_ORCA_SKILLS
  EVIDENCE: NO_ORCA_SKILLS
- [x] G3: usage golden에 삭제한 명령과 방식 선택 플래그가 없다
  CHECK: python3 -c "t=open('cmd/issueops/testdata/usage.golden.txt').read(); bad=[n for n in ('execution resume','execution switch-mode','--mode ','--direct-reason','--preserve-terminal') if n in t]; print('USAGE_CLEAN' if not bad else 'FOUND '+','.join(bad))"
  EXPECT: USAGE_CLEAN
  EVIDENCE: USAGE_CLEAN
- [x] G4: 실행 기록 계약과 도메인에 방식 선택 식별자·필드가 남지 않는다
  CHECK: python3 -c "import subprocess; names=['ExecutionMode','ExecutionSelection','json:'+chr(34)+'selection','json:'+chr(34)+'driver','resolved_mode','requested_mode','probe_attempted','readiness_fingerprint','explicit_direct_reason']; a=['git','grep','--untracked','-n','-F']; [a.extend(['-e',n]) for n in names]; r=subprocess.run(a+['--','internal/contract','internal/domain','internal/application','internal/port','internal/adapter','cmd',':(exclude)internal/contract/toolconformance/types.go'],capture_output=True,text=True); print('NO_MODE_FIELDS' if r.returncode==1 else 'FOUND '+str(r.stdout.count(chr(10))))"
  EXPECT: NO_MODE_FIELDS
  EVIDENCE: NO_MODE_FIELDS
- [x] G5: released direct generation이 reseed 뒤 새 세션 claim으로 인수된다
  CHECK: go test ./cmd/issueops/issueopsapp -run TestSuccessfulDirectClaimObservesReleasedReseededGeneration -count=1 -v
  EXPECT: /^--- PASS: TestSuccessfulDirectClaimObservesReleasedReseededGeneration/m
  EVIDENCE: PASS | ok  	issueops/cmd/issueops/issueopsapp	1.887s
- [x] G6: CLI·MCP·response contract golden이 갱신된 표면과 일치한다
  CHECK: go test ./cmd/issueops/contractgolden ./cmd/issueops/issueopsapp -run Golden -count=1
  EXPECT: /^ok\s+issueops\/cmd\/issueops\/contractgolden/m
  EVIDENCE: ok  	issueops/cmd/issueops/contractgolden	0.330s | ok  	issueops/cmd/issueops/issueopsapp	8.312s
- [x] G7: 전체 race 테스트가 통과한다
  CHECK: python3 -c "import subprocess; b=subprocess.run(['go','test','-race','./...','-count=1'],capture_output=True); print('RACE_PASS' if b.returncode==0 else 'RACE_FAIL %d'%b.returncode)"
  EXPECT: RACE_PASS
  EVIDENCE: RACE_PASS
- [x] G7a: go vet이 통과한다
  CHECK: python3 -c "import subprocess; a=subprocess.run(['go','vet','./...'],capture_output=True); print('VET_PASS' if a.returncode==0 else 'VET_FAIL')"
  EXPECT: VET_PASS
  EVIDENCE: VET_PASS
- [x] G8: self-verify가 95점 게이트를 넘는다
  CHECK: python3 -c "import json,subprocess; r=subprocess.run(['./bin/issueops','self-verify','--seed=100','--target-score=95','--llm-eval=false','--json'],capture_output=True,text=True); d=json.loads(r.stdout); print('SELF_VERIFY_OK' if r.returncode==0 and d.get('ok') else 'SELF_VERIFY_FAIL')"
  EXPECT: SELF_VERIFY_OK
  EVIDENCE: SELF_VERIFY_OK
- [x] G9: 지운 명령·플래그·필드 이름이 현행 표면에 남지 않는다
  CHECK: python3 -c "import subprocess; names=['--owner-host','--owner-model','--owner-effort','--direct-reason','--expected-readiness-fingerprint','--issue-body-sha256','--context-packet-sha256','--issue-snapshot-file','--preserve-terminal','--sealed','--mode auto','--mode direct','--mode orca','execution resume','switch-mode',chr(96)+'issue_snapshot'+chr(96),chr(34)+'issue_snapshot'+chr(34),'issue_snapshot_source','issue_snapshot_file','json:'+chr(34)+'issue_snapshot','context_packet_sha256','issue_body_sha256','resolved_mode','readiness_fingerprint','explicit_direct_reason']; ex=[':(exclude).issueops/issues',':(exclude).issueops/plans',':(exclude).issueops/research',':(exclude).issueops/archive',':(exclude).issueops/verified-execution',':(exclude).issueops/adr/20*',':(exclude).issueops/cautions/20*',':(exclude)docs/research',':(exclude)docs/repo-efficiency-audit-2026-10-01.md',':(exclude)internal/domain/commandparse/issueops_test.go']; a=['git','grep','--untracked','-l','-F']; [a.extend(['-e',n]) for n in names]; r=subprocess.run(a+['--','.']+ex,capture_output=True,text=True); print('NO_REMOVED_NAMES' if r.returncode==1 else 'FOUND '+' '.join(r.stdout.split()))"
  EXPECT: NO_REMOVED_NAMES
  EVIDENCE: NO_REMOVED_NAMES
- [x] G10: production 명령 handler로 prepare, release, replace preview, reseed, claim 인계 체인이 끝까지 동작하고, lease가 generation 1 active, 1 released, 2 claimable, 2 active 순서로 바뀌며 claim 뒤 holder가 claim actor와 같다
  CHECK: go test ./cmd/issueops/issueopsapp -run TestDirectHandoffChainThroughProductionCommands -count=1 -v
  EXPECT: /^--- PASS: TestDirectHandoffChainThroughProductionCommands/m
  EVIDENCE: PASS | ok  	issueops/cmd/issueops/issueopsapp	1.278s
- [x] G11: `.issueops` 문서 배치와 링크 계약에 위반이 없다. 단 새 main(6f1ad90d)이 이미 갖고 있는 `.issueops/issues/561/plan.md`의 깨진 링크 한 건은 이 사이클 범위 밖이라 예외로 둔다
  CHECK: python3 -B -c "import sys,io,contextlib; sys.path.insert(0,'skills/project-docs-optimize'); import scripts.check as c; o=io.StringIO(); e=io.StringIO(); f=contextlib.redirect_stdout(o); g=contextlib.redirect_stderr(e); f.__enter__(); g.__enter__(); rc=c.main(['--root','.','--mode','check']); g.__exit__(None,None,None); f.__exit__(None,None,None); v=[l for l in (o.getvalue()+e.getvalue()).splitlines() if l.startswith('- [')]; print('DOCS_OK' if v==['- [broken_link] .issueops/issues/561/plan.md: target does not exist: testing/unit-and-contract.md'] or (rc==0 and not v) else 'DOCS_VIOLATIONS '+' | '.join(v))"
  EXPECT: DOCS_OK
  EVIDENCE: DOCS_OK
- [x] G12: 현행 cautions 모듈에 사라진 Orca 동작 설명이 남지 않는다
  CHECK: python3 -c "import subprocess; r=subprocess.run(['git','grep','-l','-F','-e','orca_runtime_ready','-e','orca_remove','-e','orca_resources_absent','-e','orca terminal','-e','orca orchestration','--','.issueops/cautions/issueops-execution.md','.issueops/cautions/issueops-lifecycle.md','.issueops/cautions/issueops-stages.md','.issueops/cautions/audit-and-process.md'],capture_output=True,text=True); print('NO_ORCA_CAUTION_NAMES' if r.returncode==1 else 'FOUND '+' '.join(r.stdout.split()))"
  EXPECT: NO_ORCA_CAUTION_NAMES
  EVIDENCE: NO_ORCA_CAUTION_NAMES
