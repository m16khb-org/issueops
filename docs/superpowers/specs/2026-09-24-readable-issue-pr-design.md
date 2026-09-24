# 이슈·PR을 사람이 읽는 문서로 만드는 설계

- 날짜: 2026-09-24
- 상태: proposed
- 범위: `issueops remote`의 게시·동기화 명령, `internal/domain/artifacttemplate`, 새 패키지 `internal/domain/artifactreadability`, 관리 구간 렌더러 `internal/adapter/provider/issuebody`, 완료·정리 경로, 게시 관련 스킬, `.github`·`.gitlab` 템플릿

## 요약

이슈와 PR은 팀원과 리뷰어가 읽는 문서다. 그런데 지금 하네스가 게시하는 본문에는 세 종류의 내용이 섞여 있다. 사람을 위한 설명, 다음 에이전트 세션을 위한 지시, 감사용 기록이다. 가독성을 지키는 검사는 형식만 보거나 건너뛸 수 있다.

이 설계는 세 가지를 바꾼다.

- 본문은 요약이 먼저 오는 짧은 계약을 따르고, 사람에게 필요한 내용만 담는다.
- 해시, 커밋 SHA, plan 원문 같은 구현 자료는 `.issueops/issues/<n>/`와 record로 옮긴다.
- 가독성 검사를 `issueops remote`의 게시·동기화 명령 안에서 실행해 건너뛸 수 없게 한다.

## 원칙

하네스는 사람이 읽기 쉽게 정리된 이슈와 PR을 만들고, 구현에 쓰는 자료는 `.issueops/` 아래에 둔다. 2026-09-24 사용자가 결정한 원칙이다.

- 본문에는 사람이 전체 흐름을 파악하고 이해하는 데 필요한 내용만 둔다.
- 해시, 커밋 SHA, 라벨 점수, 정리 감사, 에이전트 사이의 지시, plan·spec 원문은 본문에 넣지 않는다.
- 본문에 남는 하네스 흔적은 화면에 보이지 않는 HTML 주석 표지뿐이다. 생성 표지는 결과가 불명확할 때 중복 생성을 막는다. 구간 경계 표지는 정리 단계가 완료 반영 여부를 확인하는 데 쓴다.
- 이 규칙은 에이전트가 쓰는 부분과 하네스가 자동으로 붙이는 부분에 똑같이 적용한다.

## 문제

### 조사 범위

2026-09-23에 세 가지를 조사했다.

- 코드: 템플릿 렌더러와 검증, 게시·동기화 명령, 관리 구간 렌더러, 게시 관련 스킬
- 게시된 본문:
  - 이 저장소의 이슈 40건과 PR 40건. 지표는 전수로 계산했고, 이슈 10건과 PR 9건은 끝까지 읽었다.
  - 사내 GitLab 저장소의 이슈 40건과 MR 40건. 지표는 전수로 계산했다. 비공개 저장소이므로 이 문서에는 집계 수치만 쓴다.
- 외부 자료: 작성 가이드, 설계 문서 템플릿, 모범 사례, AI 작성물에 대한 유지보수자 정책, 실증 연구

### 관찰

**템플릿이 절 수를 고정한다.**

- PR은 필수 절이 13개다(`internal/domain/artifacttemplate/template.go:343`). 현재 템플릿으로 게시된 PR/MR 8건은 모두 정확히 13절이었다.
- 사내 저장소에서 팀원이 쓴 이슈는 중앙값이 8절·4,015자였다. issueops가 쓴 이슈는 13절·8,358자였다.

**완료 구간이 본문의 대부분을 차지한다.**

- GH#446, #449, #507, #477에서 완료 구간은 본문의 44~95%였다.
- 이 구간에는 plan·spec 전문, SHA-256 목록, 최종 커밋 SHA, verified-execution 요약, 정리 감사가 들어간다(`internal/adapter/provider/issuebody/issue_body_section.go:66`). 정리 감사에는 로컬 절대 경로가 포함된다.
- 내용이 없어도 소제목 7개를 "(없음)"으로 출력한다.

**하네스 데이터와 용어가 본문에 섞인다.**

- 모든 이슈에 라벨 점수 절이 들어간다. 원인은 `internal/domain/artifacttemplate/template.go:322`의 기본 문구다.
- 이 저장소의 본문 80건에서 execution이 287회, lease가 213회, 봉인이 205회, generation이 197회 나왔다. 명령어 속 단어까지 센 상한값이다.
- 리뷰 에이전트 이름(brooks, turing, shannon)도 설명 없이 나온다.

**본문이 결정 변경을 따라가지 못한다.**

- GH#490은 본문과 첨부된 plan이 서로 다른 설계를 말한다.
- 계획 검토에서 정정한 사실이 본문에 반영되지 않았다.

**검사가 형식만 보고, 건너뛸 수도 있다.**

- `Validate`는 필수 제목이 있는지와 한글이 한 글자 이상 있는지만 확인한다(`internal/domain/artifacttemplate/template.go:101`).
- `--template`을 빼면 검증하지 않는다(`cmd/issueops/issueopscli/remotecmd/remote_child_pr.go:287`). `sync-issue`와 `sync-pr`은 아예 검증하지 않는다(`cmd/issueops/issueopscli/remotecmd/remote_body_sync.go:59`).
- 한글 비율 검사는 스킬에 든 Python 스크립트라서, 에이전트가 기억해야 실행된다.
- CAUTIONS에는 이 검사가 `issueops remote` 명령 안에서 실행된다고 적혀 있지만(`.issueops/cautions/issueops-lifecycle.md:60`), 구현은 그렇지 않다.
- 커밋 `c9085123`이 기록한 사고에서는 본문을 provider MCP 도구로 직접 게시해 모든 검사를 건너뛰었다.

