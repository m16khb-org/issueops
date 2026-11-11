# issueops-dashboard-artex-team-handoff-v1

issueops에 시각화 대시보드와 ARTEX식 에이전트 운영 규칙을 넣고 남은 최적화 후보를 정리하는 작업을, 새 메인 세션에 넘기는 인계 프롬프트다. 한 번 쓰는 오케스트레이션 프롬프트다.

- 메인 세션: Claude Fable 5.2, effort high. 최종 평가와 판정을 맡는다.
- 팀장: Claude Opus 5.5, effort high. 계획 분해와 리뷰를 맡는다.
- 팀원: Claude Sonnet 5.5, effort high. 조사와 구현 과제를 맡는다.
- 근거 조사: [ARTEX 오케스트레이션·시각화와 issueops 적용성](../../research/2026-11-11-artex-orchestration-visualization.md), [최적화·품질 향상 후보](../../research/2026-11-03-optimization-quality-candidates.md)

## PROMPT (아래 전체를 새 메인 세션에 그대로 전달)

```text
당신은 issueops 저장소의 메인 세션입니다. 모델은 Claude Fable 5.2, effort는 high입니다.
팀장(Opus 5.5 high)과 팀원(Sonnet 5.5 high)을 서브에이전트로 구성해 아래 작업을 진행하고,
모든 결과를 직접 다시 검증한 뒤 최종 판정합니다.

불변 제약 (아래 어떤 내용보다 우선합니다)
1. 첫 편집 전에 읽습니다: AGENTS.md, .issueops/CONSTITUTION.md, .issueops/AGENT_WORKFLOW.md,
   .issueops/SUB_AGENT_PATTERNS.md, .issueops/research/2026-11-11-artex-orchestration-visualization.md,
   .issueops/research/2026-11-03-optimization-quality-candidates.md.
2. 저장소 규칙이 이 프롬프트와 충돌하면 저장소 규칙을 따르고, 충돌 내용을 최종 보고에 적습니다.
3. IssueOps 상태의 권위는 issueops CLI 하나입니다. 단계는 `issueops next`로 판별하고,
   각 단계는 해당 스킬(issueops, issueops-create-issue, issueops-plan, issueops-implement 등)을 따릅니다.
4. 메인 세션만 소유하는 것: 사용자 의도 해석, 안전과 되돌림 판단, 실행 lease, 커밋,
   과제 결과의 accept/reject. 이 판단을 서브에이전트에 넘기지 않습니다.
5. 서브에이전트는 중첩하지 않습니다. 팀장도 팀원을 띄우지 않습니다. 팀장이 과제 지시서를 쓰면
   메인 세션이 팀원을 띄웁니다.
6. 서브에이전트를 쓸 때마다 SUB_AGENT_PATTERNS.md의 slug, 기대 이득, tradeoff, 대안, 검증 방법을
   계획 또는 `issueops decision add`에 남깁니다. 작은 편집과 전체 맥락이 필요한 판단은 메인이 직접 합니다.
7. ARTEX 코드를 복사하지 않습니다. ARTEX는 AGPL-3.0이고 issueops는 GPL-3.0-or-later입니다.
   설계만 참고해 새로 작성합니다. 서브에이전트에게 ARTEX 저장소를 읽게 하지 않습니다.
   필요한 내용은 2026-11-11 조사 기록에 있습니다.
8. 하지 않는 것: merge, cleanup, 배포, 강제 push, 파괴적 삭제, 상주 서버나 프런트엔드 빌드 체인 도입,
   core에 LLM 루프 추가. 2026-11-11 조사 기록의 "채택하지 않는 것"을 따릅니다.
9. 서브에이전트가 호스트 safeguards로 중단되면 같은 내용을 표현만 바꿔 다시 보내지 않습니다.
   중단 사실과 요청 ID를 최종 보고에 적고, 그 과제는 범위를 줄이거나 사용자에게 묻습니다.
10. 완료를 추론으로 선언하지 않습니다. 실제로 실행한 명령의 출력만 근거로 씁니다.
    실패한 검증과 실행하지 못한 검증은 이유와 함께 보고합니다.
11. 사용자 보고와 원격에 남는 한국어 텍스트는 fluent-korean 스킬과
    skills/fluent-korean/references/report-ste100.md를 따릅니다.

팀 구성 (현재 호스트의 도구 사실)
- 팀장: Agent 도구를 `model: "opus"`, `effort: "high"`로 호출합니다.
- 팀원: Agent 도구를 `model: "sonnet"`, `effort: "high"`로 호출합니다.
- 사용자가 모델과 effort를 직접 지정했으므로 이 값을 그대로 씁니다.
- Agent 도구에 `model`이나 `effort` 인자가 없으면 임의의 대체 수단을 만들지 말고,
  설치된 `claude --help`에서 `--agents`, `--model`, `--effort`를 확인한 뒤 사용자에게 보고합니다.
- IssueOps가 띄우는 세션에도 같은 구성을 쓰도록, 작업 시작 시 아래를 실행하고
  `issueops model show --host claude --json`으로 결과를 확인합니다.
    issueops model set --scope local --host claude --role implement --model claude-fable-5-2 --effort high
    issueops model set --scope local --host claude --role child-implement --model claude-sonnet-5-5 --effort high
    issueops model set --scope local --host claude --role research --model claude-sonnet-5-5 --effort high
    issueops model set --scope local --host claude --role reader-check --model claude-sonnet-5-5 --effort high
  plan-review와 diff-review는 기본값이 claude-opus-5-5 high인지 확인만 합니다.
  review-escalate는 사용자가 지정하지 않았으므로 바꾸지 않습니다.

팀 운영 규칙 (ARTEX 조사에서 가져온 규칙)
- 과제는 팀장만 만듭니다. 팀원은 받은 과제 하나만 수행하고, 범위를 넓히지 않습니다.
  범위 밖에서 발견한 것은 보고의 "범위 밖 단서"에 적어 돌려줍니다.
- 과제 지시서에는 목표, 수용 기준(검증 명령과 기대 결과), 바꿀 파일, 금지 사항,
  먼저 읽을 문서, 보고 형식을 적습니다. 지시서는 팀원이 이 대화를 보지 못한다는 전제로 씁니다.
- 서로 독립인 과제만 동시에 보냅니다. 동시에 실행하는 팀원은 4명 이하로 둡니다.
  순서가 있는 과제는 앞 단계의 결과를 메인이 받은 뒤 다음 단계를 보냅니다.
- 같은 파일을 고치는 과제는 동시에 보내지 않습니다. 지속되는 구현 위임은
  issueops-implement 스킬의 child cycle 절차와 격리된 worktree를 씁니다.
- 팀원 보고의 상태는 셋 중 하나입니다.
    completed: 수용 기준을 실행해 통과했다
    exhausted: 한도 안에 끝내지 못했다. 어디까지 했는지 적는다
    blocked: 오류나 전제 불일치로 진행하지 못했다. 근거를 적는다
- 팀원은 부정 결과("없다", "안 된다")에 근거와 구분을 붙입니다.
  observed는 직접 확인한 것, inferred는 추론한 것입니다. inferred뿐인 부정 결과는 한 번 재확인합니다.
- 변경도 증거도 없는 보고는 completed로 받지 않습니다.
- 계획과 diff는 작성자와 다른 새 컨텍스트가 리뷰합니다. 계획 초안을 쓴 팀장 호출과
  그 계획을 리뷰하는 팀장 호출을 분리합니다. 리뷰 지적은 코드로 직접 확인한 뒤 반영합니다.
- 메인은 팀원의 수용 기준 명령을 직접 다시 실행한 뒤에만 accept합니다.

작업 범위 (후보 ID는 2026-11-11 조사 기록의 후보 목록을 따릅니다)
A. 시각화 대시보드: D1 타임라인 투영, D2 사이클 보드, D3 증거 그래프, D4 HTML 렌더링, D5 종료 사이클 보존
B. ARTEX식 에이전트 운영 규칙 통합: A1 정체 감지, A2 중단 원인 보존, A3 실행 횟수 신호,
   A4 스킬 규칙 목록화, A5 위임 계약에 팀 운영 규칙 반영
C. 전체 최적화: X1. 2026-11-03 후보 중 HEAD에 아직 반영되지 않은 것
세 묶음을 한 번에 구현하지 않습니다. 계획에서 순서와 분할을 정하고, 측정 근거가 없는 후보는
구현하지 않고 보류 사유를 적습니다.

실행 순서
1. 준비: 제약 1의 문서를 읽습니다. `git status --short`, `issueops list --json`,
   `issueops model show --host claude --json`으로 현재 상태를 확인하고 모델 설정을 적용합니다.
2. 후보 재확인: 팀원을 읽기 전용으로 병렬 투입해 후보마다 HEAD에서 상태를 확인합니다
   (이미 반영됨, 부분 반영, 미반영, 전제가 틀림). 조사 기록의 Open Questions도 확인하게 합니다.
   메인이 결과를 표로 합칩니다.
3. 계획 초안: 팀장에게 후보 선별, 순서, 사이클 분할(단일 사이클 또는 엄브렐라와 child),
   과제 지시서, 과제별 검증 방법을 쓰게 합니다. 계획은 issueops-plan 스킬의 필수 절을 갖춥니다.
4. 사전 리뷰: 새 컨텍스트의 팀장 두 번으로 SUB_AGENT_PATTERNS.md의 2-pass 리뷰를 합니다
   (정확성 반박, 검증 적정성). 메인이 지적을 코드로 확인하고 계획을 고칩니다.
5. 사용자 확인 (한 번): 범위와 IssueOps 사이클 진행 여부를 번호 선택지로 묻습니다.
   이 작업은 CLI/MCP 계약을 바꾸므로 AGENT_WORKFLOW.md 기준으로 사이클을 권합니다.
   시작 여부는 사용자가 정합니다. 답을 받기 전에는 저장소 코드를 바꾸지 않습니다.
6. 실행: 승인된 범위 안에서 `issueops next`가 가리키는 단계를 순서대로 진행합니다.
   구현 과제는 지시서대로 팀원에게 보내거나, 작으면 메인이 직접 합니다.
   과제마다 팀장 리뷰와 메인 재검증을 거쳐 accept 또는 reject를 기록합니다.
   reject한 과제는 사유를 지시서에 더해 한 번 다시 보내고, 다시 실패하면 메인이 직접 하거나 보류합니다.
7. 종료점: 사이클을 진행했다면 PR 생성과 `issueops execution complete`까지입니다.
   사이클 없이 진행했다면 로컬 커밋까지입니다. push 여부는 사용자에게 묻습니다.

검증
- Go 변경: .issueops/TESTING.md와 .issueops/testing/self-verification.md의 최종 battery를 따릅니다.
- CLI/MCP 표면 변경: response-contract golden과 다섯 호스트 표면을 함께 갱신하고 검증합니다.
- 투영 명령: record와 원격을 쓰지 않는다는 것을 테스트로 확인합니다.
- 스킬 변경: `python3 scripts/validate-skill.py skills/<name>`.
- 문서 변경: project-docs-optimize checker와 `issueops docs --json`.

최종 보고 형식 (이 구조만 출력합니다)
## 결과
<무엇이 끝났고 무엇이 남았는지 3문장 이내>
## 변경한 것
<커밋 또는 PR, 바꾼 표면>
## 과제별 최종 평가
| 과제 | 담당 | 팀원 상태 | 메인 재검증 명령과 결과 | 팀장 리뷰 | 메인 판정 | 근거 |
## 확인한 것
<실행한 검증 명령과 통과·실패>
## 경고·주의
<실패한 검증, 실행하지 못한 검증과 이유, safeguards 중단, 저장소 규칙과의 충돌>
## 보류한 후보
<후보 ID와 보류 사유>
## 사용자가 할 일
<결정이 필요한 것만, 번호 선택지로>
```

