# IssueOps 병렬 세션 인계 도그푸딩 B 보고서

## 범위와 실행 identity

- lifecycle: `io-73e95250e1ad`
- issue: `https://github.com/m16khb-org/issueops/issues/510`
- branch: `510-installed-cycle-dogfood-b`
- canonical worktree: `/Users/m16khb/Workspace/issueops.worktrees/510-installed-cycle-dogfood-b`
- 봉인 base 및 초기 HEAD: `58b335defd080dc410cb238fc8e229a7d7d37b4e`
- 봉인 plan SHA-256: `1c5e72a55d46a4854c31f58958d34afdba3b19f7f3c304422565884dd9a9da69`
- 갱신된 handoff material SHA-256: `54cb1011caec4707de20462dce7f9f9703a7c2043444110fe269a2fb1ac30dd1`
- 현재 owner: generation 2, Codex session `01a0bf26-3124-7ad3-ba91-852c646ce0f4`, process PID `49354`, 시작 시각 `2026-09-20T14:09:05Z`

이 사이클의 변경 범위는 이 보고서와 `.issueops/issues/510/gates.md` 두 파일뿐이다. 런타임 코드, source checkout, 다른 lifecycle, 병합, 브랜치·worktree 삭제는 범위에 포함하지 않았다.

## 설치본 identity

갱신된 handoff material과 실제 설치 상태를 다음과 같이 대조했다.

| 항목 | 관측값 |
|---|---|
| source HEAD | `d746ee7c04b58dd2fc2d01661823ae27b73d9195` |
| 설치 바이너리 | `/Users/m16khb/Workspace/issueops/bin/issueops` |
| 설치 바이너리 SHA-256 | `6e5cbb0e2e54631b297d8cb55d5736b1d2f844c7d6531b441f903d084c6c96dd` |
| source cwd의 `inspect.issueops_root` | `/Users/m16khb/Workspace/issueops` |
| 최종 update receipt | `/tmp/issueops-dogfood-fixes/handoff-fix-update.json` |
| update transition | `9d3e29a2be3377f66edfdf5d1e754db0`, `committed=true` |
| host activation | Codex, Claude, Omo, agy 모두 `ok=true` |

연결된 `daemon_status` MCP를 한 번 호출한 결과 `ok=true`, `reachable=true`, `identity_verified=true`였고, `instance.build_sha`는 설치 바이너리 SHA-256과 같았다. PID `88615`, 시작 시각 `2026-09-20T14:30:34Z`, executable도 같은 설치 바이너리였다. 이 MCP 관측은 CLI의 source cwd `inspect` 결과와 별도 증거다.

## 실제 세션 인계

1. 초기 HEAD, plan digest, 갱신 material digest가 봉인값과 일치하는지 확인했다.
2. 기존 DIRECT generation 1이 `released`이고 pending mutation이 없음을 읽었다.
3. 이 세션의 실제 `execution whoami`가 반환한 session과 process receipt를 사용했다. 이전 owner를 사칭하거나 state를 직접 편집하지 않았다.
4. `execution replace --preview`가 반환한 inventory fingerprint `2c5d813370e52e15e2edee7212da937dad76b23e8133122f18b2cfb2e614d212`를 그대로 사용해 reseed했다.
5. 반환된 token-backed claim 명령을 그대로 실행해 generation 2가 `active`이고 holder가 이 세션과 PID를 가리키는지 확인했다.
6. `/tmp/issueops-dogfood-fixes/cycle/active-b.json`에 관측 시각, lifecycle, worktree, generation, host, session과 실제 process receipt만 기록했다. token은 기록하지 않았다.

`execution sync-base --preview`는 main `d746ee7c04b58dd2fc2d01661823ae27b73d9195`와 봉인 base 사이의 merge가 필요하다고 보고했다. merge를 적용하면 source repair 파일이 이 report-only branch의 diff에 포함돼 봉인된 두 파일 범위와 G4를 위반하므로 적용하지 않았다. `base_advanced`는 경고로만 남기고 봉인 base를 유지했다.

## 병렬 owner와 stale-owner 차단

Barrier 증거 `/tmp/issueops-dogfood-fixes/cycle/parallel-observed.json`의 SHA-256은 `515a6057748c9148c82b6f9eeef843ed73263113f0e9fcc0968cee61955529c4`다. 관측 구간은 `2026-09-20T14:35:29.316369Z`부터 `2026-09-20T14:35:39.357669Z`까지였다.

| lane | lifecycle | branch | generation | session | PID |
|---|---|---|---:|---|---:|
| A | `io-75f917ecd7a1` | `509-installed-cycle-dogfood-a` | 2 | `01a0bf27-1dc8-7631-918c-2c619b1cc48b` | 50921 |
| B | `io-73e95250e1ad` | `510-installed-cycle-dogfood-b` | 2 | `01a0bf26-3124-7ad3-ba91-852c646ce0f4` | 49354 |

