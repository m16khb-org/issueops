# 레포 효율성 조사와 출력 캡처 개선

## 결론

후속 요청으로 나머지 14개 후보도 구현했다.
변경·측정·검증 근거는 [14개 구현 결과](repo-efficiency-implementation-2026-10-01.md)에 기록했다.
아래 내용은 최초 조사와 우선 개선 선정 당시의 근거다.

우선 개선 대상으로 검증 명령의 출력 캡처를 선정했다. 출력 예산은 최종 문자열에만
적용되고, 실행 중에는 출력 전체를 메모리에 모으고 있었다. 실제 `Run` 호출에서
32 KiB 예산으로 32 MiB를 출력하면 부모 프로세스가 약 160 MiB를 누적 할당했다.
반환 문자열만 검사하는 기존 테스트로는 이 문제를 잡을 수 없었다.

큰 파일이나 비슷한 코드라는 이유만으로 구조를 바꾸지는 않았다. 안전을 위한
재관측, provider별 차이, prefix와 tail의 다른 계약은 중복 제거 대상에서 제외했다.

## 조사 범위와 한계

기준 커밋은 `82d4c647`이다. 추적 파일 4,081개를 다음과 같이 빠짐없이 배정했다.
전체 영역의 파일·패키지 목록을 확인하고 각 영역의 주요 실행 경로를 상세히 읽었다.
모든 파일의 모든 줄을 검토했다는 뜻은 아니다. 생성된 OpenWiki와 과거 사고 기록은
현재 소스와 동일한 권위로 취급하지 않았다.

| 조사 영역 | 추적 파일 수 | 상세 확인의 중심 |
| --- | ---: | --- |
| domain·contract·port | 869 | 출력 계약, 상태 분류, 집계, UTF-8 경계 |
| application | 431 | next, inventory, readiness, cleanup, publication |
| IssueOps 실행 어댑터 | 302 | 프로세스 관측, cleanup, sync-base, 상태 읽기 |
| outbound | 157 | SQLite, record inventory, bounded I/O, 유지보수 |
| host·설치 어댑터 | 94 | 설치·readback, hooks, quoting, host probe |
| 나머지 실행 어댑터 | 452 | 검증 runner, policy, provider, Orca, cmux, worker |
| CLI·MCP | 735 | dispatch, SDK routing, daemon, transport 계약 |
| 검증·CI·fixture | 134 | architecture 검사, CI 소유권, 검사 스크립트 |
| 스킬·설정 | 203 | 스킬 목록, 중복, 링크, Python 검사 연결 |
| 문서·나머지 파일 | 704 | canonical 문서, 과거 자료, root 설정 |

영역별 보고서는 파일 목록만 확인한 범위와 상세히 읽은 파일을 구분한다.
메모리 재현은 메인이 직접 실행했다. 아래의 다른 성능 후보는 측정값이 없는 한
운영 환경의 병목이나 이미 입증된 속도 개선으로 해석하면 안 된다.

## 선정 근거와 측정

실행 경로는 `verification.Run` → `RunEnv` →
`BudgetCommandOutput`이다. 기존 `process.go`는 stdout/stderr 각각의
`bytes.Buffer`를 전체 문자열로 변환한 뒤 예산을 적용했다.

32 KiB 예산으로 실제 subprocess가 출력하도록 하고, 부모 프로세스의
`runtime.MemStats.TotalAlloc` 차이를 측정했다.

| 출력 크기 | 수정 전 누적 할당 | 수정 후 누적 할당 | 반환 출력 |
| ---: | ---: | ---: | ---: |
| 8 MiB | 42,011,056 bytes | 236,776 bytes | 32 KiB |
| 16 MiB | 83,958,856 bytes | 235,336 bytes | 32 KiB |
| 32 MiB | 167,843,272 bytes | 235,496 bytes | 32 KiB |

세 경우 모두 출력·byte 수·절단 여부는 맞았지만 고정 4 MiB 할당 상한에서 실패했다.
이는 peak RSS나 retained heap 측정이 아니다. 할당량이 총 출력에 비례한다는 증거다.

개선 계약은 양수 예산 B에서 스트림별 O(B) 메모리로 마지막 B bytes만 보관하면서
초과 출력도 계속 소비하고 전체 byte 수를 세는 것이다. UTF-8은 write chunk마다
자르지 않고 최종 tail을 렌더링할 때 처리한다. 0 이하 command budget은 기존처럼
무제한이다. timeout·종료 코드·환경 변수·stdin·DTO는 변경하지 않는다.

수정 후 같은 세 입력은 모두 4 MiB 할당 상한을 통과했다. 32 MiB 사례의 누적
할당은 약 99.86% 감소했다. 첫 구현의 eager allocation도 별도로 검증했다.
4 MiB 예산에 2 bytes만 출력했을 때 8,502,136 bytes가 할당되는 문제가 있어,
버퍼를 실제 출력량에 따라 성장하도록 보완했다. 같은 실행은 이후 100,984 bytes를
할당했다. 이 값들도 단일 로컬 실행의 누적 할당이며 서비스 전체 속도 향상을
뜻하지 않는다.

