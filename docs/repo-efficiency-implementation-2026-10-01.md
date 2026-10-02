# 효율성 후보 14개 구현 결과

## 범위

[전체 조사](repo-efficiency-audit-2026-10-01.md)의 후속 후보 14개를 모두 구현했다.
이전 출력 캡처 개선은 그대로 보존했다. 기존 fail-closed 계약을 없애서 속도를
높이는 제안은 채택하지 않고, 같은 목적을 만족하는 안전한 범위로 구현했다.
아래 수치는 로컬 측정과 호출 수 근거이며 서비스 전체 latency나 SLO를 뜻하지 않는다.

## 14개 결과

| 번호 | 구현 | 효과와 보존 조건 |
| --- | --- | --- |
| 1 | `issueopsnext/selected.go`, record store의 `ReadSelected` | 실행 정보가 있는 명시/env ID는 strict read 1회, list·sibling scan 0회. 저장소 경계·ID 우선순위를 유지한다. 실행 전 branch의 root 충돌 검사는 안전상 stream으로 수행한다. |
| 2 | `sqlstore.WalkExisting` → `Store.ScanEach` → filtered inventory | 전체 raw/decoded slice 보관을 없애고 일치하는 record만 보관한다. 모든 row의 strict decode와 진단은 유지한다. 요청 repo 정규화를 재사용한다. |
| 3 | readiness operation별 observation factory | branch/status는 같은 operation 안에서 각각 2회 → 1회. 다음 요청·동시 요청·fetch 경계에서는 새로 관측한다. |
| 4 | lsof parser의 raw-path별 containment memo | 같은 snapshot의 동일 경로는 한 번만 resolve한다. false 결과도 재사용하며 symlink·FD/access·상대 경로 거부와 결과 순서를 유지한다. |
| 5 | cleanup snapshot의 PID별 ancestry memo | N=100/1,000에서 각각 100/1,000회 계산, PID당 한 번. 오류도 재사용하지만 requester/occupant 거부와 broken descendant skip은 그대로다. |
| 6 | sync-base의 확정 blocker 조기 반환 | 거부된 preview/apply에서 `ls-remote`·fetch 0회. 관측하지 않은 원격 사유를 만들어 넣지 않는다. 성공 경로와 적용 fence는 유지한다. |
| 7 | strict state decoder의 단일 검증 책임 | 성공한 read의 record/domain validation 2회 → 1회. schema·ID·shape·invariant 거부와 mutation 검증은 유지한다. |
| 8 | operational-health 요청별 count+unique 인덱스 | cycle/resource 반복 선형 검색을 제거한다. 중복을 덮어쓰지 않고 lease-holder 양방향 확인·전체 정렬 Findings를 유지한다. |
| 9 | architecture의 root별 immutable AST snapshot | DDD consumer와 lease-codec 검사가 AST를 공유한다. mutable metadata는 복사하고, fresh collection과 독립 go-list 안정성 검사는 보존한다. |
| 10 | root 및 skill Python suite runner | root를 한 번, 스킬별 별도 process로 실행한다. 기존 스킬 파일 10개·156개 테스트를 실제 실행했다. import/CWD와 동명 모듈을 격리한다. |
| 11 | `Go test match guard` 기본 단계 | Python 다음에 기존 shell self-test를 30초 한도로 실행한다. 누락·실패는 완료를 차단한다. 기본 계획 28단계, summary contract v6, snapshot schema v1이다. |
| 12 | MCP의 단일 SDK dispatch | 구형 test dispatcher를 제거하고 실제 SDK CallTool로 테스트한다. typed direct content와 `IsError` 변환 결함을 함께 수정했다. |
| 13 | host JSON merge·readback 공통화 | installutil의 좁은 helper를 주입한다. agy/Omo/Claude의 경로·권한·진단·dry-run 차이와 Codex TOML은 유지한다. |
| 14 | existing SQLite root의 중복 디렉터리 검사 제거 | fresh Lstat 결과를 재사용해 중복 MkdirAll/Stat을 없앤다. 매 Open의 삭제된 root 정리와 permission/sidecar 검사는 그대로다. |

`next`는 현재 제공되는 CLI 기능이다. 새 `issueops_next` MCP 도구를 추가하지 않았다.
실제 CLI의 root conflict, foreign repo, alias, 환경 ID, missing, corrupt, auto
7개 사례를 실행했다. MCP 검증은 실제 제공되는 SDK 도구와 CLI/MCP 동등성 경로에서
수행했다.

## 정량 근거

### 저장소 필터

실제 SQLite 데이터를 준비한 뒤 seeding을 제외하고 측정했다.

