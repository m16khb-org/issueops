# 구현 실행 원장

## 현재 기준

- 사용자 승인: I1–I10 전부 구현, 공유 MCP는 mutation 포함.
- 시작 HEAD: `92eaa00143841f964c285dacdd41aedbeeecdf2e`.
- 사용자에 의한 `rootchat-design-system.zip` 제거는 유지한다.
- 구현에 대한 commit/push는 아직 요청되지 않았다.

## 설계 DAG

run: `dag_129d6144-7f08-4a16-a36b-57edd1b37099`.

| 노드 | 명시한 모델 | 현재 역할 |
|---|---|---|
| correctness-map | GPT-6 Luna | I1/I2 수정·테스트 경계 |
| host-mcp-contracts | GPT-6.1 Sol | I3/I4/I5 공개 계약 |
| request-io-map | GPT-6 Luna | I6 중복 관측 경계 |
| catalog-read-map | Claude Sonnet 5.5 | I7 bounded discovery |
| observability-map | GPT-6.1 Sol | I8/I9 관측 계약 |
| shared-authority-design | Claude Opus 5.5 | I10 요청별 실행 권한 |
| contract-reducer | GPT-6 Astra | 통합 계약·파일 소유권 |
| contract-review | Claude Fable 5.1 | 독립 설계 검증 |

### 라우팅 복구

`model:high`와 `model (high)`는 task 자식 profile 검사에서 거부됐다.
신규 project category도 현재 task dispatcher에서 인식되지 않았다.
코드 작업은 시작되지 않았고, 같은 DAG를 amend해 실패 노드만 복구했다.

현재는 이미 등록된 사용자 `metis` profile의 high reasoning 설정과 순수
모델 ID를 조합한다. 이는 task config에 정의된 사용자 profile이며,
각 노드의 실제 역할은 별도 TASK/SCOPE로 제한한다.
Luna 완료와 Sol·Sonnet·Opus 실행의 모델 ID는 task/workflow 영수증으로 확인했다.
task 상태 API는 model을 노출하지만 실제 provider 요청의 effort 값은 노출하지
않으며 `PI_THINKING_LEVEL`도 첫 자식에 export되지 않았다.
high 설정 근거와 모델 실행 근거를 구분하며, 노출되지 않은 API 값을 관측했다고
주장하지 않는다.

전역 config는 수정하지 않았다. 시험용 `.omo/omo.jsonc`와 자동 생성된
`omo.jsonc.bak.2026-10-02T07-08-54-620Z`는 소유 내용을 확인한 뒤 삭제했다.

## 검증 환경

기존 `scripts/python_test_requirements.txt`:

- pydantic 2.11.7.
- typer 0.27.1.

전역 Python 대신 다음 격리 환경을 생성했다.

```text
/tmp/issueops-ten-improvements-01a0fa31/venv
Python 3.12.14
```

고정 dependency 설치와 import/version 확인은 성공했다.
검증 명령에는 다음 PATH를 사용한다.

```sh
PATH=/tmp/issueops-ten-improvements-01a0fa31/venv/bin:$PATH \
  python3 scripts/python_suite_runner.py
```

첫 기준선 monitor `mon_XBTPKCZCQQ2K347N`은 root suite의 기존 개인정보 검사에서
실패했다. 앞선 연구 문서의 로컬 사용자 절대경로가 원인이었다. 테스트를
변경하지 않고, 이번에 만든 연구 문서 17개의 경로 39줄을 `$REPO_ROOT`와
`$HOME`로 익명화하고 출처 색인에 표기 규칙을 명시했다.

재검증 monitor `mon_G5X30VS3R28RVHTF`은 **exit 0**이다.
root 61개(기존 optional local-background 1개 skip), skill suite 6개·파일 10개가
완료됐다. 전체 출력의 suite별 수는 61, 1, 96, 2, 17, 13, 27이다.
이는 production code 변경 전 Python 기준선이며, 변경 후 전체 검증을 대신하지 않는다.

변경 전 `PATH=.../venv/bin:$PATH go test ./... -count=1`도
`mon_KW6EA001E0XXWQM7`에서 **exit 0**이었다. 이 실행은 아래 production 변경
전의 기준선이다.

## I2 첫 구현 증거