## 개선 후보와 우선순위

아래 숫자는 측정 점수가 아니라 비교를 위한 판단이다. 영향·실현성은 5가 높고,
위험은 5가 크다. 실제 효과가 재현된 후보를 추상적인 구조 정리보다 우선했다.
문제를 작은 실행 경계로 나누고, 가치·실현성·위험을 함께 비교했다.

| 후보 | 영향 / 실현성 / 위험 | 근거와 판단 |
| --- | --- | --- |
| 검증 출력의 bounded capture | 5 / 5 / 2 | 누적 할당 선형 증가를 실측했다. 이번 개선으로 선정했다. |
| `next --id`의 전체 inventory scan | 4 / 3 / 3 | `application/issueopsnext/service.go:66-89`. 전체 조회 후 같은 record를 다시 읽는다. root-conflict 진단 보존이 필요하다. |
| repo 필터 전 전체 record decode | 3 / 3 / 4 | `application/issueopsinventory/service.go:30-53`. 정규화는 요청 내에서 이미 재사용한다. 영속 index 변경은 별도 설계가 필요하다. |
| readiness와 cleanup의 Git 중복 관측 | 3 / 3 / 4 | `application/issueopscycle/readiness_service.go:43-46,90-112`. 요청 내 사실 공유 후보지만 재관측 안전성부터 검증해야 한다. |
| cleanup의 전역 lsof 경로 처리 | 3 / 3 / 3 | `adapter/issueops/execution_process.go:276-279,390-397`. 영역 조사에서 약 5.3 MB 출력을 관측했다. `+D` 전환은 대형 트리에서 더 느릴 수 있어 제외했다. |
| cleanup ancestry 반복 탐색 | 2 / 4 / 3 | `adapter/issueops/cleanup_workspace_processes.go:67-104,131-151`. 요청자·점유자·전체 PID를 중복 순회한다. 스냅샷 내 메모이제이션 후보다. |
| sync-base blocker 이후 Git probe | 3 / 3 / 3 | `adapter/issueops/execution_sync_base.go:154-229`. 조기 반환 후보지만 보조 진단과 fetch 계약을 확인해야 한다. |
| 상태 읽기의 이중 validation | 2 / 4 / 3 | `adapter/issueops/issueops_state.go:28-63,84-102`. decoder와 caller가 같은 성공 record를 재검사한다. schema·invariant 검사는 유지해야 한다. |
| operational-health의 반복 선형 검색 | 2 / 3 / 3 | `domain/operationalhealth/classifier_resources.go`. cycle별 unique lookup과 역색인 비교가 반복된다. 대규모 fixture 측정 전에는 미채택한다. |
| architecture의 반복 AST 파싱 | 2 / 4 / 2 | `internal/architecture/ddd_responsibility_test.go:62,144`. 한 실행의 공유 inventory 후보다. 독립 byte-stability 관측은 유지해야 한다. |
| 스킬 내부 Python 테스트의 자동 실행 | 4 / 3 / 3 | `application/selfverify/steps.go:69`는 root `scripts`만 발견한다. 스킬별 import·격리 계약을 확인한 별도 실행기가 필요하다. |
| `verify-go-test-match` 자체 검사의 자동 연결 | 2 / 4 / 2 | 스크립트와 self-test는 있지만 현재 기본 검사 경로에서 실행되지 않는다. 검사 소유권을 한 곳에 정해야 한다. |
| MCP 구형 dispatch와 SDK dispatch | 3 / 3 / 3 | `cmd/issueops/mcpcli/mcp_tools.go:124-166`, `mcp_sdk_server.go:125-151`. 기존 wiring 테스트가 구형 경로를 사용한다. 실제 SDK 경로로 옮기는 별도 작업 후보다. |
| host 설치 JSON merge·readback 중복 | 2 / 4 / 2 | `adapter/{agy,omo}/mcp.go`, 각 activation 경로. host별 차이를 남긴 좁은 공통화 후보다. |
| SQLite cache hit의 root 전체 검사 | 2 / 2 / 4 | `adapter/outbound/sqlstore/sqlstore.go:121-149`. 파일 권한과 제거된 root 정리가 안전 불변식이라 측정 없이 생략하지 않는다. |

## 제거하면 안 되는 반복

- `application/issueopsreview/local_changes.go:15-38`의 두 스냅샷 읽기는
  변경 도중의 불안정한 관측을 거부하기 위한 것이다.
- cleanup finish와 abandon은 유사해도 효과 순서·실패 단계가 다르다.
  하나의 범용 executor로 합치는 것은 이번 개선 범위를 넘는다.
