# #550 main CI install 실패와 #548 후속 정비: verified-execution report

- lifecycle: `io-7426b49dc042`, direct execution generation 2
- issue: https://github.com/m16khb-org/issueops/issues/550
- branch: `550-ci-install-and-followups`, base `main` @ `bedad262470fa0f5b2d85fe50fd17875caf1f311`
- 계획: `.issueops/issues/550/plan.md`, 게이트 원장: `.issueops/issues/550/gates.md`, CI 원인 판정: `.issueops/issues/550/ci-diagnosis.md`
- 상태: 구현 완료 초안(4단계). 5단계 정리에서 확정한다.

## 수용 기준별 결과

| 기준 | 결과 | 근거 |
|---|---|---|
| main CI verify job이 install과 self-verify까지 통과 | install 통과 확인, self-verify는 다음 push의 CI로 확인 | run 37596145318에서 install 통과, Python 단계의 rg 부재 실패를 P1-3으로 수정. G11은 원장에서 abandon하고 push 뒤 PR CI run으로 확인 |
| P2 후보마다 수정·유지·분리 결론 | 충족 | 아래 P2 표 |
| 고친 후보마다 재현·측정과 focused 검증 | 충족 | 각 항목의 RED→GREEN, G9 benchmark 전후 동일 |
| 성능 주장은 전후 측정으로만 | 해당 없음 | 성능 개선을 주장하지 않는다 |
| revise·regress 상한 5, 경계 테스트 | 충족 | G14 |
| 리뷰 스킬과 README가 상한 5와 3라운드부터의 effort 상승을 같게 설명 | 충족 | G15 |
| 2026-07-02·2026-09-08 ADR 부분 대체 기록과 색인 표시 | 충족 | G16 |

## 의도 대조

봉인 intent(`.issueops/issues/550/intent.md`)의 성공 기준과 원장·이슈 완료 기준을 대조했다.

| 성공 기준 | 덮는 근거 | 상태 |
|---|---|---|
| main CI verify job이 install·self-verify까지 통과 | G5(CI 단언), G12·G13(진단), 원장 밖 PR CI run | install 통과는 run 37596145318로 확인. self-verify 통과는 원장에서 채울 수 없어 G11을 abandon했고, push 뒤 PR 발행 전에 run URL로 확인한다. 확인 전까지 이 기준은 미충족이며 PR 발행의 선행 조건이다 |
| 후보마다 수정·유지·분리 결론 | P2 표, 계획 P2 표 | 충족 |
| 수정한 후보마다 재현·측정과 focused 검증 | G7·G8·G9·G12·G14·G17, 각 항목의 RED→GREEN | 충족 |

비목표(CLI·MCP 공개 출력 계약 변경, 패키지 병합, CP3·W6)는 건드리지 않았다. 사용자 추가 지시(상한 3→5)는 contract_change로 이슈 본문에 반영했고 G14~G16이 덮는다.

## P1. CI install

- P1-1 진단 커밋 `69ed5d85`: `internal/adapter/mcpservice/supervisor.go`의 `runLoadStep`이 launchd bootstrap과 systemd daemon-reload/enable/start 실패에 실패한 명령과 출력 끝 2048바이트(`policy.TailBytes`, UTF-8 경계)를 붙인다. RED: 에러가 `exit status 1`뿐 → GREEN(G12).
- 이 커밋만 push하고 run 37595246857이 끝날 때까지 다음 push를 하지 않았다. 로그: `systemctl --user enable issueops-mcp.service: exit status 1: Failed to enable unit: Unit file issueops-mcp.service does not exist.` daemon-reload는 성공했으므로 (a) user manager 부재는 반증됐고, 판정은 (b) 임시 HOME unit 경로 불일치다(G13). 후속: HOME을 바꿔 설치하는 Linux 환경 전반의 결함으로 남긴다.
- P1-2 stdio 커밋 `ffefc8f8`(원인 기록 커밋 `fa9026b8` 뒤): CI install에 `--mcp-transport=stdio`, `scripts/ci_workflow_test.py` 단언(RED: 이전 ci.yml에서 실패), `.issueops/operations/install.md` 문단. run 37596145318에서 install 통과.
- P1-3 rg fallback: 그 run에서 install에 가려져 있던 self-verify Python 단계가 처음 돌아 `skills/pr-review/tests/test_context_pack.py`의 `test_rg_fallback_lists_definition_and_callers`가 실패했다. runner에 ripgrep이 없어 `_rg_symbol`이 빈 목록을 돌려준 것이다(rg 없는 PATH로 로컬 재현). `rg`가 `FileNotFoundError`면 `grep -rnwFI`(명시 glob과 `.git`·바이너리 제외)로 찾는다. RED: 새 테스트가 이전 코드와 이전 grep 인자에서 실패 → GREEN. rg 없는 PATH에서 `DefsFallbackTest` 통과(G17).