두 status는 lifecycle, branch, worktree, session, PID가 모두 달랐고 lease가 동시에 `active`였으며 pending mutation은 없었다. sender가 각 OS process를 확인해 `process_verified=true`를 기록했다. 각 former owner의 `execution reconcile --confirm`은 exit 1과 `IssueOps execution mutation requires the current write lease holder`로 거부됐고, 시도 전후 durable record SHA-256이 각 lane에서 동일했다. Receiver는 이 barrier가 유효해진 뒤에만 tracked 문서를 쓰기 시작했다.

## 선행 수리 검증과 이번 cycle의 구분

아래 결과는 설치본의 host·claim 수리를 검증한 선행 증거이며, 이 report-only cycle의 단계 완료를 대신하지 않는다.

- `/tmp/issueops-dogfood-fixes/self-verify-final.json`: 26/26 step 통과, minimum goal score 100, termination eligible. `risk QA tier` 명령은 `go test -race ./... -count=1 && go vet ./...`였다.
- `/tmp/issueops-dogfood-fixes/live-final.json`: 실제 Codex·Claude·Omo 9/9 case 완료, environment/transport failure와 schema drift가 모두 0이었다.
- `/tmp/issueops-dogfood-fixes/update.json`: transition `1500796940cc793d7935a20c189d788e`, committed, 네 host `ok=true`였다.
- `/tmp/issueops-dogfood-fixes/start-fix-update.json`: transition `9ebb694c3d5f47f8b111320d45b35866`, committed, 네 host `ok=true`였다.
- `/tmp/issueops-dogfood-fixes/handoff-fix-self-verify.json`: 26/26 step 통과였다.
- `/tmp/issueops-dogfood-fixes/handoff-fix-update.json`: transition `9d3e29a2be3377f66edfdf5d1e754db0`, committed, 네 host `ok=true`였다.

이번 cycle의 고유 증거는 generation 2 claim, 두 active owner의 실제 동시 관측, former owner 거부와 record 불변, G1-G4 원장, docs-only 검증, 독립 구현 리뷰, draft PR readback, completion receipt다. 이 보고서 작성 시점에는 draft PR 게시와 execution complete를 아직 실행하지 않았으므로 성공했다고 기록하지 않는다. 최종 URL과 completion 상태는 report fingerprint를 바꾸지 않는 `/tmp/issueops-dogfood-fixes/cycle/result-b.json`과 durable lifecycle receipt로만 판정한다.

## 검증 방식과 한계

이 변경은 런타임 동작을 바꾸지 않는 문서 두 파일의 추가다. 새 Go 코드와 새 테스트는 없으며, 전체 race·benchmark를 다시 실행하지 않는다. 이전 설치본 검증은 위 선행 증거로만 인용하고, 이번 cycle에는 다음 report-only 검증을 적용한다.

정리 전 line delta는 2개 문서 101줄, code file은 0개였다. 검증 방식과 수치를 명시한 뒤 정리 후 line delta는 103줄, code file은 0개다. source code 입력이 없으므로 SNR, cyclomatic entropy, duplicate-code ratio, boilerplate-to-logic ratio는 모두 N/A이며 런타임 hot path와 allocation 변화도 0개다.

- G1: 보고서 regular content가 비어 있지 않다.
- G2: 봉인 base 대비 diff에 whitespace 오류가 없다.
- G3: sender가 만든 두-cycle barrier의 owner/process/stale-owner 불변식을 모두 만족한다.
- G4: tracked diff와 untracked 파일의 합집합이 보고서와 gate 원장 두 경로와 정확히 같다.
- project docs 양방향 검토, 독립 적대 리뷰, draft PR 원격 readback과 completion의 final HEAD 일치는 후속 lifecycle 단계가 별도로 소유한다.

첫 `gates init`은 G4 Python 식의 집합 합집합 연산자 `|`를 원장 segment 구분자로 해석해 `created=false`, `gate_count=0`으로 거부했다. 허용 파일의 정확한 집합 비교를 유지하면서 같은 연산을 `set.union(...)`으로 바꿔 한 번 교정했고, 원장 네 개를 생성했다.

## Verified Execution 증거 계약

- Success criteria: G1-G4가 각각 exit 0과 정해진 출력 조건을 만족해야 한다.
- Evidence artifact: `.issueops/issues/510/gates.md`, `/tmp/issueops-dogfood-fixes/cycle/active-b.json`, `/tmp/issueops-dogfood-fixes/cycle/parallel-observed.json`.
- Cleanup receipt: 이 검증이 새 server, container, browser, port, QA 전용 temp directory를 만들지 않았으므로 정리 대상이 없다. canonical worktree와 lifecycle record는 사용자 지시에 따라 보존한다.
- Verification mode: report-only 저위험 변경에 맞춘 proportionate lightweight mode다.
- Skipped checks: 런타임 코드가 없으므로 새 TDD와 전체 benchmark 재실행을 생략했다. 원격 게시와 completion은 이 보고서에서 선행 주장하지 않고 각 단계의 실제 readback으로 검증한다.