| row 수 | 기존 B/op | stream B/op | 정규화 호출 |
| ---: | ---: | ---: | --- |
| 100 | 296,656 | 219,656 | 3 → 2 |
| 1,000 | 2,839,341 | 2,111,589 | 3 → 2 |
| 10,000 | 29,365,618 | 21,202,864 | 3 → 2 |

할당 bytes가 약 26–28% 줄었다. 메인 재측정의 10,000 row는 21,202,960 B/op였다.
일부 timing 표본은 느려졌으므로 latency 개선은 주장하지 않는다.
관련 없는 손상 row도 진단하고, clock은 scan 종료 후 한 번 읽는다.

### operational-health

| cycle 수 | 기존 ns/op | 구현 후 ns/op | 기존 allocs/op | 구현 후 allocs/op |
| ---: | ---: | ---: | ---: | ---: |
| 100 | 1,654,042 | 321,541 | 11,468 | 1,777 |
| 1,000 | 171,066,042 | 4,140,750 | 1,013,347 | 16,324 |
| 10,000 | 12,792,276,167 | 61,465,542 | 100,131,327 | 161,418 |

메인 재측정의 10,000 cycle은 약 40.0 ms/op, 161,417 allocs/op였다.
입력 불변성, 기존 결과 digest, duplicate identity, legacy task 및 lease-holder
역색인 회귀 검사를 함께 통과했다.

### AST와 SQLite

- 같은 1,464개 source·78개 artifact의 3회 수집: 1.1566초 → 0.4285초.
  fixture counter로 root별 load 1회와 source별 parse 1회를 검증했다.
- SQLite의 1-root 표본: 40 → 38 allocs/op. 10/100-root에서도 2 allocations가
  줄었다. 여전히 필요한 O(N) root sweep을 제거했다고 주장하지 않는다.

## 검증과 테스트 환경

모든 항목의 인접 검사와 독립 검토를 통과했고, 메인이 관련 race 검사 및 실제
next CLI·SDK·host adapter matrix를 다시 실행했다. 인벤토리는 정규 collector로
생성했으며, 기존 policy 목록을 보존했다. CLI/MCP 및 response contract golden은
변경 없이 통과했다.

Python은 기존 Slack PEP 723 의존성을
`scripts/python_test_requirements.txt`에 고정했다. CI는 checkout 밖의
`RUNNER_TEMP`에 Python 3.13 환경을 준비한다. 로컬 준비법은
[Python 검사 환경](../.issueops/testing/unit-and-contract.md#python-검사-환경)에 있다.
이는 테스트 환경이며 native installer가 다른 도구나 라이브러리를 설치하지 않는다.

실행 결과는 root 61 tests와 skill 156 tests였다. root의 기존 optional local-file
skip 1개는 unittest 의미대로 유지했다. skill skip·import 오류·빈 테스트 파일·실패는
검사 실패다. Python 재실행 명령도 expanded runner를 사용한다.

최종 단일 실행은 28/28 단계, 최소 100점, coverage gap 0개,
`termination_eligible=true`로 통과했다. 전체 race와 vet를 실제 실행했고,
최신 binary의 CLI/MCP smoke, build, macOS/Linux lint도 통과했다.
개별 검사나 과거 27단계 결과를 이어 붙인 판정이 아니다.

로컬 증거 디렉터리는 `/tmp/issueops-all14-S4SQ0t/`이다.

- `final-self-verify-5.json`: 최종 28단계 결과. 345.670초, summary contract v6.
- `full-race-3.stdout`, `full-race-3.stderr`: 마지막 실행의 전체 race 원문.
- `lint-native-final.log`, `lint-linux-final.log`: 두 lint 모두 exit 0, 진단 없음.
- `implemented-1.md`부터 `implemented-14.md`: 항목별 구현·회귀·측정 근거.

통합 중 드러난 inbound scanner fake, host installer dependency 주입,
risk QA 단계 인덱스도 갱신했다. 기존 검사를 삭제하거나 완화하지 않았다.
readiness의 scope 초기화 호출은 보존하고 사용하지 않는 대입만 제거했다.

## DAG와 복구

- 설계: `dag_6b39ac07-950f-4c4e-b393-b579e16212bd`
- 구현 A: `dag_bad77b85-6796-4244-8918-4d6c0a23677f`
- 구현 B: `dag_8ca38116-f0b6-439d-b37a-07d3c0c11182`

`/dag`에서 의존 관계와 실행 결과를 볼 수 있다. 조회·독립 검토는 GPT-6 Luna,
구현은 GPT-6.1 Sol, 종합과 검증은 메인 Astra가 맡았다.
AST 노드의 모델 접근 오류와 guard 노드의 범위·재개 용량 문제는 메인이 보완했다.
완료된 결과는 재사용했고, 실패 때문에 다른 구현을 다시 실행하지 않았다.
커밋·push·실제 native 설치·원격 쓰기는 하지 않았다.