## P2. 후보 결론

| 후보 | 결론 | 근거와 검증 |
|---|---|---|
| codex hook 이벤트 목록 | 유지, 주석 추가 | 예전 설치본의 issueops hook을 지우는 목록이며 설치는 SessionStart만 한다는 주석. 동작 변경 없음 |
| benchmark 예시 경로 | 수정 | 없는 `internal/core/…`, `cmd/issueops/issueops.go`를 실제 경로로. 18 fixture·342 차원 점수 digest 전후 동일(G9) |
| readability CPU 전용 회귀 | 유지 | 시간 단언은 부하에서 넘쳤다(`.issueops/cautions/2026-10-03-http.md`). `Check`에 이차 경로 없음 |
| readability fixture | 보강 | 반복 문장 끝 줄바꿈으로 `addLineFindings`도 크기에 비례해 실행 |
| 직접 테스트 없는 패키지 | 수정 | `issueopsorphancleanup`(NormalizeRequest, validGitOID, InspectInventory의 ready와 요구 코드 9개), `issueopsbodysync`(CAS 불일치 무쓰기, 확인 쓰기와 baseline 기록, child 검증이 본문 읽기보다 먼저), `nativehost`(host별 executable 판정) (G8) |
| quality inspect 비용 | 유지(방향 기록) | fingerprint 캐시와 증거 재사용 금지가 맞는 설계. 비용이 다시 문제면 실패 패키지만 재실행하는 증분 방식을 검토 |
| Python skip 정책 | 유지 | 문서화된 정책 |
| `os.IsNotExist` | 수정 | production 69곳(36파일)을 `errors.Is(err, fs.ErrNotExist)`로. 호출부 69곳마다 err를 대입한 줄을 확인했다. 66곳은 os.Stat·Lstat·ReadFile·ReadDir·Remove·Chmod·OpenFile을 직접 호출한 결과이고, 3곳은 wrapper 두 개(`readOmoAuthFile`, `readBoundedEvidenceFile`)의 결과지만 두 wrapper 모두 unix·os 에러를 감싸지 않고 그대로 돌려준다. 그래서 wrapped 에러가 들어오는 호출부가 없고 판정이 바뀌는 곳도 없다. 남은 호출 0건은 G7이 확인한다. `sort.Slice`는 유지 |
| 대형 에이전트 문서 | 유지 | ADR·증거 문서가 참조 |
| 원자적 쓰기 통합 | 유지(방향 기록) | capability 간 공유 금지가 의도된 계층 규칙. 공용화는 중립 foundation ADR이 먼저 |
| 분기 많은 함수 | 유지(방향 기록) | dispatcher 성격. 기능 변경으로 손댈 때 함께 나눈다 |

## P3. 리뷰·재계획 상한 3→5 (사용자 추가 지시)

- `reviseRoundCap`, `regressCap`을 5로. 경계 테스트: revise 다섯 번째 허용·여섯 번째 거부, waived 라운드 제외, regress 4회면 허용·5회면 거부, 7회면 실제 횟수 보고. RED: 바꾼 테스트 5개가 상한 3에서 실패 → GREEN(G14).
- `skills/issueops-review/SKILL.md`: 최대 5라운드, 3~5라운드는 한 단계 높은 effort, 비-waived revise 다섯 번까지·여섯 번째 거부, regress 다섯 번까지. `README.en.md` 같은 문장(G15).
- 결정 기록 `.issueops/adr/2026-10-07-review-revise-and-regress-rounds-are-capped-at-five.md`(`project_docs_append`), ADR 색인의 새 행과 두 기존 행 "superseded in part"(`project_docs_revise`)(G16).
- contract_change feedback 기록 → #550 본문 동기화 → mark-issue-updated.

## 계획 리뷰

- 4라운드 revise(G14 공회전, 2026-09-08 ADR·README.en.md 누락), 5라운드 pass, 6라운드 revise(grep fallback의 `.git`·바이너리 잡음), 7라운드 pass(BSD grep 2.6.0과 GNU grep 3.6에서 제외 규칙 실측). 4~7라운드는 claude-opus-5-5, effort xhigh.
- 6라운드 revise는 설치된 main 빌드의 상한이 아직 3이라 네 번째 비-waived revise로 거부돼 근거를 적어 waive로 기록했다. 지적은 고쳤다.

## ai-slop-clean

