# 남은 검증 문제 후속 수정

**완료:** 최종 self-verify 28/28 통과, 최소 목표 점수 100,
`termination_eligible=true`. native·Linux lint도 모두 0 issues다.

이 문서는 [이전 결과](result.md)의 문서·Omo 검증·린터 실패를 해결하는 후속 기록이다.
이전 하네스 구현 변경을 보존했고 사용자 전역 설정과 인증은 수정하지 않았다.

## 원인과 수정

### 문서 fixture와 redaction

`host-usage-verification.md`와 `shared-http-verification.md`의 개인 경로를
`$WORKSPACE`로 정규화했다. 전자의 합성 인증 입력 원문도 제거했다.
과거 실행 결과와 판정은 유지했으며, fixture fingerprint와 redaction 규칙은
바꾸지 않았다. 기존 회의록 contract 검사 15개가 통과했다. 기존 optional
local-file skip 1개는 그대로다.

### Omo HTTP native integration

기존 `hasCanonicalOmoMCP`는 stdio의 command/args/env만 허용해 실제 HTTP 설치를
잘못 거부했다. 기존 stdio 계약을 유지하고 다음 HTTP 계약을 추가했다.

- type은 http, endpoint는 literal loopback IP의 명시적 유효 port와 `/mcp` 경로다.
- userinfo, query, fragment와 원격 주소를 거부한다.
- Authorization은 서버와 같은 최소 256-bit base64url Bearer 형식이다.
- HTTP entry에 command/args를 섞거나 header에 공백·개행을 넣으면 거부한다.

정상 HTTP 두 사례가 기존 구현에서 실패하는 RED를 확인했다. 새 표 기반 테스트는
IPv4·IPv6·custom port와 잘못된 주소·경로·인증·hybrid 설정을 함께 검증한다.
일반 오류는 설정 원문이나 인증 값을 노출하지 않는다.

### 린터와 Go toolchain

`.golangci.yml`과 CI를 golangci-lint v2.12.2/action v8로 맞췄다. CI는 기존처럼
go.mod toolchain으로 린터를 빌드한다. 로컬도 같은 toolchain으로 분석한다:

```sh
GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" golangci-lint run ./...
GOTOOLCHAIN="go$(go list -m -f '{{.GoVersion}}')" GOOS=linux golangci-lint run ./...
```

v1 설정은 공식 `migrate` 명령으로 임시 경로에서 변환해 대조했다.
제외 preset과 issue 출력 한도를 그대로 유지했다. v2 staticcheck의 기본값은
기존 v1보다 넓으므로 원래 staticcheck(SA)·gosimple(S1) 집합을 명시했다.
기존에 쓰지 않던 stylecheck(ST)·quickfix(QF)를 이번 호환성 수정에서 추가하지 않았다.

이 구분은 upstream v1 구현으로 확인했다:
[staticcheck](https://github.com/golangci/golangci-lint/blob/v1.64.8/pkg/golinters/staticcheck/staticcheck.go),
[gosimple](https://github.com/golangci/golangci-lint/blob/v1.64.8/pkg/golinters/gosimple/gosimple.go).

실제 기존 검사 집합의 오류는 다음과 같이 수정했다.

- HTTP 서버 instance 해제 오류를 반환 오류에 결합한다.
- 잠금·State 테스트의 Rollback/CloseRoot 오류를 검사한다.
- deprecated SDK Logging 필드 직접 접근 대신 직렬화된 capability 필드의
  부재를 확인한다. handshake의 tools/resources 검사도 유지한다.
- 언어 서버가 정의 외 참조 0개로 확인한 미사용 helper 3개를 제거했다.
  production helper 삭제는 정규 DDD inventory 생성기로 반영했다.

## 검증 명령

```sh
GOTOOLCHAIN=go1.26.3 golangci-lint config verify
GOTOOLCHAIN=go1.26.3 golangci-lint run ./...
GOTOOLCHAIN=go1.26.3 GOOS=linux golangci-lint run ./...
go test -race ./internal/adapter/verification/probe/nativeintegration -count=1
python3 -m unittest discover -s scripts -p meeting_notes_skill_contract_test.py
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
```

운영 규칙은 [테스트 문서](../../testing/unit-and-contract.md)에,
재발 방지 근거는 [주의사항](../../cautions/lessons/2026-10-03-native-http-validation-and-lint-toolchain.md)에
반영했다. 문서 수정은 읽은 소스와 실행 근거를 기준으로 `apply_patch`를 사용했다.
커밋·푸시는 하지 않았다.

## 확인된 실행 결과

- `golangci-lint config verify`: 통과.
- Go 1.26.3의 native lint: `0 issues`, exit 0.
- 같은 toolchain의 `GOOS=linux` lint: `0 issues`, exit 0.
- HTTP·SDK focused race 검사: nativeintegration, mcpcli, issueopsapp 모두 통과.
- 기존 문서 식별 정보 검사: 15개 실행, 성공, 기존 optional skip 1개.
- CI YAML을 파싱해 setup-go의 go.mod 선택과 action v8/v2.12.2/goinstall 연결을 확인했다.
  실제 GitHub Actions 실행은 커밋·푸시하지 않았으므로 수행하지 않았다.
- 운영 문서 checker: `ok=true`, `violations=[]`.

## 최종 단일 run

Go 1.26.3과 준비한 Python 3.13 환경에서 최신 binary를 빌드하고 실행했다.
원본 JSON은 `/tmp/issueops-remaining-final.json`이다.

```sh
GOTOOLCHAIN=go1.26.3 \
PATH="/tmp/issueops-improvement-python.QksUWu/bin:$PATH" \
  ./bin/issueops self-verify --seed=100 --target-score=95 \
  --llm-eval=false --progress=jsonl --json
```

- exit 0, `ok=true`, 28개 step 모두 성공, 소요 364,511ms.
- `minimum_goal_score=100`, `termination_eligible=true`.
- `risk QA tier`에서 `go test -race ./... -count=1 && go vet ./...`가 통과했다.
  같은 run의 전체 테스트·golden은 이 성공한 full-suite 증거를 재사용했다.
- build, Python, native integration, redaction audit, CLI/MCP·state smoke와
  나머지 QA step도 모두 통과했다. 과거 run의 부분 성공을 합치지 않았다.
- 이번 후속 요청에서 다룬 검증 실패는 남아 있지 않다.
