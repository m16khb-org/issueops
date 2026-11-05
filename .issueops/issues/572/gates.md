# Gates: 572

- [x] G1: package clause가 디렉터리 이름과 같다
  CHECK: python3 -B -c "import subprocess;o=subprocess.run(['go','list','-f','{{.Name}}','./internal/domain/issueopsremote','./internal/domain/guardpattern','./internal/domain/traceclassification'],capture_output=True,text=True,check=True).stdout.split();print(' '.join(o))"
  EXPECT: issueopsremote guardpattern traceclassification
  EVIDENCE: issueopsremote guardpattern traceclassification
- [x] G2a: Effective Go 측정 설정의 production 지적이 darwin에서 0건이다
  CHECK: python3 -B -c "import os,subprocess;r=subprocess.run(['golangci-lint','run','-c','.issueops/issues/572/effective-go-lint.yml','./...'],env={**os.environ,'GOOS':'darwin'},capture_output=True,text=True);print('effective-go-clean' if r.returncode==0 else r.stdout[-2000:])"
  EXPECT: effective-go-clean
  EVIDENCE: effective-go-clean
- [x] G2b: Effective Go 측정 설정의 production 지적이 linux에서 0건이다
  CHECK: python3 -B -c "import os,subprocess;r=subprocess.run(['golangci-lint','run','-c','.issueops/issues/572/effective-go-lint.yml','./...'],env={**os.environ,'GOOS':'linux'},capture_output=True,text=True);print('effective-go-clean' if r.returncode==0 else r.stdout[-2000:])"
  EXPECT: effective-go-clean
  EVIDENCE: effective-go-clean
- [x] G2c: Effective Go 측정 설정의 production 지적이 windows에서 0건이다
  CHECK: python3 -B -c "import os,subprocess;r=subprocess.run(['golangci-lint','run','-c','.issueops/issues/572/effective-go-lint.yml','./...'],env={**os.environ,'GOOS':'windows'},capture_output=True,text=True);print('effective-go-clean' if r.returncode==0 else r.stdout[-2000:])"
  EXPECT: effective-go-clean
  EVIDENCE: effective-go-clean
- [x] G3: 저장소 lint 게이트가 darwin과 linux에서 통과한다
  CHECK: ./scripts/lint-local.sh
  EXPECT: 0 issues.
  EVIDENCE: ==> golangci-lint run ./... (GOOS=linux) | 0 issues.
- [x] G4: 세 GOOS 빌드와 vet이 통과한다
  CHECK: python3 -B -c "import os,subprocess,sys;[subprocess.run(['go',c,'./...'],env={**os.environ,'GOOS':g},check=True) for g in ('darwin','linux','windows') for c in ('build','vet')];print('builds-ok')"
  EXPECT: builds-ok
  EVIDENCE: builds-ok
- [x] G5: gofmt 대상 파일이 없다
  CHECK: python3 -B -c "import subprocess;fs=subprocess.run(['git','ls-files','*.go'],capture_output=True,text=True,check=True).stdout.split();out=subprocess.run(['gofmt','-l',*fs],capture_output=True,text=True).stdout.strip();print(out or 'gofmt-clean')"
  EXPECT: gofmt-clean
  EVIDENCE: gofmt-clean
- [x] G6: 전체 테스트가 race 검사와 함께 통과한다
  CHECK: python3 -B -c "import subprocess;r=subprocess.run(['go','test','-race','./...','-count=1'],capture_output=True,text=True);o=(r.stdout+r.stderr).splitlines();ok=[l for l in o if l.startswith('ok')];bad=[l for l in o if l.startswith(('FAIL','--- FAIL','panic:'))];print('go-test-race-ok' if r.returncode==0 and ok else '; '.join(bad[-10:]) or 'go test failed')"
  EXPECT: go-test-race-ok
  EVIDENCE: go-test-race-ok
- [x] G15: 전체 테스트가 race 없이도 통과한다
  CHECK: python3 -B -c "import subprocess;r=subprocess.run(['go','test','./...','-count=1'],capture_output=True,text=True);o=(r.stdout+r.stderr).splitlines();ok=[l for l in o if l.startswith('ok')];bad=[l for l in o if l.startswith(('FAIL','--- FAIL','panic:'))];print('go-test-ok' if r.returncode==0 and ok else '; '.join(bad[-10:]) or 'go test failed')"
  EXPECT: go-test-ok
  EVIDENCE: go-test-ok