- 범위: 이번 diff의 파일과 직접 관련된 파일.
- 제거(weak-artifact): `loadOutputLimit` 위의 이름을 되풀이하는 주석 한 줄.
- 이름 정리: `skills/issueops-review/SKILL.md`의 `$ROUND3_EFFORT`를 `$ESCALATED_EFFORT`로. 이제 3~5라운드에 같은 값을 쓴다.
- 주장 정리: report의 `os.IsNotExist` 판정 불변 근거를 호출부 분류(직접 호출 66곳, wrapper 결과 3곳)로 구체화했고, `ci-diagnosis.md`의 후속 범위를 "이미 실행 중인 user manager와 다른 HOME으로 설치하는 Linux 환경"으로 좁혔다.
- 유지: 추가한 주석(CI가 stdio를 고르는 이유, codex hook 목록의 용도, `runLoadStep`이 출력을 붙이는 이유, readability fixture의 줄바꿈 이유, grep fallback의 제외 규칙)은 코드만으로 드러나지 않는 이유다. 테스트 fake는 다른 패키지의 비공개 fake를 공유할 수 없어 패키지마다 둔다.
- 측정(코드 파일 `*.go *.py *.sh *.yml`의 base 대비 추가 줄과 untracked 파일, 주석·print 줄을 잡음으로 분류): 정리 전 SNR 0.959(signal 512, noise 22, total 534), 정리 후 0.961(signal 512, noise 21, total 533).
- 범위 밖 발견: 없음.

## 성능 측정

- hot path 변경 없음. `errors.Is`는 wrap chain을 한 번 순회한다(측정하지 않음, 성능 주장도 하지 않는다).
- benchmark artifact 문구 변경 전후: 18 fixture, 342 차원, 점수 digest `7405bbec3a6764a7`, 평균·최소 100으로 같다(G9).
- grep fallback(7라운드 리뷰어 실측): 이 저장소에서 기호 하나에 캐시가 데워진 상태로 약 1~3초, 차가울 때 약 9초. 기호당 30초 timeout이 상한이고 rg가 없을 때만 돈다.

## 프로젝트 문서 반영

- 4단계: ADR 2026-10-07(상한 5)과 `.issueops/ADR.md` 색인(새 행, 두 기존 행 "superseded in part"), `.issueops/operations/install.md`(HOME을 바꾼 Linux 설치는 stdio 명시).
- 6단계: caution `.issueops/cautions/2026-10-07-ci-home-runner-rg-worktree-lint-cache-binary.md`와 CAUTIONS 색인 행, `.issueops/conventions/go-and-packages.md`(`errors.Is(err, fs.ErrNotExist)` 관용구), `.issueops/testing/self-verification.md`(CI 임시 HOME 설치의 stdio 선택). 모두 `project_docs_append`/`project_docs_revise`로 쓰고 결과를 기대 파일과 diff로 대조했다.
- 문서 → 구현 대조: ADR 2026-10-02(stdio 자동 전환 금지)는 CI가 명시적으로 고르는 방식으로 지켰고, AGENTS.md의 독립 실행 철학은 CI에 rg를 설치하지 않고 스크립트 fallback을 둔 것으로 지켰다.
- docs checker: `.issueops/adr/roadmap.md` 줄 수 초과(253줄) 1건. base에서도 같고 이번 diff가 손대지 않은 기존 부채다.

## 계약 표면과 side effect

- CLI·MCP JSON 출력, record schema, Linux 기본 transport: 변경 없음.
- 변경: supervisor load 에러 문구(비교·파싱하는 곳 없음), CI workflow의 install transport, revise·regress 상한, pr-review의 rg 부재 시 동작, 문서·ADR.
- 원격: issue branch push 3회(진단, 원인 기록+stdio), #550 본문 동기화 1회.
- durable state: contract_change feedback, compatibility review, devils-advocate 4~6라운드 기록, 게이트 원장.

## 검증

- 게이트 원장 G1~G17의 실행 결과는 `.issueops/issues/550/gates.md`의 EVIDENCE를 따른다. 4단계 끝의 1차 실행(2026-10-07): 15개 충족. G10은 로컬 python3에 pydantic·typer가 없어 slack-delegate 스위트에서 실패했고, `scripts/python_test_requirements.txt`를 설치한 uv venv를 PATH 앞에 두고 다시 돌린 self-verify는 27단계 모두 통과했다. G11은 7단계에서 원장 게이트로는 채울 수 없다고 판단해 abandon했다. HEAD의 CI run은 원장을 커밋해 push해야 생기고, 그 결과를 EVIDENCE로 쓰면 gates.md와 HEAD가 다시 바뀐다. CI verify job의 install·self-verify 통과는 8단계 push 뒤 PR 발행 전에 run URL로 확인하고 PR 본문과 완료 기록에 남긴다.
- 같은 날 golangci-lint 공유 cache가 지워진 #548 worktree 경로의 결과를 돌려줘 `cache clean` 뒤 다시 실행했다(0건).

## 남은 일

- HOME을 바꿔 설치하는 Linux 환경에서 HTTP MCP supervisor가 unit을 찾지 못하는 결함(판정 (b)): 후속 이슈 후보.
- self-verify의 Python 뒤 단계는 이번 PR CI에서 처음 Linux·임시 HOME으로 돈다. 새 실패가 나오면 원인을 확인한 뒤 고친다.
