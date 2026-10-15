# Gates: 563

- [x] G1: CI와 같은 Linux·go.mod toolchain·golangci-lint v2.12.2 lint가 지적 없이 끝난다
  CHECK: python3 -c "import os,subprocess,sys,tempfile; d=tempfile.mkdtemp(); v=subprocess.run(['go','list','-m','-f','{{.GoVersion}}'],capture_output=True,text=True,check=True).stdout.strip(); subprocess.run(['go','install','github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2'],env=dict(os.environ,GOBIN=d),check=True); r=subprocess.run([d+'/golangci-lint','run','./...'],env=dict(os.environ,GOOS='linux',GOTOOLCHAIN='go'+v),capture_output=True,text=True); out=r.stdout.strip(); print(out.splitlines()[-1] if out else r.stderr.strip()[-300:]); sys.exit(r.returncode)"
  EXPECT: 0 issues.
  EVIDENCE: 0 issues.
- [x] G2: gofmt·go vet·go build·race 전체 테스트가 모두 통과한다
  CHECK: python3 -c "import subprocess,sys; fmt=subprocess.run(['gofmt','-l','cmd','internal'],capture_output=True,text=True).stdout.strip(); cmds=[['go','vet','./...'],['go','build','-o','/dev/null','./cmd/issueops'],['go','test','-race','./...','-count=1']]; bad=[' '.join(c[:2]) for c in cmds if subprocess.run(c,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL).returncode]; bad=(['gofmt'] if fmt else [])+bad; print('ALL_PASS' if not bad else 'FAIL '+','.join(bad)); sys.exit(1 if bad else 0)"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
- [x] G3: lint 설정·nolint 억제가 늘지 않고 production Go 파일이 바뀌지 않는다
  CHECK: python3 -c "import subprocess,sys; run=lambda a: subprocess.run(a,capture_output=True,text=True,check=True).stdout; base=run(['git','merge-base','HEAD','origin/main']).strip(); assert base; cfg=run(['git','diff',base,'--','.golangci.yml']); diff=run(['git','diff',base,'--unified=0','--','*.go']).splitlines(); sup=[l for l in diff if l.startswith('+') and 'nolint' in l]; prod=[p for p in run(['git','diff','--name-only',base,'--','*.go']).split() if not p.endswith('_test.go')]; bad=bool(cfg or sup or prod); print('SUPPRESSION_OR_PROD_CHANGE' if bad else 'NO_SUPPRESSION'); sys.exit(1 if bad else 0)"
  EXPECT: NO_SUPPRESSION
  EVIDENCE: NO_SUPPRESSION
- [x] G4: self-verify의 모든 단계(go test ./... -count=1 포함)가 통과한다
  CHECK: python3 -c "import subprocess as s,json; b=s.run(['go','build','-o','bin/issueops','./cmd/issueops'],capture_output=True,text=True); p=s.run(['./bin/issueops','self-verify','--seed=100','--target-score=95','--llm-eval=false','--json'],capture_output=True,text=True); d=json.loads(p.stdout or '{}'); print('SELF_VERIFY_OK='+str(b.returncode==0 and bool(d.get('ok'))))"
  EXPECT: SELF_VERIFY_OK=True
  EVIDENCE: SELF_VERIFY_OK=True