공용 HTTP 신원 설계와 독립적인 네 probe duration 손실은 메인이 먼저 수정했다.

- 소유 파일: `internal/adapter/verification/probe/contractauditworker/`의
  `validation_{contract_check,tool_conformance,command_audit,worker_lifecycle}.go`
  및 새 `validation_duration_test.go`.
- RED `mon_VZQ4790D651B9T6H`: 고정값 137, 149, 163, 60 ms를 기대하는 네
  subtest가 모두 실제값 0으로 실패했다.
- 세 단일 명령 probe는 기존 `StepResult.DurationMS`를 보존한다.
  worker lifecycle은 enqueue/status/cancel/list의 측정값을 합산한다.
- 이는 명령 duration의 보존·합산이다. 별도의 probe 전체 wall-clock 측정이라고
  주장하지 않으며 새 clock이나 sleep을 추가하지 않았다.
- GREEN `mon_6X3K5VHQTQR7JJEZ`:
  `go test ./internal/adapter/verification/probe/contractauditworker -count=1 -v`
  **exit 0**. 기존 executable-wrapper·성공·오류 경로와 새 회귀 테스트가 통과했다.
- 해당 패키지 LSP 진단 0건, 변경 파일 `gofmt -l` 출력 없음.

I2의 재사용 표식·표본 수 통계와 최종 통합 검증은 아직 남아 있다.
후속 작업자는 이 다섯 파일의 검증된 변경을 유지하며 이어서 작업한다.

## 첫 구현 DAG

`dag_6e524376-e786-47de-a5c0-09f29acc2c63`을 시작했다.
JSONL 완전성, 재사용 통계, 요청별 Git I/O, 문서 catalog I/O, SDK 호환성의
독립 producer 다섯 개와 모두에 의존하는 실제 검증 node 하나다.
메인은 별도 authority contract/port 기반을 준비한다.
duration 다섯 파일은 B에게 인계했고 검증된 변경을 유지하도록 지시했다.

Fable의 설계 검토 F1-F4는 메인이 현재 소스와 대조해 contract.md에 반영했다.
capability-local VerifiedActor alias를 사용하며, signature 변경 호출자와
branch/delegation·9개 wiring 파일을 G에 명시적으로 배정했다.
stdio와 HTTP의 actor 혼합 규칙 및 grant span·ancestry 저장 규칙도 구분했다.

처음 Fable의 effort export는 medium이었다. 사용자가 high 전환을 선택한 뒤
세션에 연결된 `tool.bash`에서 `MODEL=gpt-6-astra REASONING=high`를 확인했다.
반면 기존 JS kernel의 `env()`는 초기 medium 값을 유지했다. 현재 세션 값은
terminal 도구의 `sessionEnvOverrides`가 `session.thinkingLevel`에서 주입한다.
따라서 후속 작업자는 cached eval env 대신 `tool.bash`의 값을 확인한다.
I7 Sonnet 보고서도 high를 기록했다. 이후 사용자가 effort는 세션 운영 참고일 뿐이라고
명확히 했으므로 별도 검증 게이트에서 제외했다. 제품 코드·설정에는 반영하지 않는다.

## Authority 기반과 SQL 단계

메인이 authority DTO·port, 내부 VerifiedActor 및 capability-local alias를 추가했다.
`mon_Z1TSNJ200HX9NTCW`의 contract/port 테스트는 exit 0이며 JSON으로 credential이나
검증 결과를 입력받지 않는 회귀 테스트가 포함된다. LSP error와 gofmt 출력은 없다.

`dag_293118e6-6609-4076-bd24-5f29f263f84e`는 검증된 RecordReader port를 소비하는
Opus의 I8/WithRecordGuard 구현과 그 뒤 Luna의 실제 검증이다.
sqlstore와 issueopsrecord만 쓰므로 첫 구현 DAG와 파일 소유권이 겹치지 않는다.
메인은 service/install DTO 기반을 별도로 준비한다.

## 완료 판정

작업자 보고는 주장이다. 메인이 실제 diff·테스트·실행 결과를 확인한 뒤만
todo를 완료한다. 구현 producer는 자신의 변경과 회귀 검증을 함께 맡으며,
각 code DAG는 독립 검증 node로 끝낸다.