- [x] G16: 세 패키지의 importer 별칭이 한 가지로 정리된다
  CHECK: python3 -B .issueops/issues/572/check_import_aliases.py
  EXPECT: aliases-clean
  EVIDENCE: aliases-clean
- [x] G7: lint-diagnose가 자식 종료 코드를 그대로 돌려주고 stderr에 줄을 더하지 않는다
  CHECK: python3 -B -c "import subprocess;subprocess.run(['go','build','-o','bin/issueops','./cmd/issueops'],check=True);p=subprocess.run(['bin/issueops','project','lint-diagnose','--','sh','-c','exit 7'],capture_output=True,text=True);print(p.returncode,p.stderr.rstrip().splitlines()[-1])"
  EXPECT: 7 ------------------------------------
  EVIDENCE: 7 ------------------------------------
- [x] G8: response-contract golden이 갱신 없이 통과하고 testdata diff가 없다
  CHECK: python3 -B -c "import subprocess;subprocess.run(['go','test','./cmd/issueops/contractgolden','./cmd/issueops/issueopsapp','-run','Golden','-count=1'],check=True);d=subprocess.run(['git','diff','--name-only','7714640b0b40b4b03e64ad1405140d47d26e9c84','--','cmd/issueops/testdata','internal/adapter/testdata'],capture_output=True,text=True,check=True).stdout.strip();print(d or 'golden-unchanged')"
  EXPECT: golden-unchanged
  EVIDENCE: ok  	issueops/cmd/issueops/issueopsapp	4.279s | golden-unchanged
- [x] G9: 현행 문서와 코드에 옛 이름이 없고 dated 기록은 바뀌지 않았다
  CHECK: python3 -B .issueops/issues/572/check_renames.py
  EXPECT: renames-clean
  EVIDENCE: renames-clean
- [x] G10: 저장소 신호가 이름 변경 뒤에도 참이다
  CHECK: go test ./internal/adapter/augmentation -run TestCollectSignalsHoldOnThisRepository -count=1 -v
  EXPECT: --- PASS: TestCollectSignalsHoldOnThisRepository
  EVIDENCE: --- PASS: TestCollectSignalsHoldOnThisRepository (0.89s) | PASS | ok  	issueops/internal/adapter/augmentation	1.018s
- [x] G11: provider 시작 실패 에러가 원인을 보존한다
  CHECK: go test ./internal/adapter/provider/providerutil -run TestBoundedCommandStartErrorUnwrapsCause -count=1 -v
  EXPECT: --- PASS: TestBoundedCommandStartErrorUnwrapsCause
  EVIDENCE: --- PASS: TestBoundedCommandStartErrorUnwrapsCause (0.00s) | PASS | ok  	issueops/internal/adapter/provider/providerutil	0.155s
- [x] G12: stdio MCP가 종료 ctx에서 처리 중인 요청과 EOF 대기를 끝낸다
  CHECK: go test ./cmd/issueops/mcpcli -run TestServeMCPStream -count=1 -v
  EXPECT: --- PASS: TestServeMCPStreamCancelsInFlightRequestOnShutdown
  EVIDENCE: --- PASS: TestServeMCPStreamCancelsInFlightRequestOnShutdown (0.10s) | PASS | ok  	issueops/cmd/issueops/mcpcli	2.630s
- [x] G14: 실제 `issueops mcp` stdio 프로세스가 `channel_recv` 대기 중 SIGTERM에 drain 상한 안에 종료 코드 0으로 끝난다
  CHECK: python3 -B .issueops/issues/572/stdio_signal_smoke.py
  EXPECT: stdio-signal-ok
  EVIDENCE: stdio-signal-ok
- [x] G13: provider 패키지가 processlease를 직접 import하지 않는다
  CHECK: go test ./internal/architecture -run TestProductionGraphHasNoForbiddenAdapterEdges -count=1 -v
  EXPECT: --- PASS: TestProductionGraphHasNoForbiddenAdapterEdges
  EVIDENCE: --- PASS: TestProductionGraphHasNoForbiddenAdapterEdges (4.55s) | PASS | ok  	issueops/internal/architecture	4.706s