- GitHub와 GitLab branch 준비 절차는 provider 계약이 다르다.
- policy의 bounded 출력은 prefix·redaction 계약을 가진다. 검증 runner의
  tail 보존 구현으로 대체하거나 그대로 빌려 쓰면 의미가 달라진다.
- `go list`를 다시 실행해 정렬 결과의 안정성을 확인하는 architecture 검사는
  동일한 목록을 캐시에서 꺼내 비교하는 것으로 바꿀 수 없다.
- 반환값이나 경로를 전역 cache에 저장하면 다른 workspace와 변경된 worktree에
  잘못된 결과를 재사용할 수 있다. 필요하다면 요청 범위 안에서만 공유한다.

## DAG와 복구

조사 DAG는 10개 영역 조사 후 후보 검증으로 합류한다.
구현 DAG는 6.1 Sol의 구현·회귀 검사 후 Luna의 독립 검증으로 이어진다.
메인은 재현, 범위 확정, 실제 diff 검토와 전체 검증을 담당한다.

- 조사 run: `dag_105da9e0-abd2-40d1-959b-af7d18c00c6b`
- 구현 run: `dag_3bdfc3de-be91-4105-8264-7ef4a1373aca`
- 세션의 `/dag`에서 노드와 의존 관계를 볼 수 있다.

초기 모델 시작 실패와 호스트 재시작으로 일부 자식 상태가 유실됐다.
완료된 보고서를 보존하고 실패·유실된 영역만 복구했다. 코어 영역은 메인이
직접 보완했다. 모델 지정과 실제 fallback 실행은 구분했으며, 후속 노드는
현재 카탈로그의 GPT-6 Luna와 GPT-6.1 Sol을 명시했다.

## 검증 기록

- 수정 전 self-verify: 27/27 단계 통과, 최저 목표 점수 100, 약 323초.
- 수정 전 allocation 재현: 8/16/32 MiB 모두 고정 상한에서 실패했다.
- 사전 독립 리뷰: 정확성·검증 적정성 모두 PASS. 구현 검증을 대신하지 않는다.
- 메인이 실행한 focused race 검사: `internal/domain/selfverify`와
  `internal/adapter/verification/...`의 모든 보고된 패키지가 통과했다.
- 동일 overlay의 대량 출력 세 사례와 작은 출력 사례가 모두 통과했다.
- 변경 Go 파일의 LSP 오류와 `git diff --check` 오류가 없었다.
- 기존 formatter와 chunk별 differential 비교, UTF-8/invalid bytes, tiny·0·음수
  예산, 동시 stdout/stderr, 실패 종료와 실제 할당 상한을 인접 테스트로 검증한다.

최신 binary 빌드와 macOS `golangci-lint run ./...`는 통과했다.
새 버퍼 파일과 formatter 심볼은 정규 DDD collector의 생성 결과대로 등록했으며,
생성 결과의 byte 일치와 `go test -p 1 ./internal/architecture -count=1`도 통과했다.

첫 변경 후 전체 검증은 디스크 부족으로 중단됐다. Linux lint와 self-verify의 전체 race
검사가 `no space left on device`로 실패했다. 다른 검사와의 병행을 중단하고
`GOFLAGS=-p=1 GOMAXPROCS=2`로 self-verify를 첫 단계부터 다시 실행했지만,
일부 테스트의 임시 디렉터리 생성이 같은 오류로 실패했고 race 단계는 10분 제한을
넘겼다. 중단 당시 디스크 여유는 423 MiB였다. 공유 Go 빌드 캐시 43 GiB는
승인 없이 삭제하지 않았다.

사용자가 디스크를 정리한 뒤 54 GiB의 여유 공간을 확인했고, 아래 전체 게이트를
첫 단계부터 재개했다. 재개 실행의 최종 JSON과 종료 코드는 세션 완료 보고의
검증 근거로 제공한다. 부분 통과나 수정 전 27/27 결과를 변경 후 전체 검증의
성공으로 합치지 않는다. self-verify의 risk QA가
실제로 실행한 vet/race 명령을 확인하고, 같은 성공 실행에 포함된 전체
테스트·build·golden은 중복 실행하지 않는다.

```sh
go build -o bin/issueops ./cmd/issueops
./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json
golangci-lint run ./...
GOOS=linux golangci-lint run ./...
```

재개한 위 단일 실행은 exit 0으로 완료됐다. self-verify는 435,627 ms에
27/27 단계 통과, 최저 목표 점수 100, `termination_eligible=true`를 기록했다.
`risk QA tier`는 `go test -race ./... -count=1 && go vet ./...`를 실제로
실행해 통과했다. build·golden·docs·inspect·MCP smoke·QA gate도 통과했고,
macOS와 Linux lint는 모두 진단 출력 없이 exit 0이었다.