**설명 원칙이 공개 문서 작성에 연결돼 있지 않다.**

- 라우터는 `explain`을 대화로 보고할 때만 쓰도록 연결한다(`skills/issueops/SKILL.md:79`).
- 게시 스킬 세 개는 `explain`을 참조하지 않는다.

**본문 형태를 정의하는 곳이 흩어져 있다.**

- Go 렌더러, 저장소 템플릿 7개, 스킬 4개가 각자 절 구성을 정의하고, 서로 어긋난다.
- 예를 들어 사람용 PR 템플릿에는 3줄 요약이 있지만, 에이전트가 쓰는 경로에는 없다.

이 저장소에서 가장 읽기 좋은 PR은 13절 템플릿이 표준 게시 경로가 되기 전(2026-08-30 이전)에 자유 형식으로 쓴 것들이다.

- PR #495는 첫 문장이 요약이고, 절 제목이 독자의 질문을 따른다: "원인", "고친 것", "같은 실패가 반복되지 않게", "남는 위험".
- PR #489는 요약, 결함, 변경, 검증, 호환성 5개 절로 끝난다.
- 옛 표지가 붙은 PR 23건 중 21건이 3~6절이었다.

### 원인

1. 본문 하나가 세 독자를 동시에 상대한다. 팀원, 다음 에이전트 세션, 감사 기록이다.
2. 템플릿이 읽는 순서가 아니라 누락을 막는 목록으로 설계됐다. 2026-06-23 템플릿 조사(`docs/superpowers/research/2026-06-23-issueops-template-research.md`)는 필요한 정보의 범위를 기준으로 절을 골랐다.
3. 가독성 검사가 형식만 보고, 게시 경로 밖에 있어서 건너뛸 수 있다.
4. 하네스 용어를 독자의 말로 바꾸는 단계가 없다.
5. 계획 검토에서 나온 정정을 본문 갱신으로 이어 주는 규칙이 없다.

### 외부 자료가 가리키는 방향