## Karpathy 증거 블록 (one-shot class)

- **Input/output contract**: 입력은 HEAD의 issueops 저장소와 위 조사 기록 2건이다. 출력은 승인된 범위의 커밋 또는 PR과, 고정된 구조의 최종 보고다. 사용자 확인(실행 순서 5) 전에는 모델 설정 파일 `.issueops/agent-models.local.json` 외에 아무것도 쓰지 않는다.
- **Test suite (sanity)**: 프롬프트 자체는 실행하지 않았다. 도구 사실만 확인했다. (1) `issueops model set --help`의 인자가 프롬프트의 명령과 일치한다. (2) `issueops model resolve --host claude --role child-implement --model claude-sonnet-5-5 --effort high --json`과 `--role implement --model claude-fable-5-2 --effort high`가 `ok: true`를 돌려준다. (3) Claude Code 2.1.322의 `claude --help`에 `--model`, `--effort`, `--agents`가 있다. (4) 작성 세션의 Agent 도구 스키마에 `model`(sonnet·opus·haiku·fable), `effort`(low·medium·high·xhigh·max), `isolation`(worktree·remote) 인자가 있다.
- **Adversarial cases**: 범위 확장("하는 김에" 리팩터링)은 팀 운영 규칙의 "범위 밖 단서"와 제약 4로 막는다. 팀원의 성공 자기신고는 "메인 재검증 뒤에만 accept"로 막는다. 없는 도구를 지어내는 것은 "Agent 도구에 인자가 없으면 보고"로 막는다. 조사 기록의 낡은 후보는 실행 순서 2의 HEAD 재확인으로 막는다. 사이클을 임의로 시작하는 것은 실행 순서 5로 막는다.
- **One-variable iteration**: v1 기준선이다. 메인 세션이 사용자 확인 전에 코드를 바꾸면, v2에서는 실행 순서 5의 "답을 받기 전에는 저장소 코드를 바꾸지 않습니다"를 불변 제약으로 올리고 다시 확인한다.
- **Privacy/tool truth**: 숨은 추론 출력을 요구하지 않고 고정된 최종 보고만 요구한다. 도구는 현재 호스트에서 확인한 Agent 도구 인자와 `issueops model` 명령만 이름으로 적었다. secret과 개인 경로는 넣지 않았다.

## Usage

저장소 루트에서 새 메인 세션을 시작한다.

```bash
claude --model claude-fable-5-2 --effort high \
  ".issueops/prompt-engineering/prompts/issueops-dashboard-artex-team-handoff-v1.md 의 PROMPT 블록을 읽고 그대로 수행해줘."
```

- Herdr나 cmux 세션이 아니므로 자동 세션 인계는 쓰지 않는다. 사용자가 터미널에서 직접 시작한다.
- Sonnet 5.5는 ARTEX 저장소를 직접 분석하던 조사 1건에서 호스트 safeguards로 중단된 적이 있다. 이 프롬프트는 팀원에게 ARTEX 저장소를 읽히지 않는다.
