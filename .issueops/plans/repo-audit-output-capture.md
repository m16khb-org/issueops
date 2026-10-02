# 전체 조사와 검증 출력 캡처 개선

## 요청과 범위

레포 전체의 비효율·병목·중복을 조사하고, 실제로 재현한 개선을 구현한다.
조사 범위는 기준 커밋 `82d4c647`의 추적 파일 4,081개다. 전체 파일을 영역별로
분류하되 상세히 읽은 파일과 목록만 확인한 파일은 조사 보고서에서 구분한다.
조사 DAG는 10개 영역 조사 후 후보 검증으로 합류한다.

모델 배분은 사용자의 후속 지시에 따라 메인 Astra, 조회 Luna, 구현 6.1 Sol을
사용한다. 실행 도구가 해당 모델을 거부하면 실제 오류와 대안을 명시한다.
완료된 조사 결과는 재사용한다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md`: 측정된 문제만 최적화하며 정확성을 먼저 지킨다.
- `.issueops/CONVENTIONS.md`, `conventions/go-and-packages.md`: 순수 규칙은 domain,
  process 실행은 adapter에 둔다. concrete adapter 사이에 새 의존을 만들지 않는다.
- `.issueops/TESTING.md`, `testing/self-verification.md`: 최종 단일 self-verify와
  실제 실행 증거를 사용한다. Go 변경은 vet/race의 실행 여부까지 확인한다.
- `.issueops/SUB_AGENT_PATTERNS.md`: 조사에는 `parallel-independent-research`와
  `high-volume-exploration`을 사용한다. 이득은 context isolation과 parallel speed,
  비용은 시작 오류와 종합 작업이다. 실패 영역은 메인이 보완하고 보고서를 검증한다.
- 독립 리뷰는 `devils-advocate-review`로 정확성과 검증 적정성을 별도 판정한다.
- 생성된 OpenWiki, 사용자 홈 설정, 원격 이슈·PR, Git 이력은 변경하지 않는다.

## 확인된 문제

`internal/adapter/verification/process.go:31-36`은 stdout/stderr를 제한 없는
`bytes.Buffer`로 모으고, 프로세스가 종료된 뒤 문자열로 변환하여 예산을 적용한다.
32 KiB 예산의 실제 `Run` 호출에서 다음 누적 할당을 측정했다.

| 출력 | 부모 프로세스 누적 할당 | 최종 보관 출력 |
| ---: | ---: | ---: |
| 8 MiB | 42,011,056 bytes | 32 KiB |
| 16 MiB | 83,958,856 bytes | 32 KiB |
| 32 MiB | 167,843,272 bytes | 32 KiB |

이는 peak RSS 측정이 아니다. 출력 크기에 비례하는 할당을 재현한 결과다.
수정 전 self-verify는 27/27 단계 통과, 최저 목표 점수 100이었다.

## 구현 계약

1. 양수 예산 B에는 스트림별 O(B) 저장 공간으로 마지막 B bytes만 보관한다.
   모든 출력은 계속 소비하고 전체 byte 수는 정확히 누적한다.
2. 순수 버퍼 구현을 `internal/domain/selfverify`에 두고 기존 출력 formatter와
   절단 마커 계산을 공유한다. ring buffer는 chunk마다 전체 보관분을 복사하지 않는다.
   작은 출력에 큰 예산을 미리 할당하지 않도록 실제 출력량에 따라 성장하되,
   capacity는 B를 넘지 않는다. 빈 스트림은 ring 저장 공간을 할당하지 않는다.
3. 기존 `TailWithBudget`와 `BudgetCommandOutput` 결과를 보존한다.
   non-positive command budget은 기존과 같이 무제한이며, 빈 출력, 아주 작은
   예산, UTF-8 chunk 분할, 원래/생략 byte 수를 보존한다.
4. `internal/adapter/verification/RunEnv`의 두 `bytes.Buffer`를 해당 버퍼로 바꾼다.
   timeout, process group, 종료 코드, 환경 변수, stdin, DTO/schema는 바꾸지 않는다.
5. buffer 경계와 실 subprocess 양쪽에 회귀 테스트를 둔다. 양쪽 스트림을 큰
   출력으로 채워도 부모의 누적 할당이 고정된 여유 상한 안에 있어야 한다.

## 대안과 제외 범위

- 전체 캡처 후 문자열 절단은 현재 문제를 해결하지 않는다.
- 임시 파일 스풀은 메모리를 줄이지만 불필요한 디스크 I/O와 정리 책임을 늘린다.
- policy의 bounded writer는 prefix·redaction 계약이 달라 그대로 재사용하지 않는다.
- 모든 runner의 공통화, 프로세스 취소 정책 변경, 전역 cache, 대규모 구조 재편은
  이번 개선에 포함하지 않는다. 다른 조사 후보는 근거·위험·우선순위와 함께 보고한다.

## 실행과 검증

- 재현: 외부 overlay 테스트가 실제 `Run`에 8/16/32 MiB를 전달한다.
  현재 세 경우 모두 4 MiB 할당 상한에서 실패한다.
- 구현: domain 버퍼·formatter와 verification adapter 및 인접 테스트만 수정한다.
- 회귀: 기존 formatter와 chunk별 결과 비교, tiny/zero/negative budget,
  ASCII/UTF-8, stdout/stderr 동시 출력, 성공/실패 종료를 검증한다.
- focused: `go test -race ./internal/domain/selfverify ./internal/adapter/verification/... -count=1`.
- 동일 overlay 명령을 수정 후 실행하여 세 크기 모두 상한 통과를 확인한다.
- 진단: 변경한 Go 파일의 LSP 오류가 없어야 한다.
- 최종: 최신 binary 준비 후 `self-verify --seed=100 --target-score=95 --llm-eval=false --json`.
  self-verify가 실행하지 않은 `go vet ./...`와 `go test -race ./... -count=1`을 별도 수행한다.
- 실제 표면: 최신 binary의 self-verify가 이 runner로 test/build/smoke를 실행하며,
  JSON의 byte 수와 truncation 필드를 확인한다.
- 최종 diff에서 범위 밖 변경과 사용자 변경 훼손이 없음을 확인한다. 커밋하지 않는다.

## 완료 기준

조사 범위·후보·기각 이유가 보고서에 있고, 선택한 개선이 코드에 반영되며,
기존 출력 계약과 고정 메모리 상한이 실제 실행으로 증명되어야 한다.
DAG의 노드 완료 표시만으로 작업 완료를 주장하지 않는다.