- **첫 문단에서 무엇을 왜 하는지 답한다.** Google CL 설명 가이드는 첫 줄이 단독으로 읽혀야 한다고 한다. Go 제안 #60078은 첫 문장이 곧 결정이다.
- **해결책보다 문제를 먼저, 구체적인 장면으로 보여 준다.** Shape Up과 리눅스 커널의 submitting-patches 문서가 이렇게 권한다.
- **선택 이유와 대안을 적는다.** Google, Rust RFC, PEP의 Rejected Ideas가 이를 요구한다.
- **관찰한 사실과 추정을 나누고, 확인하지 못한 것을 밝힌다.** Mozilla 버그 작성 가이드와 커널의 AI 도구 문서가 이렇게 요구한다.
- **해당 없는 절은 뺀다.** Go 제안 템플릿, KEP, Linear가 그렇게 한다.
- **원하는 피드백을 밝힌다.** GitHub 블로그와 Kubernetes PR 템플릿이 이를 권한다.
- **결정이 바뀌면 설명을 고친다.** GitLab 핸드북이 이를 요구한다.
  - 연구 결과도 있다. 설명과 코드가 어긋난 에이전트 PR은 수락률이 28.3%로, 나머지 PR(80.0%)보다 낮았다(Gong et al., MSR'26, arXiv:2601.04886).
  - 다만 관찰 연구라서 인과관계를 증명하지는 않는다.

## 목표와 성공 기준

- 처음 보는 팀원이 `## 요약`만 읽고 세 가지에 답할 수 있다. 무엇이 왜 바뀌는지, 끝나면 무엇이 달라지는지, 어떻게 확인하는지다.
- 본문의 코드 밖에는 해시, 커밋 SHA 전문, 로컬 절대 경로, 하네스 용어가 없다.
- `issueops remote`로 게시하거나 동기화하는 본문은 가독성 검사를 건너뛸 수 없다.
- 새 형식으로 게시한 본문은 다음 기준을 만족한다.
  - 첫 절이 요약인 비율이 100%다.
  - 코드 밖의 64자리 hex가 0건이다.
  - 필수 절은 PR 4개, 구현 이슈 5개다.
- 글자 수와 하네스 용어 횟수는 이번 조사와 같은 방법으로 재서 기준선과 비교한다.

## 결정

### 1. 책임 배치

| 역할 | 소유자 | 내용 |
|---|---|---|
| 본문 계약 | `internal/domain/artifacttemplate` | 종류별 필수 절과 선택 절, 요약 규칙, 필드 별칭을 정의하는 유일한 원본이다. 스킬은 절 목록을 복사하지 않고, `issueops remote render-template`이 출력한 골격을 받아 쓴다. |
| 가독성 검사 | 새 패키지 `internal/domain/artifactreadability` | 제목과 본문을 받아 critical 목록과 warning 목록을 돌려주는 순수 함수다. |
| 집행 | `issueops remote`의 `create-issue`, `create-child`, `create-pr`, `sync-issue`, `sync-pr`, `reflect-completion` | 계약 검사와 가독성 검사를 항상 실행한다. preview는 결과를 보여 주고, `--confirm`은 critical이 있으면 거부한다. |
| 관리 구간 | `internal/adapter/provider/issuebody` | 진행 결과 구간과 계획 검토 구간을 사람이 읽는 형태로 렌더한다. 경계 표지는 유지한다. |
| 작성 지침 | `skills/issueops-remote-write/references/readable-body.md` | 구조(explain), 문장(fluent-korean), 용어 변환표, 공개 모범 사례, 독자 검토 절차를 한 문서에 둔다. |
| 결정 변경 반영 | 기존 feedback 장치 | 본문의 사실을 바꾸는 검토 결과를 `contract_change`로 기록한다. 그러면 기존 게이트가 본문 갱신을 강제한다. |

다음은 바꾸지 않는다.

- provider 어댑터 API
- 본문 표지의 형식
- lease·generation·CAS 절차
- "원격 이슈 본문이 범위의 원본"이라는 규칙
- record 스키마

### 2. 본문 계약

#### 공통 규칙

1. **첫 절은 `## 요약`이다.** 2~3문장으로 쓴다.
   - 이슈의 요약은 무엇이 문제인지, 왜 지금 고치는지, 끝나면 무엇이 달라지는지에 답한다.
   - PR의 요약은 무엇을 왜 바꿨는지, 그 결과 무엇이 달라지는지에 답한다.
2. **필수 절은 종류마다 4~5개다.** 선택 절은 쓸 내용이 없으면 만들지 않는다. "없음"만 적은 절도 두지 않는다.
3. **필수 절은 아래 표의 순서대로 둔다.** `render-template`이 출력하는 골격도 이 순서를 따른다. 필수 절 사이에는 작성자가 절을 자유롭게 추가할 수 있다. 검사와 동기화는 필수 절의 제목만 기준으로 삼는다.
4. **사실과 추정을 나눈다.** 원인이 추정이면 추정이라고 밝힌다. PR에는 확인하지 못한 것을 따로 적는다.
5. **하네스 기록은 본문에 쓰지 않는다.** 원칙 절을 따른다.

#### 종류별 절

| 종류 | 필수 절(순서대로) | 선택 절 |
|---|---|---|
| 구현 작업(`implementation_task`), 기능(`feature`) | 요약, 배경, 완료 기준, 범위, 검증 | 접근 방법과 대안, 위험, 열린 질문, 하위 Task |
| 버그(`bug`) | 요약, 재현 절차, 기대 동작과 실제 동작, 완료 기준, 검증 | 원인, 범위, 환경과 로그 |
| 제안(`proposal`) | 요약, 배경, 제안, 대안과 선택 이유, 범위 | 완료 기준, 위험, 열린 질문 |
| 하위 작업(`child_task`) | 요약, 완료 기준, 범위, 선행 조건과 병합 조건 | 검증 |
| PR/MR(`pull_request`) | 요약, 변경 내용, 확인한 것, 리뷰 포인트 | 대안과 선택 이유, 위험과 되돌리기, 호환성과 마이그레이션, 남은 일 |

#### 절별 내용

- **요약(PR)**: 마지막 줄에 `Closes #n`을 둔다. PR과 이슈의 연결을 본문 문자열로 검사하는 코드는 없으므로, 이 줄을 요약으로 옮겨도 깨지는 곳이 없다.
- **요약(하위 작업)**: 부모 이슈 링크와 이 작업이 맡는 부분을 쓴다.
- **배경**: 누가 어떤 상황에서 무엇을 겪는지 쓴다. 입력과 출력, 수치, 로그 한 줄 같은 구체적인 장면을 하나 넣는다. 현재 코드 근거는 그 뒤에 짧게 붙인다.
- **범위**: 하는 것과 하지 않는 것을 함께 적는다. 기존의 비목표 절과 구현 범위 절을 합친 것이다.
- **확인한 것**: 확인한 동작과 확인 방법, 결과를 쓴다. 확인하지 못한 것은 따로 적는다. 결과를 "pass"로만 쓰지 않는다.
- **리뷰 포인트**: 판단이 필요한 곳과 원하는 피드백의 종류를 쓴다.
- **검증(이슈)**: 제목을 `## 검증`으로 유지한다. 구현 세션에 넘기는 컨텍스트가 이 절의 첫 코드 블록에서 검증 명령을 꺼내기 때문이다(`internal/adapter/issueops/execution_owner_context.go:587`).

#### 호환성

- 기존 `--field` 키는 새 절의 별칭으로 받는다.

  | 기존 키 | 새 절 |
  |---|---|
  | `problem`, `current_evidence` | 배경 |
  | `non_goals`, `implementation_scope` | 범위 |
  | `intent` | 요약 |
  | `reviewer_focus` | 리뷰 포인트 |
  | `risk_rollback` | 위험과 되돌리기 |
  | `breaking_changes`, `docs_migration`, `user_impact` | 호환성과 마이그레이션 |

- 사람이 읽지 않는 필드도 받기는 한다. 해당 필드는 `worktree_cleanup`, `scope_management`, `change_type`, `automation_evidence`, `feedback_log`다. 다만 본문에 렌더하지 않고, preview에서 warning으로 알린다.
- 라벨 점수 절은 없앤다.
  - 선택하거나 거절한 라벨과 threshold는 `issueops decision add --kind review --title "라벨 판단"`으로 record에 남긴다.
  - 이슈를 만드는 시점에는 워크트리가 아직 없으므로 `.issueops/issues/<n>/`에 파일로 둘 수 없다.
  - 라벨 점수를 본문에 요구하는 Go 게이트는 없다.
- 필수 절 수는 PR이 13개에서 4개로, 구현 이슈가 8개에서 5개로 줄어든다.

#### 예시

GH#510의 앞부분을 새 계약에 맞춰 다시 쓰면 다음과 같다. 원래 본문에 적힌 사실만 썼다.

```markdown
## 요약
최근 고친 실행 증거 처리와 문서 라우팅이 실제 사이클에서도 끝까지 이어지는지
아직 확인하지 못했습니다. 갱신한 설치본으로 이슈 생성부터 완료 기록까지 한
사이클을 실제로 돌리고, 결과를 검증 보고서로 남깁니다.

## 배경
설치본의 명령과 MCP는 각각 정상으로 확인했습니다. 하지만 실제 사이클에서 단계
전환과 구현 세션 인계가 끝까지 이어지는지는 따로 확인해야 합니다.

## 완료 기준
- [ ] 작업 공간을 담당하는 세션은 하나뿐이고, 작업을 넘긴 세션은 그 뒤로 구현하지 않는다.
- [ ] draft PR과 완료 기록이 같은 사이클과 같은 최종 커밋을 가리킨다.
- [ ] 보고서가 실제로 관찰한 것과 확인하지 못한 것을 나눠 적는다.

## 범위
- 하는 것: 검증 보고서 작성, 인계와 완료 기록 확인
- 하지 않는 것: 새 런타임 기능, PR 병합, 브랜치·워크트리 삭제
```

원래 본문의 "문제" 절에는 문제가 아니라 계획이 적혀 있었다. 새 계약에서는 이 내용이 요약과 배경으로 나뉜다. "canonical worktree", "active owner", "release"는 독자의 말로 바꿨다.

PR #512(13절, 2,164자)를 새 계약에 맞춰 다시 쓰면 다음과 같다.

```markdown
## 요약
두 이슈를 서로 다른 Codex 세션에 동시에 맡겨도 서로 간섭하지 않는지 실제로 돌려
확인했고, 결과를 보고서로 남깁니다. 런타임 코드는 바꾸지 않았습니다.
Closes #510

## 변경 내용
- 병렬 실행을 관찰한 보고서 `dogfood-report.md`를 추가했습니다.
- 검증 항목 4개와 결과를 기록한 `gates.md`를 추가했습니다.

## 확인한 것
- 두 사이클이 서로 다른 프로세스에서 동시에 활성 상태를 유지했습니다(실제 실행).
- 작업을 넘긴 세션의 요청은 거부됐고, 기록은 바뀌지 않았습니다.
- 설치본과 MCP 서버의 빌드 정보를 보고서에 기록했습니다.

## 리뷰 포인트
- 보고서의 판정이 관측 기록과 맞는지 봐 주세요. 바뀐 파일은 위 두 개입니다.
```

### 3. 하네스가 붙이는 구간

#### 진행 결과(완료 구간)

- `issueops remote reflect-completion`에 `--body-file`을 추가한다.
  - 에이전트는 사이클을 마칠 때 사람이 읽을 진행 결과를 파일로 쓴다.
  - 명령은 이 글이 가독성 검사를 통과해야만 완료 구간에 넣는다.
- 진행 결과는 결과 1~2문장과 흐름 3~6줄로 쓴다.
  - 흐름은 계획, 검토에서 바뀐 점, 구현, 확인, 남은 일 순서다.
  - 구현 줄에 PR 링크를 넣는다.
- 완료 구간에서 다음을 뺀다: 최종 커밋 SHA, 아티팩트 해시 목록, verified-execution 요약, plan·spec 전문, 빈 소제목. 이 값들은 record와 `.issueops/issues/<n>/`에 이미 있다.
- 시작 표지와 끝 표지는 유지한다. 정리 단계는 시작 표지만 확인한다(`cmd/issueops/issueopscli/feedbackcleanup/feedback_cleanup.go:253`). 소제목을 읽는 코드는 없다.
- 진행 결과 원고는 따로 보관하지 않는다.
  - 진행 결과는 사람이 읽는 내용이므로 이슈 본문이 원본이다.
  - record에 필드로 저장하지 않는다. record 디코더는 모르는 필드를 거부한다(`internal/adapter/outbound/issueopsrecord/codec.go:28`). 필드를 추가하면 이전 빌드가 새 record를 읽지 못한다.
  - 워크트리에 파일로 두지도 않는다. PR을 만든 뒤에 쓴 파일은 커밋되지 않은 채 남아 정리 단계를 막는다. 그래서 원고는 임시 파일로 넘긴다.
  - 정리 단계는 완료 구간을 다시 쓰지 않으므로 원고를 다시 읽을 일이 없다.
  - 정리 단계가 안내하는 반영 명령(`internal/adapter/issueops/issueops_cleanup_finish.go:47`)에도 `--body-file`을 넣는다.
  - `reflect-completion`은 `--confirm`과 함께 `--body-file`을 반드시 받는다.

GH#510이 끝났다면 본문 끝에 다음 구간이 붙는다.

```markdown
## 진행 결과
두 이슈를 서로 다른 Codex 세션에서 동시에 진행해도 서로 간섭하지 않는다는 것을
실제 실행으로 확인했습니다. 자세한 관찰 내용은 PR #512의 보고서에 있습니다.

- 계획: 갱신한 설치본으로 이슈 생성부터 완료 기록까지 한 사이클을 실행한다.
- 구현: 새 Codex 세션이 작업을 넘겨받아 보고서와 검증 기록을 작성했다(PR #512).
- 확인: 두 사이클이 동시에 진행됐고, 작업을 넘기기 전 세션의 요청은 거부됐다.
```

#### 정리 감사

- `cleanup finish`는 정리 감사를 본문에 쓰지 않고 record에만 남긴다.
- 지금은 `ReflectCleanupAudit`(`internal/adapter/issueops/issueops_cleanup_finish.go:507`)가 감사 줄을 붙이면서 완료 구간 전체를 다시 쓴다.
- 이 함수는 원격 쓰기가 끝난 뒤 반영 캐시를 채운다. 원격 쓰기를 없앨 때, `issueops list`의 반영 상태가 달라지지 않는다는 것을 테스트로 고정한다.

#### 계획 검토 구간

- `reflect-devils-advocate`는 지적 원문 대신 흐름 한 줄을 쓴다. 예: "계획 검토: 1차 수정 요청(지적 3건) → 계획 수정 → 2차 통과".
- 판정은 영어 값 대신 "통과", "수정 요청", "중단"으로 쓴다.
- 판정이 중단이면 그 이유를 짧은 목록으로 덧붙인다.
- 지적 원문은 record의 `DevilsAdvocateReview.Findings`와 `History`에 이미 있다.
  - 계획 검토를 하는 시점에는 워크트리가 없다.
  - 그래서 `execution prepare`가 record를 바탕으로 `intent.md`를 만드는 것과 같은 방식으로 `plan-review.md`를 만든다. 이 파일은 봉인 artifact 디렉터리에 들어간다.
- 명령과 표지는 유지한다. `regress`는 중단 판정이 이슈에 반영됐는지(`IssueReflectedAt`)를 전제 조건으로 확인하기 때문이다(`internal/adapter/issueops/issueops_regress.go:76`).

#### 관련 이슈 댓글

- `remote sync-graph`가 쓰는 댓글을 한국어로 바꾼다(`internal/adapter/issueops/issueops_remote_sync.go:35`). 제목은 "## 관련 이슈"로 한다.
- 사이클 ID 줄은 지운다.

### 4. 구현 자료의 위치

| 위치 | 담는 것 | 독자 |
|---|---|---|
| 이슈 본문 | 요약, 배경, 완료 기준, 범위, 검증, 사이클이 끝난 뒤의 진행 결과 | 팀원, 나중에 찾아보는 사람 |
| PR 본문 | 요약, 변경 내용, 확인한 것, 리뷰 포인트, 필요한 선택 절 | 리뷰어 |
| `.issueops/issues/<n>/` | plan, spec, intent, 계획 검토 지적(`plan-review.md`), gates, 증거 로그 | 구현하는 에이전트, 깊이 확인하려는 리뷰어 |
| record(로컬 state) | 해시, 커밋 SHA, lease·generation, 정리 감사, 검토 판정 이력, 라벨 판단 | 하네스 |

- **지금의 봉인 artifact 처리**
  - 봉인 artifact(plan, spec, intent, verified-execution)는 `.issueops/issues/<n>/artifact/`에 생긴다.
  - 이 디렉터리 안의 `.gitignore`(`*`) 때문에 커밋되지 않는다(`ensureExecutionOwnerArtifactIgnore`, `internal/adapter/issueops/execution_owner_context.go:558`).
  - 그래서 워크트리를 지우면 사라진다. 지금 완료 구간이 plan 원문을 이슈 본문에 복사해 보존하는 이유가 이것이다.
- **바꾸는 방식**: `.issueops/issues/<n>/` 전체를 PR 커밋에 포함한다. 이 저장소의 이전 사이클과 사내 저장소의 최근 사이클은 이미 plan과 증거 로그를 이 경로에 커밋했다.
- **`.gitignore`(`*`)의 원래 목적**
  - 2026-09-01 커밋 `746272d8`이 넣었다. 커밋되지 않은 artifact 때문에 `worktree_clean` 검사와 변경 fingerprint가 실패하는 문제를 막으려는 것이었다.
  - 이 규칙을 없애면 artifact가 커밋 대상이 된다. 따라서 8단계(커밋·푸시)가 이 경로를 함께 커밋해야 하고, 범위 검사 게이트가 이 경로를 허용해야 한다.
- **기존 ADR과의 관계**
  - 2026-09-09 ADR은 "추적되는 INTENT.md"안을 기각했다. 두 사본이 어긋날 수 있고, PR diff가 복잡해진다는 이유였다. 대신 "리뷰어 가시성이 필요해지면 다시 본다"고 적었다.
  - 이 설계가 그 조건에 해당한다. 따라서 새 ADR로 결정을 바꾼다.

### 5. 가독성 검사

#### 집행 위치

| 명령 | preview | `--confirm` |
|---|---|---|
| `create-issue`, `create-child`, `create-pr` | 계약 검사와 가독성 검사의 결과를 보여 준다 | 같은 검사를 다시 실행하고, critical이 있으면 거부한다. `--template`은 반드시 지정해야 한다. |
| `sync-issue`, `sync-pr` | 제안 본문을 검사한다. 원격의 현재 본문도 검사해서 warning으로만 보여 준다 | 제안 본문에 critical이 있으면 거부한다 |
| `reflect-completion` | 진행 결과 원고를 검사한다 | critical이 있으면 거부한다 |

- **sync의 템플릿 추론**: `sync-*`에서 `--template`을 빼면 제안 본문의 제목으로 템플릿을 추론한다. 옛 13절 본문을 새 형식으로 바꾸는 동기화에서는 원격에 있는 옛 제목이 새 계약과 맞지 않기 때문이다.
  - `## 재현 절차`가 있으면 버그로 본다.
  - `## 제안`이 있으면 제안으로 본다.
  - `--url`이 하위 작업을 가리키면 하위 작업으로 본다.
  - PR 명령이면 PR로 본다.
  - 그 밖에는 구현 작업으로 본다. 기능은 구현 작업과 필수 절이 같다.
  - record 디코더 제약 때문에 템플릿 종류를 record에 저장하지 않는다.
- **원격 본문 검사의 목적**: 원격의 현재 본문을 검사하면, `issueops remote` 밖에서 게시된 본문을 찾아 새 계약에 맞게 다시 쓸 수 있다.
- **검사하지 않는 부분**: 작성 본문을 검사할 때 관리 구간(표지, 진행 결과, 계획 검토)은 제외한다.
- **Python 게이트**: Go로 옮긴 뒤 삭제한다. 실행 경로에서 이 스크립트를 참조하는 곳은 `issueops-remote-write` 스킬 하나뿐이다.

#### critical: 게시를 막는다

판정이 객관적이고 오탐이 적은 것만 둔다.

1. **한국어 비율**: 코드, URL, 경로를 뺀 문장에서 한글이 20자 이상이고, 한글 대비 영어 단어 비율이 1.2 이하여야 한다. 지금의 Python 게이트와 같은 기준이다.
2. **요약 절**: 첫 절이 `## 요약`이고 비어 있지 않아야 한다.
3. **필수 절**: 필수 절이 모두 있어야 하고, "없음", "해당 없음", "N/A", "TBD" 같은 자리 표시만 적혀 있으면 안 된다.
4. **해시**: 코드 밖에 64자리 hex(SHA-256)가 있으면 안 된다.

#### warning: preview에 줄 번호와 함께 보여 준다

1. 요약이 400자를 넘는다.
2. 코드 밖에 하네스 용어가 나온다. 초기 목록은 generation, lease, fingerprint, 봉인, sealed, canonical worktree, grill, plan-prep, readback, reconcile, brooks, turing, shannon, boehm이다.
3. 로컬 절대 경로(`/Users/…`, `/home/…`)가 있다.
4. 코드 밖에 40자리 커밋 SHA 전문이 있다. 꼭 필요하면 짧은 SHA와 설명을 함께 쓴다.
5. 선택 절이 비어 있다.
6. fluent-korean 규칙 중 기계로 잡을 수 있는 패턴이 나온다.
   - 단정을 피하는 어미: "~라고 할 수 있습니다", "~것으로 보입니다"
   - 서론 선언: "~을 살펴보겠습니다", "~하고자 합니다"
   - 엠대시 3개 이상
   - 문장 속에서 인과를 나타내는 화살표(→) 3개 이상
7. 확인한 것 절이나 검증 절에 결과가 "pass", "통과", "ok"뿐인 행이 있다.
8. 같은 문장이 두 절 이상에 반복된다.
9. 본문에 렌더하지 않는 필드가 입력됐다.

warning은 게시를 막지 않는다. 대신 작성 지침에 "warning마다 고치거나, 남겨 두는 이유를 한 줄로 적는다"는 규칙을 둔다.

#### 하지 않는 것

- 문장이 자연스러운지, 설명이 이해되는지는 기계로 판정하지 않는다. 이 부분은 fluent-korean 다듬기와 독자 검토가 맡는다.
- 하네스 용어를 critical로 두지 않는다. issueops 저장소의 이슈는 이 용어 자체가 주제인 경우가 많아서 오탐이 된다.

### 6. 결정 변경 반영

- 계획 검토, 구현 리뷰, 사용자 피드백에서 본문의 사실이 틀렸다고 판정되는 경우가 있다. 완료 기준, 범위, 원인 설명 같은 내용이다. 이때 `issueops feedback add --classification contract_change`로 기록한다.
- 기록이 남으면 기존 게이트 `contract_feedback_issue_update`가 PR 준비를 막는다(`internal/adapter/issueops/issueops_pr_readiness.go:68`).
- 이 게이트는 `sync-issue`로 본문을 고치고 `feedback mark-issue-updated`를 기록해야 풀린다.
- 새로 만들 Go 코드는 없다. `issueops-review`와 `issueops-plan` 스킬에 이 기록 규칙을 추가한다.
- plan 원문을 본문에 붙이지 않으므로, "본문과 접힌 plan이 서로 모순되는" 문제는 구조적으로 사라진다. 남는 과제는 본문 자체를 최신으로 유지하는 것이고, 이 규칙이 그 역할을 한다.

### 7. 작성 지침과 스킬

#### 작성 지침(`skills/issueops-remote-write/references/readable-body.md`)

- **구조는 explain을 따른다.**
  - 이슈는 계획 설명 순서로 쓴다: 문제 → 방식 → 근거 → 대안 → 검증.
  - PR은 작업 보고 순서로 쓴다: 바뀐 결과 → 확인한 것 → 확인하지 못한 것 → 남은 일.
  - 진행 결과는 작업 보고를 줄인 형태다.
- **문장은 fluent-korean을 호출해 다듬는다.** 본문이 길면 `references/slop-patterns.md`의 절차까지 적용한다.
- **용어 변환표**

  | 하네스 용어 | 본문에서 쓰는 말 |
  |---|---|
  | canonical worktree | 작업 공간 |
  | active owner, holder | 담당 세션 |
  | release, handoff | 작업을 넘긴다 |
  | readback | 게시한 뒤 다시 읽어 확인했다 |
  | devil's advocate | 계획 검토 |
  | G1~Gn, gates | 검증 항목 |
  | brooks, turing 같은 리뷰 에이전트 이름 | 역할 이름(독립 리뷰, 검증 루프) |
  | lease, generation, 봉인, fingerprint | 쓰지 않는다 |

- **모범 사례는 공개 사례만 쓴다.**
  - 이 저장소의 PR #495와 #489, Go 제안 #60078, 리눅스 커널 커밋 설명 사례를 쓴다.
  - 비공개 저장소의 문장과 수치는 옮기지 않는다. 거기서 얻은 원칙만 쓴다.
- **독자 검토(권장 절차)**
  - 맥락이 없는 subagent에게 제목과 본문만 준다. 그리고 네 가지 질문에 답하게 한다.
    1. 무엇이 문제이고 무엇이 바뀌는가?
    2. 왜 지금 필요한가?
    3. 끝났는지 어떻게 확인하는가?
    4. 모르는 용어나 두 번 읽은 문장이 있는가?
  - 답이 `intent.md`와 다르면 본문을 고친다.
- **warning 처리**: warning마다 고치거나, 남겨 두는 이유를 한 줄로 적는다.

#### 스킬별 변경

| 스킬 | 변경 |
|---|---|
| `issueops-remote-write` | 절차를 이 순서로 바꾼다: 골격 받기 → 작성 → fluent-korean → 독자 검토(권장) → preview 검사 결과 확인 → confirm → readback. Python 게이트 단계는 지운다. 본문 품질 규칙은 작성 지침 문서로 옮긴다. "provider MCP 도구로도 원격에 쓰지 않는다"는 규칙을 추가한다. |
| `issueops-create-issue`, `issueops-create-pr` | 절 목록, 13절 표, 옛 예시를 지운다. 대신 `render-template`과 작성 지침을 가리킨다. 조사와 기록 절차는 그대로 둔다. 라벨 판단을 `decision add`로 기록하는 단계를 추가한다. |
| `issueops-sync-issue`, `issueops-sync-pr` | 작성 지침을 가리킨다. 원격 본문 검사에서 나온 warning을 어떻게 쓰는지 추가한다. |
| `issueops-complete` | 진행 결과 원고를 쓰고 `reflect-completion --body-file`로 반영하는 단계를 추가한다. |
| `issueops-review`, `issueops-plan` | `contract_change` 기록 규칙을 추가한다. 지적을 독자가 읽을 수 있는 한국어 문장으로 쓰게 한다. |
| `issueops` 라우터 | explain을 쓰는 경우를 "사람에게 보고할 때와 이슈·PR을 쓸 때"로 넓힌다. |
| `gitlab-usecase` | 사이클 안에서는 glab MCP로 이슈나 MR을 만들거나 고치지 않는다는 규칙을 추가한다. |
| `.github`, `.gitlab` 템플릿 | 새 계약에 맞춰 다시 쓰고, 계약과 일치하는지 테스트로 확인한다. |
| `explain`, `fluent-korean` | 바꾸지 않는다. |

## 비목표

- 문장이 자연스러운지, 설명이 이해되는지를 기계로 판정하지 않는다.
- 독자 검토를 게시 조건으로 묶지 않는다. 효과를 측정한 뒤 다시 정한다.
- provider MCP 도구로 직접 게시하는 것을 기술적으로 막지 않는다. 규칙과 원격 본문 검사로 대응한다.
- 이미 게시된 이슈와 PR을 한꺼번에 고치지 않는다. 필요할 때 `sync-issue`로 새 계약에 맞춘다.
- AI 개입 고지를 본문에 강제하지 않는다.
- provider 어댑터 API, lease·generation·CAS 절차, record 스키마는 바꾸지 않는다.

## 검증

- **단위 테스트**
  - 계약: 종류별 필수 절, 필드 별칭, 렌더하지 않는 필드를 검증한다.
  - 검사 규칙: 규칙마다 통과하는 fixture와 실패하는 fixture를 둔다.
  - sync의 템플릿 추론을 검증한다.
- **명령 테스트**
  - critical이 있으면 `--confirm`이 거부된다.
  - preview JSON에 검사 결과가 들어간다.
  - `create-*`는 `--confirm`과 함께 `--template`을 요구한다.
  - sync는 원격 본문에 대한 warning을 보여 준다.
  - `reflect-completion --body-file`에도 같은 검사가 적용된다.
- **관리 구간 테스트**
  - 진행 결과 구간과 계획 검토 구간에는 64자리 hex, 로컬 절대 경로, 빈 소제목이 없다.
  - 시작 표지는 남는다.
  - 정리 감사는 본문에 쓰지 않는다.
  - `issueops list`의 반영 상태는 바뀌지 않는다.
- **golden과 벤치마크**: 응답 계약 golden을 갱신한다. 벤치마크의 절 개념(`internal/adapter/issueops/benchmark/issueops_benchmark_quality.go`, `cmd/issueops/issueopscli/benchmarkartifact`)을 새 계약에 맞춘다.
- **템플릿 일치**: `.github`와 `.gitlab` 템플릿이 계약의 필수 절과 일치하는지 테스트로 확인한다.
- **스킬 검증**: 바뀐 스킬마다 `python3 scripts/validate-skill.py`와 `python3 scripts/verify-skill-shell.py`를 실행한다.
- **전후 측정**
  - 기준선은 이번 조사 결과다.
    - PR/MR은 13절이다.
    - 완료 구간은 본문의 44~95%를 차지한다.
    - 이 저장소 본문 80건에 나온 하네스 용어 횟수를 쓴다.
    - 사내 저장소 issueops 이슈의 중앙값은 13절·8,358자다.
  - 새 형식으로 게시한 첫 이슈·PR 10건을 같은 방법으로 잰다.

## 작업 분할

parent 이슈 1개와 child 2개로 나누고, child는 순서대로 진행한다.

1. **`[s]` 사람이 쓰는 본문**
   - 본문 계약, 가독성 검사, sync의 템플릿 추론
   - 작성 지침과 remote-write·create-issue·create-pr·sync-issue·sync-pr 스킬
   - 라우터의 explain 연결, gitlab-usecase 규칙, 저장소 템플릿
   - Python 게이트 삭제, CAUTIONS 16.1 정정

   계약이 바뀌면 스킬 예시도 함께 바뀌어야 하므로 한 child로 묶는다.
2. **`[s]` 하네스가 붙이는 구간과 구현 자료**
   - 진행 결과(`reflect-completion --body-file`), 계획 검토 한 줄, prepare가 만드는 `plan-review.md`
   - record에만 남기는 정리 감사, 한국어로 바꾼 `sync-graph` 댓글
   - artifact 커밋 규칙과 새 ADR
   - complete·review·plan 스킬과 결정 변경 반영 규칙

   child 1의 검사 함수를 쓰므로 child 1이 끝난 뒤에 진행한다.

## 위험

- **전환 중 호환성**
  - 새 계약이 들어가면 옛 13절 형식의 본문은 요약 절 critical에 걸려 게시가 막힌다.
  - 계약과 스킬 예시는 child 1에서 함께 바꾼다.
  - 전환 전에 만든 사이클은 sync할 때 새 형식으로 다시 쓴다.
- **하네스 용어 오탐**
  - 하네스 용어 warning은 issueops 저장소 자체의 이슈에서 자주 뜬다.
  - 게시를 막지 않고, 남겨 두는 이유를 적고 넘어가게 한다.
- **진행 결과의 품질**
  - 진행 결과는 에이전트가 쓰므로 품질이 들쭉날쭉할 수 있다.
  - 같은 검사와 권장 독자 검토를 적용한다.
- **artifact 커밋**
  - PR diff에 plan과 spec 파일이 추가된다. 리뷰어는 `.issueops/issues/<n>/`를 건너뛰어도 된다.
  - 범위 검사 게이트가 이 경로를 허용해야 한다.
  - 봉인 확인이 파일 권한(0600)을 보는 경로가 있다. 커밋한 뒤에도 이 확인이 유지되는지 테스트한다.
- **공개 저장소**
  - 작성 지침의 사례와 테스트 fixture에는 비공개 저장소의 문장과 수치를 쓰지 않는다.

## 참고

- 이전 설계: `docs/superpowers/specs/2026-09-03-issueops-body-sync-design.md`
- ADR: `.issueops/adr/2026-09-09-issueops-seals-the-requester-intent-as-a-derived-artifact.md`
- 템플릿 조사: `docs/superpowers/research/2026-06-23-issueops-template-research.md`
- Google, "Writing good CL descriptions": https://google.github.io/eng-practices/review/developer/cl-descriptions.html
- Go 제안 #60078: https://github.com/golang/go/issues/60078
- Rust RFC 템플릿: https://github.com/rust-lang/rfcs/blob/master/0000-template.md
- Kubernetes KEP 템플릿: https://github.com/kubernetes/enhancements/blob/master/keps/NNNN-kep-template/README.md
- 리눅스 커널, Submitting patches: https://docs.kernel.org/process/submitting-patches.html
- Mozilla, Bug writing guidelines: https://bugzilla.mozilla.org/page.cgi?id=bug-writing.html
- Gong et al., "Analyzing Message-Code Inconsistency in AI Coding Agent-Authored Pull Requests", MSR'26: https://arxiv.org/abs/2601.04886
