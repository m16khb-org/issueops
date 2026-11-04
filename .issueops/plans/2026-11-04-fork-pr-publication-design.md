# 포크 head PR/MR 게시 지원 설계

- 날짜: 2026-11-04
- 상태: 설계 제안. 구현하지 않았고, 사용자가 아래 7장의 질문에 답하기 전에는 착수하지 않는다.
- 선행 문서: [2026-11-03 No enforcement hook for out-of-CLI actions](../adr/2026-11-03-no-enforcement-hook-for-out-of-cli-actions.md), [최적화·품질 후보 조사](../research/2026-11-03-optimization-quality-candidates.md)
- 범위: `issueops remote create-pr`, `remote verify-artifact`, `execution reconcile`(PR/MR 생성 복구), 그리고 이 셋이 읽는 `branch prepare` 봉인 값

## 요약

외부 저장소 기여 사이클 두 건은 `remote create-pr`이 세 조건 중 하나라도 표현하지 못해서 `gh pr create`를 직접 실행했다. 세 조건은 포크 head, 라벨·담당자 권한 없음, 한국어가 아닌 upstream 템플릿이다. 이 문서는 세 조건을 한 번에 풀지 않고 4단계로 나눠 닫는 설계를 제안한다.

핵심 결정은 다섯 가지다.

1. PR/MR을 소유하는 프로젝트(base)와 브랜치를 가진 프로젝트(head)를 분리한다. base는 기존 `code_project_key`가 이미 뜻하는 값이고, head는 새 `head_project_key`로 `branch prepare`에서 봉인한다. `create-pr`은 head를 새 플래그로 받지 않는다.
2. 후보 일치 규칙은 `Candidate.SourceProjectKey`가 이미 있으므로 한 줄만 바꾼다. 요청에 head 프로젝트가 있으면 그 값과 비교하고, 없으면 지금처럼 base와 비교한다.
3. 라벨·담당자 권한은 에이전트가 주장하지 못하고 CLI가 읽은 값으로만 면제한다. 권한이 없다고 읽힌 경우에만 `--metadata-policy skip`을 받는다.
4. 본문 검사는 `--body-profile upstream`으로 분리한다. 언어와 issueops 절 구조 검사는 끄고, 해시·로컬 경로·비밀 값 같은 언어와 무관한 검사는 유지한다.
5. 같은 저장소 PR의 바이트 단위 동작은 바뀌지 않는다. 새 필드는 모두 `omitempty`이고, 새 검사는 `head_project_key`가 봉인된 사이클에서만 실행한다.

## 1. 요구사항과 규모

### 1.1 현재 동작과 막히는 지점

| 조건 | 현재 동작 | 근거 |
|---|---|---|
| 포크 head | `gh pr create`는 `--head <branch>`만 받고 head 저장소를 지정하지 않는다. readback이 `headRepository`를 요청 프로젝트와 비교하고 다르면 `created pull request source project does not match published project authority`로 거부한다. | `internal/adapter/provider/github/provider.go:169-187`, `:335-337`, 테스트 `provider_test.go:253-265` |
| 포크 head, 복구 | 후보의 `ProjectKey`와 `SourceProjectKey`가 모두 요청의 `ProjectKey`와 같아야 한다. 포크 후보는 어떤 경우에도 채택되지 않는다. | `internal/domain/issueopspublication/candidate.go:56-60` |
| 포크 head, 목록 조회 | `gh pr list --head <branch> --limit 2`는 `<owner>:<branch>` 구문을 받지 않는다. 같은 이름의 브랜치(`main`, `patch-1`)를 쓰는 다른 포크 PR이 두 건만 있어도 `multiple-candidates`로 멈춘다. | `provider.go:386`, `gh pr list --help`의 `--head` 설명 `("<owner>:<branch>" syntax not supported)` |
| 포크 head, GitLab | `source_project_id != target_project_id`이면 `SourceProjectKey`를 빈 문자열로 돌려주고 readback이 거부한다. | `internal/adapter/provider/gitlab/provider.go:271-272`, `:345-350`, 테스트 `merge_request_gaps_test.go:134-135` |
| 프로젝트 권위 | 브랜치와 PR이 같은 프로젝트에 있다고 가정한다. `code_project_key` 주석도 "owns the branch and will own the PR/MR"라고 적는다. `branch prepare`는 `origin` URL에서 이 값을 추론한다. 포크 clone에서는 `origin`이 포크이므로 base가 포크로 봉인된다. | `internal/contract/issueops/types.go:51-55`, `internal/adapter/issueops/branch_preparation_environment.go:49-58`, `internal/domain/issueopsremote/branch_prepare.go:25-41` |
| 라벨·담당자 필수 | confirm에서 라벨과 담당자가 각각 하나 이상 있어야 한다. 영수증(`remote_artifact`)도 비어 있으면 거부한다. 소비자는 세 곳이다. | `internal/domain/issueopsremote/create_metadata.go:44-57`, `internal/domain/issueopsremote/artifact.go:75-90`, `internal/domain/issueopscompletion/artifact.go:71-76`, `internal/domain/issueops/cleanup_status.go:87-92` |
| 라벨 권한 없음 | `gh pr create`는 PR을 만든 뒤 라벨·담당자를 따로 갱신하고, 갱신이 실패하면 PR URL을 stdout에 쓴 채 `pull request update failed`로 끝난다. 라벨 권한 부족이 이 갱신 단계에서 실패하는지는 0단계에서 확인한다. [INFERENCE] 그렇다면 이 경로는 "URL이 따로 반환된 결과 불명"으로 분류되어 pending intent가 남고, 후보의 라벨이 요청과 달라 복구가 채택하지 못한다. 남은 출구는 abandon뿐이다. | `internal/adapter/provider/github/provider.go:210-212`, `internal/application/issueopspublication/create.go:51-55`, gh 소스 `pkg/cmd/pr/create/create.go`의 `submitPR`(`pr != nil` 이면 `fmt.Fprintln(opts.IO.Out, pr.URL)` 뒤 `pull request update failed`) |
| 비한국어 본문 | `artifacttemplate.Validate`가 `korean_body_required`를 내고, 가독성 검사가 한글 20자 이상·영어 단어 비율 1.2 이하·`## 요약` 첫 절·필수 절을 요구한다. confirm에서 critical이 하나라도 있으면 거부한다. | `internal/domain/artifacttemplate/template.go:114`, `internal/domain/artifactreadability/readability.go:162-170`, `internal/application/issueopsremote/template_body.go:44-90` |
| 본문 표지 | 생성 직전에 `<!-- issueops:issueops-v1 operation=... -->`를 본문 끝에 붙인다. 이 표지가 붙은 본문의 SHA-256이 복구 비교에 쓰인다. | `internal/application/issueopsremote/publication_journal.go:33-35`, `cmd/issueops/issueopsapp/issueops_publication_wiring.go:132-139` |
| 후속 안내 | 스킬은 이 세 경우를 "사용자 허락, `decision add --kind scope`, `issueops-abandon`" 순서로 안내하고, publication receipt 없이 사이클을 끝낸다. | `skills/issueops-create-pr/SKILL.md:119-137` |

ADR은 이 공백을 직접 후속 작업으로 지목했다: "포크 head를 create-pr이 지원하게 하는 일은 원격 쓰기 계약을 바꾸므로 별도 설계가 필요한 후속 작업이다."(`.issueops/adr/2026-11-03-no-enforcement-hook-for-out-of-cli-actions.md`)

### 1.2 규모

- 빈도는 낮다. 전사 조사에서 사이클 세션 358개 중 raw PR 생성은 2건(0.56%)이었고, 둘 다 세 조건을 모두 가진 외부 기여였다. 같은 저장소 PR 경로를 느리게 하거나 흔들면 얻는 것보다 잃는 것이 크다. 그래서 새 동작은 `head_project_key`가 봉인된 사이클에서만 켠다.
- 한 PR을 만드는 데 필요한 읽기 호출이 늘어난다. 포크 모드 preview는 GitHub 기준 `gh api graphql` 한 번(최대 15초, `providerReadbackTimeout`), GitLab 기준 REST 2~3번을 쓰는 것으로 설계했다. 질의가 실제로 한 번에 가능한지는 0단계에서 확인한다. 같은 저장소 모드는 호출 수가 그대로다.
- 코드 규모는 3.1의 파일 목록을 기준으로 한 거친 추정이다. 1단계 약 1,200줄(테스트 포함), 2단계 약 500줄, 3단계 약 400줄이고, 4단계는 문서와 스킬이다. 구현 중 달라질 수 있다.
- 바꾸는 외부 계약은 CLI 사용법 두 줄(`create-pr`, `verify-artifact`), `branch prepare` 플래그 한 개, 레코드 JSON의 `omitempty` 필드 몇 개다. MCP는 `issueops_execution`의 reconcile만 노출하고 `create-pr`·`verify-artifact`·`branch prepare`는 노출하지 않으므로 MCP 도구 스키마는 바뀌지 않는다(`cmd/issueops/testdata/mcp_tools.golden.json`에 `create_pr` 항목이 없다).

### 1.3 비목표

- 포크를 대신 만들거나 브랜치를 대신 push하지 않는다. `gh pr create`의 자동 fork·push 프롬프트를 쓰지 않는다. head 브랜치는 사용자가 이미 포크에 push한 상태여야 한다.
- 외부 PR을 merge하거나 외부 이슈에 쓰는 명령(`merge-pr`, `close-issue`, `reflect-completion`, `reflect-devils-advocate`)을 포크 사이클에서 쓰게 만들지 않는다. 포크 사이클의 이슈는 사용자가 소유한 프로젝트에 둔다는 전제를 7장 질문 2에서 확인한다.
- 조직 소유 포크는 1단계에서 지원하지 않는다. `gh pr create --head`의 `<user>:<branch>`는 조직을 지원하지 않는다고 도움말이 적는다. 이 경우 preflight가 `head_owner_is_organization`으로 거부한다.
- `branch retarget`은 포크 사이클에서 거부한다. 기준 브랜치를 `origin`에서 읽는 경로가 포크를 base로 오해할 수 있다고 추정하기 때문이다. [INFERENCE] 구현 전에 이 경로를 확인하는 단계를 6장 1단계에 넣었다.

### 1.4 성공 기준

1. 사용자가 포크에 push한 브랜치를 `create-pr`로 upstream에 올릴 수 있고, receipt에 head 프로젝트가 기록된다.
2. 라벨·담당자 권한이 없는 upstream에서 `--metadata-policy skip`으로 올릴 수 있고, 권한이 있는데 skip을 요청하면 거부된다.
3. 영어 upstream 템플릿 본문을 한국어 검사 없이 올릴 수 있고, 로컬 경로와 해시가 든 본문은 여전히 거부된다.
4. 생성 중 프로세스가 죽거나 응답이 모호해도 같은 head를 쓰는 다른 포크의 PR을 후보로 오인하지 않고, 중복 PR을 만들지 않는다.
5. `head_project_key`가 없는 기존 사이클의 레코드, intent payload, 응답 JSON은 바이트 단위로 같다.

## 2. 데이터와 API 계약

### 2.1 신원 모델

현재는 프로젝트 키가 하나이고, 그 키가 브랜치 소유와 PR 소유를 겸한다. 이 설계는 두 역할을 분리한다.

| 이름 | 뜻 | 봉인 시점 | 같은 저장소 사이클 |
|---|---|---|---|
| `ProjectKey` (기존 `code_project_key` 또는 이슈 프로젝트) | PR/MR을 소유하는 base 프로젝트 | `branch prepare` | 변경 없음 |
| `HeadProjectKey` (신규 `head_project_key`) | head 브랜치를 가진 포크 프로젝트 | `branch prepare` | 빈 문자열 |

`code_project_key`의 주석은 "owns the branch and will own the PR/MR"에서 "owns the PR/MR"로 고친다. 기존 값의 의미는 PR 소유 프로젝트였으므로 의미 변경은 없다.

### 2.2 새 플래그

| 명령 | 플래그 | 규칙 |
|---|---|---|
| `branch prepare` | `--head-project-key HOST/OWNER/REPO` | `--code-project-key`가 함께 있어야 한다. 값은 `ValidProjectKey`를 통과하고 호스트가 base와 같아야 하며 base와 달라야 한다(같으면 거부). 체크아웃의 `origin`이 이 값과 같아야 한다. 다르면 `origin_is_not_head_project`로 거부한다. |
| `create-pr` | `--metadata-policy exact\|skip` (기본 `exact`) | `skip`은 `head_project_key`가 봉인된 사이클에서만 받고, `--label`·`--assignee`가 비어 있어야 한다. 권한 preflight가 라벨·담당자 설정 권한 없음을 읽은 경우에만 통과한다. |
| `create-pr` | `--body-profile ko\|upstream` (기본 `ko`) | `upstream`은 `head_project_key`가 봉인된 사이클에서만 받는다. `--body-file`이 필요하고 빈 본문이면 골격을 만들지 않고 거부한다. |
| `verify-artifact` | `--metadata-policy exact\|skip` | `skip`이면 `--label`·`--assignee`가 필요 없고, 기록에 `head_project_key`가 봉인된 사이클에서만 받는다. 이 명령은 권한 preflight를 반복하지 않는다. 라이브 읽기가 PR URL, head 프로젝트, 대상 브랜치만 확인한다. |

`create-pr`에 head 프로젝트 플래그를 두지 않는 이유는 5장에 적었다. 봉인된 값이 유일한 권위이므로 에이전트가 요청마다 head를 바꿀 수 없다.

사용법 한 줄(`cmd/issueops/testdata/usage.golden.txt:98-99`)은 다음 모양이 된다.

```text
issueops remote create-pr ... [--metadata-policy exact|skip] [--body-profile ko|upstream] ...
issueops remote verify-artifact ... [--metadata-policy exact|skip] ...
```

### 2.3 intent payload와 "intent digest"

현재 intent에는 별도 해시가 없다. 동일성은 세 가지로 정해진다.

- `operation_id`와 본문 끝 표지(`publication_journal.go:33-35`).
- 저장된 payload와 기대 payload의 `reflect.DeepEqual` 비교(`internal/domain/issueopspublication/intent_transition.go:34-39`).
- 복구 때 계산하는 `BodySHA256`(표지가 붙은 본문 기준).

따라서 새 해시를 만들지 않고 `ProviderCreateRequest`에 필드를 더한다. `DeepEqual`이 새 필드를 자동으로 포함하므로 retry·failure·receipt 체크포인트가 모두 새 값까지 보호한다.

`internal/contract/issueopspublication/publication.go:69`의 `ProviderCreateRequest`와 `internal/port/provider.go:88`의 `IssueProviderCreatePullRequestRequest`에 추가하는 필드:

```go
HeadProjectKey string `json:"head_project_key,omitempty"` // 빈 값이면 head는 ProjectKey와 같은 프로젝트
MetadataPolicy string `json:"metadata_policy,omitempty"`  // "", "exact", "skip": 빈 값은 exact
BodyProfile    string `json:"body_profile,omitempty"`     // "", "ko", "upstream": 빈 값은 ko
```

같은 저장소 사이클은 세 값이 모두 비어 있으므로 payload JSON이 바이트 단위로 같다. 같은 이유로 `RemotePullRequestRequest`(`internal/contract/issueops/execution_api_types.go:100`)와 `CreateCommand`에는 `MetadataPolicy`와 `BodyProfile`만 더한다. `HeadProjectKey`는 요청이 아니라 봉인에서 온다.

`IssueProviderReconcilePullRequestRequest`(`internal/port/provider.go:106`)에도 `HeadProjectKey`를 더하고, `publicationReconcileRequest`(`issueops_publication_wiring.go:132`)가 채운다.

### 2.4 receipt 신원

`IssueOpsRemoteArtifactVerification`(`internal/contract/issueops/types.go:115`)에 `omitempty` 필드를 더한다.

```go
HeadProjectKey string `json:"head_project_key,omitempty"`
MetadataPolicy string `json:"metadata_policy,omitempty"`
BodyProfile    string `json:"body_profile,omitempty"`
```

값은 `PublicationJournal.Complete`(`publication_journal.go:148-152`)가 intent payload에서 옮긴다. `verify-artifact`로 직접 기록할 때는 기록(record)의 봉인 값에서 채운다. 영수증이 신원을 갖는 이유는 두 가지다. 사후에 "이 PR이 어느 포크에서 왔는가"를 읽을 수 있고, `sync-pr`이 같은 본문 프로파일로 검사할 수 있다.

영수증 소비자 세 곳은 정책을 읽도록 바뀐다.

- `requiredArtifactPeople`(`artifact.go:75`): `metadata_policy == "skip"`이면 라벨·담당자가 비어 있어도 통과한다.
- `validateArtifactLabelsAndAssignees`(`issueopscompletion/artifact.go:71`)와 `completion.RemoteArtifact`(`internal/contract/issueopscompletion/completion.go:64`): 같은 조건. 영수증 디코더(`internal/adapter/outbound/issueopscompletion/repository.go:155` 이후)가 `metadata_policy`를 읽는다.
- `cleanup_status.go:87-92`의 `remote_artifact_labels`·`remote_artifact_assignees`: 같은 조건.

### 2.5 후보 일치 규칙

`candidateLocationMatches`(`candidate.go:56-60`)의 `SourceProjectKey` 비교만 바꾼다.

```go
want := request.HeadProjectKey
if want == "" { want = request.ProjectKey }
// candidate.SourceProjectKey == want
```

`ProjectKey`(후보 URL의 프로젝트)는 계속 base와 같아야 한다. 이 변경은 domain 순수 함수 하나와 테이블 테스트로 닫힌다. `DecideReconcile`(`decision.go:35`)은 후보 수만 보므로 바꾸지 않는다.

`candidateContentMatches`는 `metadata_policy == "skip"`일 때 라벨·담당자 비교를 건너뛴다. 외부 저장소의 자동 라벨 봇이 라벨을 붙일 수 있기 때문이다. `exact`는 지금처럼 집합이 같아야 한다.

### 2.6 provider 인자 대응

GitHub:

| 용도 | 같은 저장소 (변경 없음) | 포크 모드 |
|---|---|---|
| 생성 | `gh pr create --repo <base> --head <branch> --base <b>` | `gh pr create --repo <base> --head <owner>:<branch> --base <b>`. `<owner>`는 `HeadProjectKey`의 두 번째 구성요소 |
| 라벨·담당자 | `--label`, `--assignee` 반복 | `exact`이면 그대로, `skip`이면 보내지 않는다 |
| readback | `gh pr view <url> --repo <base> --json ...,headRepository` 후 `headRepository.nameWithOwner`를 요청 프로젝트와 비교 | 같은 호출, 비교 대상이 `HeadProjectKey`. `isCrossRepository`를 JSON 필드에 추가하고 `true`를 요구한다 |
| 복구 목록 | `gh pr list --head <branch> --state all --limit 2` | `gh api repos/<base>/pulls -f head=<owner>:<branch> -f state=all -f per_page=2`. 서버가 head 소유자와 브랜치로 거른다 |

REST `head` 필터의 정의는 GitHub 공식 OpenAPI 설명에 있다: "Filter pulls by head user or head organization and branch name in the format of `user:ref-name` or `organization:ref-name`." 이 문구를 어댑터 주석에 그대로 남긴다(`state-policy-and-hooks.md`의 규칙: "외부 시스템의 어휘는 그 시스템의 정의에서 인용하고 출처를 코드에 남긴다").

복구 목록을 `gh pr list --limit N`으로 두지 않는 이유는 `--head`가 브랜치 이름만 걸러서 head 브랜치가 `main`인 포크 PR(흔한 형태)이 큰 저장소에서는 100건을 쉽게 넘기기 때문이다. 이 경우 0건을 권위 있는 부재로 증명할 수 없다.

REST 응답은 `gh pr list --json`과 필드 이름이 다르다. 어댑터에 `githubRESTPullRequest` 투영을 따로 두고 `port.IssueProviderReconcilePullRequestCandidate`로 옮긴다: `html_url`, `title`, `body`, `draft`, `state`와 `merged_at`(병합이면 `merged`), `labels[].name`, `assignees[].login`, `head.sha`, `head.ref`, `head.repo.full_name`, `base.ref`. `SourceProjectKey`는 `host + "/" + head.repo.full_name`을 소문자로 만든 값이다. `head.repo`가 null(포크 삭제)이면 빈 문자열이고 후보는 거부된다.

GitLab:

| 용도 | 같은 저장소 (변경 없음) | 포크 모드 |
|---|---|---|
| 생성 | `glab mr create --repo <base URL> --source-branch <b> --target-branch <t> --yes` | 같은 인자에 `--head <fork URL>` 추가. glab 1.121.0 도움말은 `-H --head  Select another head repository using the OWNER/REPO or GROUP/NAMESPACE/REPO format, the project ID, or the full URL.`이다. 전체 URL 형식을 쓴다. |
| readback | `glab mr view <iid> --repo <base URL> --output json`, `source_project_id == target_project_id`면 통과 | `source_project_id != target_project_id`를 허용하고, `glab api projects/<source_project_id>`의 `path_with_namespace`를 `HeadProjectKey`와 비교한다 |
| 복구 목록 | `glab mr list --repo <base> --source-branch <b> --all --per-page 2` | `--per-page 100`으로 읽고, 봉인된 head 프로젝트의 숫자 ID(`glab api projects/<urlencoded path>`의 `id`)와 `source_project_id`가 같은 행만 남긴다. 한 페이지가 가득 차면 `AuthoritativeZero=false`로 둔다 |

`glab`가 `--head`를 처음 지원한 버전은 이 조사에서 확인하지 못했다. 0단계에서 확인하고, 그 전까지 포크 모드 GitLab 생성은 `glab mr create --help` 출력에 `--head`가 있는지로 능력을 판정한다. 기존 `--version` 판정(`gitLabProjectRequiresGlab182`)은 `provider.go:132`(생성)와 `:295`(복구)에서 쓰므로 같은 자리에 능력 판정을 둔다.

### 2.7 preflight 계약

포크 모드의 `create-pr`은 preview와 confirm 모두 `BeginCreate` 전에 읽기 전용 preflight를 한 번 실행한다. 새 선택 포트를 둔다.

```go
// internal/port/provider.go: provider가 구현하지 않으면 포크 모드는 fail closed
type IssueProviderPullRequestPreflighter interface {
    PreflightPullRequestContext(context.Context, IssueProviderPreflightRequest) (IssueProviderPreflightFacts, error)
}
```

`IssueProviderPreflightFacts`는 관측값만 담는다.

| 필드 | GitHub 관측 | GitLab 관측 |
|---|---|---|
| `HeadReadable`, `HeadInBaseNetwork` | head 저장소의 `isFork`와 `parent`/`source`가 base와 같은 네트워크인지 | `forked_from_project` 체인이 base로 이어지는지 |
| `HeadOwnerKind` | `user` 또는 `organization` | 해당 없음 |
| `HeadRefSHA` | head 브랜치 끝 커밋 | 브랜치 끝 커밋 |
| `ViewerMetadataPermission` | `viewerPermission` (`READ`, `TRIAGE`, `WRITE`, `MAINTAIN`, `ADMIN`) | 프로젝트 접근 수준 숫자 |
| `ExistingOpenPullRequest` | base로 가는 같은 head의 열린 PR URL | 같은 소스 프로젝트·브랜치의 열린 MR URL |

GitHub는 두 저장소를 별칭으로 묶은 GraphQL 한 번으로 읽는다(`repository(owner,name){viewerPermission pullRequests(headRefName:,states:OPEN)}`와 head 저장소의 `ref(qualifiedName:)`). `gh repo view --json viewerPermission`이 실제로 `READ` 같은 값을 돌려주는 것은 이 조사에서 확인했다.

판정은 순수 domain 함수 `DecidePreflight(facts, request) []Finding`이 맡는다. 거부 코드는 다음과 같다.

| 코드 | 조건 |
|---|---|
| `head_repository_unreadable` | head 프로젝트나 브랜치를 읽을 수 없음 |
| `head_not_in_base_network` | head가 base의 포크 네트워크가 아님 |
| `head_owner_is_organization` | GitHub 조직 소유 포크 (1단계 비지원) |
| `head_ref_sha_mismatch` | 포크의 브랜치 끝이 `ExpectedHeadSHA`(워크트리 HEAD)와 다름. push가 안 됐거나 늦음 |
| `pull_request_exists` | 같은 head·base의 열린 PR이 이미 있음 (중복 생성 방지) |
| `metadata_permission_missing` | `exact`인데 라벨·담당자 설정 권한이 없음. 메시지에 `--metadata-policy skip`과 함께 `--label`·`--assignee`를 빼라는 `next_command`를 싣는다 |
| `metadata_permission_present` | `skip`인데 권한이 있음. 에이전트의 주장으로 면제하지 않는다 |
| `preflight_unavailable` | preflight 자체가 실패. 모든 경우 fail closed |

권한 임계값은 provider 공식 문서의 권한표를 인용해 어댑터 상수로 두고, 인용 문구와 URL을 주석에 남긴다. GitHub는 라벨과 담당자 설정을 `triage` 이상으로, GitLab은 문서의 역할표를 따른다. 이 임계값은 지금 확인하지 못했으므로 0단계 산출물이다. [INFERENCE]

preflight 결과는 응답 JSON의 `preflight` 객체(`omitempty`)로 preview에 보여 준다. `ProviderCreateResult`(`publication.go:94`)와 `executionissue.IssueProviderCreatePullRequestResult`에 필드를 더한다.

### 2.8 본문 프로파일 계약

`artifactreadability.Input`(`readability.go:54`)과 `artifacttemplate.IssueOpsTemplateInput`에 `Profile` 필드를 더한다. 값이 `upstream`이면 다음과 같이 달라진다.

| 검사 | `ko` (기본) | `upstream` |
|---|---|---|
| `korean_body_required`, `korean_ratio` | critical | 건너뜀 |
| `## 요약` 첫 절, 필수 절, `placeholder_section` | critical | 건너뜀 |
| `plan_link_section_forbidden` | critical | 건너뜀 (영어 템플릿의 `## Plan` 제목이 걸리지 않도록) |
| `harness_term`, 슬롭 문구, `duplicate_sentence` | warning | 건너뜀 |
| 빈 본문 | `template_missing` 계열 | critical `body_empty` |
| `sha256_hex` | critical | critical |
| `local_path` | warning | critical (외부 저장소에 로컬 경로가 새면 되돌릴 수 없다) |
| `commit_sha_full` | warning | warning |
| 비밀 값 의심(`policy.RedactFreeform`) | critical | critical |
| 저장소 PR 템플릿 제목 누락 | warning (`templateWarnings`) | critical `repo_template_sections_missing` (7장 질문 6) |
| 본문 길이 한도 | critical | critical |

`upstream` 프로파일은 본문을 한 글자도 고치지 않는다. `artifacttemplate.Render`가 만드는 한국어 골격을 쓰지 않고, 사용자가 쓴 본문과 표지만 게시한다. 저장소 템플릿 제목 비교는 이미 있는 `TemplateSectionsMissing`(`internal/domain/issueopsremote/repo_template.go:24`)을 쓴다. 이 함수는 제목을 대소문자 구분 없이 비교하므로 영어 템플릿에도 동작한다.

### 2.9 호환성

- 새 필드는 모두 `omitempty`다. 이전 바이너리가 새 레코드를 읽으면 필드를 무시하고 같은 저장소 경로로 해석한다. 이 경우 `gh pr create --head <branch>`가 upstream에서 브랜치를 찾지 못해 실패하거나 후보 비교가 거부한다. 잘못된 PR이 만들어지는 경로는 없다.
- 새 바이너리가 이전 pending intent를 읽으면 세 값이 비어 있어 기존 경로로 복구한다.
- `IssueOpsSchemaVersion`(`internal/contract/issueops/execution.go:4`)은 1로 유지한다. `.issueops/cautions/2026-07-07-issueops-orchestration-locks-additive-fields-worker-leases.md`가 기록한 대로 비소유 필드의 추가는 스키마 변경이 아니다.
- 갱신해야 하는 golden: `usage.golden.txt`, `response_contracts.golden.json`(응답에 `preflight`와 receipt 신규 필드가 든 경우만). golden은 "CLI/MCP contract를 의도적으로 바꾼 경우에만 golden 파일을 갱신한다."(`.issueops/testing/unit-and-contract.md:78`)에 따라 의도한 변경으로만 갱신한다.

## 3. 구조와 흐름

### 3.1 계층별 변경 지점

`architecture/issueops.md:30`은 이 경로를 이렇게 규정한다: "CLI create와 CLI/MCP reconcile은 같은 request-scoped handler pair를 사용하고, handler가 없으면 legacy full-flow로 우회하지 않고 fail closed한다." 새 로직도 이 vertical 안에 둔다.

| 계층 | 변경 | 파일 |
|---|---|---|
| contract | 필드 3개 + `Preflight` 응답 | `internal/contract/issueopspublication/publication.go`, `internal/contract/issueops/types.go`, `execution_api_types.go`, `internal/contract/issueopscompletion/completion.go` |
| domain (순수) | 후보 일치, `DecidePreflight`, 포크 봉인 검증(`PreparationHeadProject`), `PrepareCreateRequest`에 head 권위, 메타데이터 정책 검증, 프로파일 분기 | `internal/domain/issueopspublication/candidate.go`, 신규 `preflight.go`, `internal/domain/issueopsremote/branch_prepare.go`, `create_preparation.go`, `create_metadata.go`, `artifact.go`, `internal/domain/artifactreadability/readability.go`, `internal/domain/artifacttemplate/template.go` |
| application | `CreatePreparation`이 preflight를 호출, `PublicationJournal.Complete`가 receipt 신원 기록, `ArtifactVerificationService`가 기록의 head 키를 요청에 채움, `TemplateBodyResolver`가 프로파일 전달 | `internal/application/issueopsremote/create_preparation.go`, `publication_journal.go`, `artifact_verification.go`, `template_body.go`, `publication_command.go` |
| port | `IssueProviderPullRequestPreflighter`, 요청 구조체 필드 | `internal/port/provider.go` |
| adapter | GitHub·GitLab 생성 인자, readback, REST/ID 필터 복구, preflight, 라이브 검증 필드, 크래시 정산의 head 매칭 | `internal/adapter/provider/github/provider.go`, `gitlab/provider.go`, `internal/adapter/outbound/remoteverification/fetch.go`, `internal/adapter/issueops/publication_settlement.go` |
| 조립 | provider 매핑에 새 필드 복사, preflight 주입 | `cmd/issueops/issueopsapp/issueops_publication_wiring.go`, `internal/adapter/inbound/issueopspublication/create.go` |
| CLI | 플래그 | `cmd/issueops/issueopscli/remotecmd/remote_publication.go`, `remote.go`, `cmd/issueops/issueopscli/issueops_cli_support.go` |

### 3.2 `branch prepare`

`BranchPreparation`은 `--head-project-key`를 받아 다음을 검사한 뒤 `IssueOpsBranchPrepare.HeadProjectKey`로 봉인한다.

1. `--code-project-key`가 함께 있다. 포크 clone의 `origin`이 포크를 가리키므로 `ObserveCodeProjectKey`(`branch_preparation_environment.go:49`)가 돌려주는 값을 base로 쓰지 않는다. 추론에 맡기면 base가 포크로 봉인된다.
2. 두 키가 같은 호스트다.
3. `origin`의 프로젝트 키가 `--head-project-key`와 같다. head 브랜치는 `origin`으로 push되고, 정리 경로가 `origin`을 읽기 때문이다. 다른 원격 이름을 쓰는 사용자는 7장 질문 7에서 다룬다.

재기록 명령 두 곳은 새 플래그를 함께 내야 한다. 그러지 않으면 재기록이 봉인을 지운다(`BaseRerecordCommand`의 주석이 `base_sha`와 `link_verified`에서 겪은 같은 사고를 기록한다).

- `internal/domain/issueops/branch_prepare.go:77-95`의 `BaseRerecordCommand`
- `internal/domain/issueops/branch_await_link.go:67-86`의 `BranchLinkVerifiedCommand`

`origin`이 포크이고 `--head-project-key`가 없을 때 preview는 경고 `fork_origin_detected`를 낸다. 감지는 `gh api repos/<origin>`의 `fork`와 `parent`로 한다. 동작은 바꾸지 않는다. 지금은 이 경우 base가 포크로 봉인되어 `gh pr create --repo <fork>`가 포크 안에서 PR을 만들 수 있다. 경고에 `--code-project-key <parent> --head-project-key <origin>`을 안내한다.

issue 링크 게이트도 영향을 받는다. `branch_link_verified`는 이슈와 브랜치가 같은 저장소일 때만 만들 수 있다(`early_phase_readiness.go:45`, `issueopsnext/classify.go:353-384`, `await_link.go:46`, `cli_result_types.go:65`). 포크 사이클은 이 게이트를 면제해야 하고, 면제 조건은 `head_project_key`가 봉인된 것 하나다. GitHub `createLinkedBranch`가 포크 저장소에 브랜치를 만들 수 있는지는 0단계에서 확인한다. 가능하면 면제 대신 링크를 기록하게 하는 쪽이 더 낫다. [INFERENCE]

### 3.3 preview 흐름

```text
create-pr (no --confirm)
  ResolvePublicationDefaults        record.Branch를 head로, BranchPrepare.BaseBranch를 base로
  TemplateBodyResolver.Resolve      --body-profile에 따라 검사 (ko: 지금과 같음)
  ValidateRemoteCreateInputs        비밀 값, 길이
  CreatePreparation.Prepare
    ValidateCreateAuthority         provider, phase, artifact, generation
    PrepareCreateRequest            head == WorkspaceBranch, base == BaseBranch,
                                    HeadProjectKey = 봉인 값 (요청이 아님)
    [포크 모드] Preflight           읽기 호출, 위 7개 코드
    [exact 정책] ValidateConfirmCreateMetadata   라벨·담당자 각 1개 이상
    [skip 정책] 라벨·담당자가 비었는지, 권한 없음이 관측됐는지
  provider.Create(confirm=false)    DryRunPreview: gh/glab 인자 문자열
  응답: preview, readability, warnings, preflight
```

preflight는 `BeginCreate` 앞에서 실행되므로 거부해도 pending intent가 남지 않고, 호출이 `not_invoked`임이 자명하다. `architecture/issueops.md`의 규칙도 지킨다: "No network call (`git fetch`, `ls-remote`, `push`, provider CLI or API readback) runs while a span is held." preflight는 state root span 밖에서 실행한다.

### 3.4 confirm 흐름

preview와 같은 단계를 거친 뒤 기존 흐름을 그대로 쓴다.

```text
Prepare (preflight 포함)
  → BeginCreate: intent payload 저장(HeadProjectKey, MetadataPolicy, BodyProfile 포함), 표지 부착
  → HoldPublicationCreate(repo, head)        생성 수명 락
  → provider.Create                          gh/glab, 포크 모드 인자
  → verifier.VerifyLive(intent, url)         projectRemoteArtifact + 라이브 검증
  → repository.Complete                      receipt에 신원 기록, intent 삭제
```

preflight가 끝난 뒤 생성 사이에 틈이 있다(검사 시점과 사용 시점의 간격). 틈 사이에 포크 브랜치가 force push 되어도 생성 뒤 readback이 `HeadRefOID`를 `ExpectedHeadSHA`와 비교하므로(`provider.go:338-340`) 막힌다. 이 경우 PR은 이미 만들어진 상태라 기존대로 "알 수 없는 상태, 복구 필요"로 처리한다.

### 3.5 readback

포크 모드의 readback은 같은 저장소 모드와 비교 항목이 같고 두 가지만 다르다.

- `headRepository.nameWithOwner`는 요청의 `HeadProjectKey`와 비교한다(`provider.go:335`).
- 라벨 비교는 `MetadataPolicy`를 따른다. `exact`는 집합 동일(`providerutil.SameStrings`), `skip`은 비교하지 않고 관측값만 기록한다.

`headRepository`가 null이면(`nameWithOwner`가 빈 값) 빈 문자열로 투영되고 거부된다. 메시지는 "created pull request source project does not match published project authority"를 유지한다.

### 3.6 복구(reconcile)와 포크 후보

흐름은 `ReconcileService.Reconcile`(`reconcile.go:34-66`)을 바꾸지 않는다. `provider.Inspect`가 새 어댑터 경로를 타고 후보를 돌려주고, `DecideReconcile`이 그대로 판정한다.

```text
LoadIntent → Inspect(provider, intent)
  GitHub 포크 모드: gh api repos/<base>/pulls -f head=<owner>:<branch> -f state=all -f per_page=2
  GitLab 포크 모드: glab mr list --per-page 100 → 소스 프로젝트 ID로 필터
→ facts: CandidateCount, AuthoritativeZero, Invocation, RetryCount, CrashSettled
→ DecideReconcile: 0건+권위 있는 0+정산 완료 → 단일 재시도, 1건 → 채택, 2건+ → 보존
→ adopt: VerifyCandidate(ValidateCandidate: HeadProjectKey 규칙) → VerifyLive → Complete
```

두 가지가 달라진다.

1. 목록이 head 소유자로 서버에서 걸러지므로 다른 포크의 같은 이름 브랜치 PR이 후보에 들어오지 않는다.
2. 크래시 정산(`PublicationCrashSettlement`)의 프로세스 매칭이 head 문자열을 넓혀야 한다. 지금은 `intent.Request.HeadBranch`가 argv에 단어로 있는지로 `gh pr create` 진행 여부를 판정한다(`publication_settlement.go:77`, `:94`). 포크 모드의 `gh` argv에는 `--head owner:branch`가 들어가므로 `branch`만으로는 일치하지 않고, 정산이 "진행 중인 생성 없음"이라고 잘못 판정해 단일 재시도를 허용한다. 중복 PR이 생길 수 있는 경로다. 그래서 head 인자를 만드는 함수를 domain 하나로 모으고, 생성 인자와 정산 매칭이 같은 함수를 쓴다. 매칭은 `branch`와 `owner:branch` 둘 다 허용한다.

### 3.7 `verify-artifact`

`ArtifactVerificationService.Verify`(`artifact_verification.go:31-43`)는 `Validate`가 읽은 기록의 `BranchPrepare.HeadProjectKey`를 요청에 채운 뒤 라이브 검증을 호출한다. 라이브 읽기(`fetchGitHubPullRequestArtifactContext`, `fetch.go:16`)는 `headRepository`를 추가로 읽고, `ValidateArtifact`(`internal/domain/remoteverification/artifact.go:32`)가 요청의 head 키와 비교한다. 지금 이 경로는 head 저장소를 전혀 보지 않는다. 이 비교는 요청에 `HeadProjectKey`가 있을 때만 실행하므로 같은 저장소 사이클의 동작은 그대로다.

`ValidateArtifact`의 라벨·담당자 비교(`requireRemoteValues`, `artifact.go:54`)는 이미 "요청 ⊆ 실제"의 부분집합 비교이므로 외부 봇이 붙인 라벨에 흔들리지 않는다. `skip` 정책에서는 빈 요청이 항상 통과한다.

### 3.8 GitLab과 head 프로젝트 해석

GitLab은 head 프로젝트의 문자열 경로와 숫자 ID가 모두 필요하다. 생성은 전체 URL(`--head`), 복구 필터는 숫자 ID를 쓰고, readback은 `source_project_id`의 경로로 되돌려 비교한다. 해석은 어댑터 호출 하나 안에서 한 번만 하고 결과를 그 호출 안에서만 재사용한다. 영속 캐시는 만들지 않는다. 프로젝트 이름이 바뀌어도 봉인 값이 오래된 것으로 읽혀 거부되므로(4장) 캐시 일관성 문제가 없다.

## 4. 병목과 장애

| 상황 | 현재 | 설계 |
|---|---|---|
| 응답이 모호한 생성(타임아웃, 프로세스 종료) | pending intent가 남고, `execution reconcile`이 후보를 채택하거나 크래시 정산 뒤 단일 재시도한다 | 같은 상태 기계를 쓴다. 규칙: "Timeout or error is ambiguity, not absence; retry remains blocked until `execution reconcile` proves one exact outcome."(`architecture/issueops.md`). 포크 모드의 차이는 후보 목록(서버 필터)과 정산 매칭(`owner:branch`)뿐이다 |
| GitHub 라벨 권한 없음 | 1.1의 추정이 맞다면 PR은 만들어지고 `pull request update failed`로 끝나, URL이 딸린 모호 상태가 되어 영구히 막힌다 | preflight가 `viewerPermission`으로 사전에 거부하고 `skip`을 안내한다. skip에서는 라벨 인자를 보내지 않으므로 이 실패 경로가 사라진다 |
| GitLab 라벨 권한 없음 | 라벨이 조용히 버려진다고 추정한다. readback의 정확 비교가 실패한다 [INFERENCE: GitLab `IssuableBaseService#filter_params`가 권한 없는 사용자의 메타데이터 파라미터를 제거한다고 기억한다. 0단계에서 실제 인스턴스로 확인한다] | 같은 preflight 규칙. 권한이 없으면 `skip` |
| 담당자가 권한은 있지만 담당 가능 사용자가 아님 | readback 실패, 모호 상태 | 이번 설계의 범위가 아니다. 기존 동작을 유지한다 |
| 중복 생성(같은 head·base로 재실행) | pending intent가 두 번째 생성을 막는다. pending이 없는 상태(`not_invoked` 정리 뒤)는 막지 않는다 | preflight의 `pull_request_exists`가 열린 PR을 사전에 거부한다. GitHub는 같은 head·base의 열린 PR을 하나만 허용하는 것으로 알려져 있어 이중 안전장치가 된다 [INFERENCE: 0단계에서 확인] |
| 같은 이름 브랜치를 쓰는 다른 포크 PR | `gh pr list --head`가 포크 구분 없이 반환해 `multiple-candidates`로 멈추거나 100건 한도에 걸린다 | 서버 필터(GitHub), 소스 프로젝트 ID 필터와 페이지 가득 참 시 비권위(GitLab) |
| 같은 포크 브랜치를 재사용한 이전 PR(닫힘·병합) | 같은 저장소 모드에도 있는 문제. 후보 두 건으로 `multiple-candidates` 보존 | 이번 설계에서 바꾸지 않는다. 사용자는 abandon으로 정리한다. 외부 기여는 브랜치를 새로 만드는 쪽이 안전하다고 스킬에 적는다 |
| 포크 삭제·이름 변경·이전 | 해당 없음 | readback의 `headRepository`가 null이거나 달라져 거부, intent는 보존된다. 봉인 값이 오래된 것으로 읽힌다. 회복은 abandon 뒤 새 `branch prepare` |
| 외부 봇이 PR 본문이나 라벨을 즉시 수정 | 같은 저장소에선 드문 경우 | `exact`는 라벨이 어긋나면 실패한다. `skip`은 라벨을 비교하지 않는다. 본문 수정은 readback의 `got.Body != req.Body`(`provider.go:332`)로 실패한다. 설계에서 면제하지 않고 위험으로 남긴다(7장 질문 8) |
| preflight 지연 | 해당 없음 | GitHub는 호출 하나(최대 15초). GitLab은 2~3호출이라 병렬로 실행한다. 각 호출은 `RunBoundedReadbackContext`의 15초 제한을 받는다 |
| 속도 제한 | 해당 없음 | preview마다 읽기 호출이 생긴다. 한 PR당 preview가 몇 번인지는 에이전트가 정하므로, 포크 모드 preflight 결과를 응답에 실어 재호출 이유를 줄인다 |
| preflight와 생성 사이의 변경 | 해당 없음 | 생성 뒤 readback이 최종 방어선이다(3.4) |
| 라벨 권한이 중간에 생김·사라짐 | 해당 없음 | 생성 시점 preflight 값만 쓴다. 영수증에 `metadata_policy`가 남으므로 이후 소비자가 일관되게 읽는다 |

## 5. 절충점과 기각한 대안

| 선택 | 기각한 대안 | 이유 |
|---|---|---|
| head 프로젝트를 `branch prepare`에서 봉인한다 | `create-pr --head-project`로 요청마다 받는다 | 요청마다 받으면 에이전트가 head를 바꿔도 막을 곳이 없다. `PrepareCreateRequest`는 이미 head 브랜치와 base 브랜치를 봉인 값과 대조한다(`create_preparation.go:90`). head 프로젝트도 같은 방식으로 맞춘다 |
| `ProjectKey`는 PR 소유 프로젝트로 유지하고 `HeadProjectKey`를 더한다 | `ProjectKey`를 head로 바꾸고 `BaseProjectKey`를 더한다 | 영수증 검증, 완료 검증, 정리 경로 등 소비자 대부분이 `ProjectKey`를 PR 프로젝트로 읽는다. 의미를 뒤집으면 모든 소비자를 바꿔야 한다 |
| 후보 일치는 `SourceProjectKey` 비교만 바꾼다 | 후보 구조에 head 필드를 따로 더한다 | `SourceProjectKey`가 이미 head 저장소를 뜻한다. 이름만 맞으면 된다 |
| GitHub 복구 목록은 REST `head=owner:branch` | `gh pr list --head <branch>`에 `--limit 100`을 주고 클라이언트에서 거른다 | head 브랜치 이름이 `main`인 포크 PR이 큰 저장소에서 100건을 넘기면 0건을 증명할 수 없다. REST 필터는 서버가 head 소유자까지 걸러준다 |
| GitHub 생성은 `gh pr create --head owner:branch` | `gh api POST repos/<base>/pulls`로 직접 생성 | REST 생성은 `html_url`·`number`가 구조화되어 stdout 파싱이 사라지고 조직 포크도 지원한다. 그러나 라벨·담당자가 별도 호출이 되어 부분 상태가 늘고, 변경 표면이 새 쓰기 경로 하나만큼 커진다. 1단계는 기존 `gh pr create` 경로를 재사용한다. 조직 포크 수요가 확인되면 재평가한다 |
| 라벨·담당자 면제는 관측한 권한이 없을 때만 | 에이전트가 `--no-labels`를 주장하면 허용 | 이 저장소는 관측한 값만 믿는다(retarget도 "never asserted"). 면제를 주장으로 열면 라벨 필수 규칙이 사실상 선택 사항이 된다 |
| 권한 판정 후 `skip`은 라벨·담당자 둘 다 건너뜀 | 필드별 정책 | GitHub는 둘 다 `triage`가 기준이라고 추정하므로 필드별 구분이 필요 없다. [INFERENCE] GitLab은 기준이 다를 수 있으나 확인 전에 복잡도를 더하지 않는다(7장 질문 5) |
| 라벨 시도 후 실패를 허용하는 `best-effort` | `gh pr create`에 라벨을 주고 실패해도 계속 | PR은 만들어졌는데 영수증과 intent가 일치하지 않는 부분 상태가 곧 현재 장애다. 사전 관측으로 아예 보내지 않는 쪽이 상태가 단순하다 |
| 본문 프로파일은 `--body-profile upstream` 하나 | 언어를 자동 감지해 검사를 끈다 | 언어 자동 감지는 틀리면 한국어 게이트가 조용히 꺼진다. 명시 플래그 + 포크 사이클 한정이면 감사가 쉽다 |
| 영어 PR에도 표지를 붙인다 | 표지 없이 게시 | 표지가 `BodySHA256`과 복구 식별의 일부다(`publication_journal.go:33-35`). 숨은 HTML 주석이라 렌더링에 보이지 않는다. 외부 저장소에 흔적이 남는 문제는 7장 질문 3에서 묻는다 |
| preflight를 `Prepare`에서 실행 | `provider.Create`가 안에서 확인 | `Prepare`에서 실패하면 intent가 없으므로 `not_invoked`가 명확하다. `provider.Create` 안에서 실패하면 pending intent가 남는다 |
| 같은 저장소 모드는 preflight를 하지 않는다 | 모든 모드에 preflight | 같은 저장소 사이클은 브랜치를 push할 수 있는 권한이 있으므로 라벨 권한 문제가 없다고 본다. [INFERENCE] 호출 수와 지연을 늘릴 이유가 없고, 기존 동작이 바이트 단위로 같다는 성공 기준을 지킨다 |
| 포크가 없는 외부 기여(`gh pr create`의 자동 fork)는 비지원 | CLI가 fork를 만들고 push | 원격 쓰기를 둘 더하고, 포크 생성은 되돌릴 수 없다. 사용자가 push한 브랜치를 전제로 한다 |

## 6. 단계별 구현 계획

각 단계는 독립해서 병합할 수 있고, 앞 단계 없이 뒤 단계를 켜지 않는다. 단계마다 검증 명령은 `gofmt -l`, `go vet`, 대상 패키지 `go test`, `golangci-lint`, 새·변경 함수 `gocyclo <= 12`이다. golden은 계약을 바꾸는 단계에서만 갱신한다.

### 0단계: 관측 확인 (코드 없음)

버려도 되는 포크 저장소 두 개로 아래를 확인하고 결과를 이 문서 부록으로 추가한다. 추정을 사실로 바꾸는 단계이고, 결과에 따라 2·3장이 바뀔 수 있다.

1. `gh pr create --repo <upstream> --head <user>:<branch> --label x`를 트리아지 권한 없는 계정으로 실행해 PR 생성 여부, stdout, 종료 코드, 오류 문구를 기록한다.
2. `gh api graphql` 한 번으로 base의 `viewerPermission`·열린 PR과 head의 `isFork`·`parent`·`ref`를 읽는 질의를 확정한다.
3. `gh api repos/<base>/pulls -f head=<user>:<branch> -f state=all` 응답 필드를 확인한다.
4. `glab mr create --head`가 처음 들어간 버전, 포크 MR의 `source_project_id`·`target_project_id`, `glab api projects/<id>`의 `path_with_namespace`를 확인한다.
5. GitLab에서 권한 없는 사용자가 라벨을 보낼 때의 동작(조용히 버림인지 오류인지)과 역할별 임계값을 문서 인용과 함께 기록한다.
6. GitHub `createLinkedBranch`가 포크 저장소에 브랜치를 만들 수 있는지 확인한다.

산출물: 부록 표, 어댑터 주석에 인용할 공식 문구와 URL.

### 1단계: head 프로젝트 신원과 포크 생성·복구·검증

범위: 2.1~2.5, 2.6(생성·readback·복구), 2.7(preflight 중 head 관련), 3.2, 3.6, 3.7, 3.8.

변경:

- contract: `HeadProjectKey` 필드, `Preflight` 응답.
- domain: `candidateLocationMatches` 규칙, `PreparationHeadProject`, `PrepareCreateRequest`의 head 권위, `DecidePreflight`(head 관련 코드), head 인자 헬퍼.
- adapter: GitHub `owner:branch`·REST 복구·readback 비교, GitLab `--head`·소스 프로젝트 해석·복구 필터·readback, 라이브 검증 필드, 정산 매칭.
- CLI: `branch prepare --head-project-key`, 재기록 명령 두 곳, 링크 게이트 면제, `fork_origin_detected` 경고.
- 점검: `origin`과 `CodeProjectKey`를 읽는 모든 곳. 확인한 목록은 `branch/prepare.go:62-65`, `issueopscompletion/repository.go:147-153`, `artifact_verification.go:86`, `publication_journal.go:146`, `publication_verifier.go:49`, `branch_await_link.go:81`, `branch_prepare.go:89`이고, `branch retarget`의 `origin` 조회는 포크 사이클에서 거부한다.

테스트(소비자가 보는 동작 기준, 소스 문자열 검사 금지):

| 테스트 | 확인하는 동작 |
|---|---|
| `TestGitHubCreatePullRequestForkHeadPassesQualifiedHead` (신규) | 가짜 `gh`가 받은 argv에 `--repo base --head owner:branch`가 있다. 기존 `TestGitHubCreatePullRequestRejectsForkOriginReadback`는 `HeadProjectKey`가 비어 있는 요청에서 그대로 거부한다 |
| `TestGitHubForkReadbackAcceptsSealedHeadRepository`, `...RejectsOtherFork` | `headRepository`가 봉인 값이면 통과, 다른 포크면 `source project` 거부, null이면 거부 |
| `TestGitHubReconcileForkUsesServerHeadFilter` | 가짜 `gh api`가 `head=owner:branch` 쿼리를 받고, 다른 포크의 같은 이름 PR이 후보에 없다 |
| `TestValidateCandidateHeadProject` (domain, 테이블) | 비어 있으면 base와, 있으면 head와 비교. 포크 후보가 같은 저장소 intent에는 채택되지 않는다 |
| `TestGitLabForkCreateAndReadback`, `TestGitLabReconcileForkFiltersBySourceProjectID`, `...PageFullIsNotAuthoritativeZero` | `--head` 전체 URL, 소스 프로젝트 경로 해석, 100건 가득 참이면 권위 있는 0이 아님 |
| `TestPublicationCrashSettlementSeesQualifiedHead` | argv에 `--head owner:branch`가 든 진행 중인 생성이 있으면 정산이 false |
| `TestBranchPrepareHeadProjectKey` | `--code-project-key` 없음·호스트 불일치·`origin` 불일치·같은 키가 각각 거부된다. 봉인 뒤 `status`에 보인다 |
| `TestRerecordCommandsKeepHeadProjectKey` | `BaseRerecordCommand`와 `BranchLinkVerifiedCommand`가 `--head-project-key`를 유지한다 |
| `TestSameRepoPublicationBytesUnchanged` | 기존 differential 테스트(`testing/issueops-execution.md`의 byte-for-byte)의 record·`external_intent_v1` 행이 바뀌지 않는다 |
| `TestForkPreviewRefusesBeforeBeginCreate` | preflight 거부 시 pending intent가 없다 |

완료 조건: 0단계 확인 반영, 위 테스트 통과, 기존 publication 테스트가 수정 없이 통과, 버려도 되는 포크 쌍에서 preview → confirm → `verify-artifact` → `execution complete`를 한 번 실제로 실행한다.

### 2단계: 라벨·담당자 정책과 권한 preflight

범위: 2.2의 `--metadata-policy`, 2.4 receipt 필드와 소비자 세 곳, 2.7의 권한 코드.

테스트:

| 테스트 | 확인하는 동작 |
|---|---|
| `TestMetadataPolicySkipRequiresObservedMissingPermission` | 권한이 있으면 `metadata_permission_present`, 없으면 통과, 관측 실패면 `preflight_unavailable` |
| `TestMetadataPolicySkipRejectsLabelsAndAssignees` | skip과 `--label`을 함께 주면 거부 |
| `TestMetadataPolicySkipOnlyInForkCycle` | 같은 저장소 사이클에서 skip을 주면 거부 |
| `TestReceiptSkipPolicyPassesCompletionAndCleanup` | skip 영수증이 `execution complete`와 cleanup status를 통과한다. exact 영수증은 라벨·담당자가 비면 여전히 거부한다 |
| `TestGitHubCreateSkipSendsNoLabelArgs` | argv에 `--label`·`--assignee`가 없다 |
| `TestCandidateSkipPolicyIgnoresLabels` | 봇이 붙인 라벨이 있어도 skip 후보가 채택된다 |

### 3단계: 본문 프로파일

범위: 2.8, `sync-pr`이 영수증의 `body_profile`을 읽도록 연결(`internal/application/issueopsbodysync/sync.go`).

테스트:

| 테스트 | 확인하는 동작 |
|---|---|
| `TestUpstreamProfileAcceptsEnglishTemplateBody` | 영어 본문과 영어 제목으로 `create-pr --confirm`이 readability 거부 없이 통과 |
| `TestUpstreamProfileStillRefusesLocalPathAndSha256` | `/Users/...`가 든 본문과 64자리 hex가 든 본문은 critical |
| `TestUpstreamProfileRequiresBodyFile` | 빈 본문이면 골격을 만들지 않고 거부 |
| `TestUpstreamProfileRepoTemplateHeadings` | 저장소 템플릿 제목이 빠지면 `repo_template_sections_missing` |
| `TestKoreanProfileUnchanged` | 기본 프로파일의 가독성 결과가 기존 테스트와 같다(`remote_readability_test.go`) |
| `TestSyncPRUsesReceiptBodyProfile` | `upstream` 영수증의 `sync-pr`이 한국어 검사로 거부하지 않는다 |

### 4단계: 문서와 스킬

- `skills/issueops-create-pr/SKILL.md:119-137`의 "`remote create-pr`이 표현할 수 없는 PR" 절을 새 흐름으로 교체한다. 포크 사이클 준비(`branch prepare --head-project-key`), preview의 preflight 결과 읽기, `skip` 사용 조건, 영어 본문 작성 방법(한국어 윤문 스킬 호출 대신 upstream 템플릿 따르기)을 적는다.
- `skills/issueops-prepare/SKILL.md`의 "issue와 code가 다른 프로젝트에 있을 때" 절에 포크 사이클을 더한다.
- `.issueops/CAUTIONS.md` 계열에 라벨 권한 실패의 모호 상태와 정산 argv 매칭을 기록한다.
- ADR 하나를 `bin/issueops project append --kind adr`로 추가한다. 2026-11-03 ADR의 "표현할 수 없는 PR은 abandon으로 끝난다"를 대체하는 결정이다. 색인 행은 통합 담당이 `.issueops/ADR.md`에 추가한다.
- `python3 scripts/validate-skill.py skills/issueops-create-pr`, `skills/issueops-prepare`.

## 7. 사용자에게 묻는 질문

각 질문에 권장 답을 붙였다. 답이 없으면 권장 답으로 진행한다.

1. 포크 head 봉인(`branch prepare --head-project-key`)에 CLI가 사용자 승인 기록을 요구해야 하는가? 권장: 요구하지 않는다. 스킬이 "사용자 허락과 `decision add --kind scope` 기록"을 안내하고, CLI는 강제하지 않는다. 2026-11-03 ADR이 enforcement를 다시 넣지 않기로 한 방향과 같다.
2. 포크 사이클을 "이슈는 사용자 소유 프로젝트, 코드는 upstream"으로 한정해도 되는가? 권장: 한정한다. 외부 이슈에 한국어 진행 보고를 쓰는 `reflect-completion`과 `close-issue`를 포크 사이클에서 막는 별도 작업은 이 설계에 넣지 않는다.
3. upstream PR에 `<!-- issueops:issueops-v1 operation=... -->` 표지를 붙여도 되는가? 권장: 붙인다. 복구 식별에 필요하다. 싫다면 표지 없이 `BodySHA256`과 head SHA만으로 복구하는 약한 식별로 바꾸고, 그 경우 `multiple-candidates` 위험이 커진다.
4. 포크 사이클에서 `merge-pr`, `mark-ready`, `label`, `assign`, `retitle`은 어떻게 처리하는가? 권장: 1단계에서는 거부하지 않고 영수증의 base 프로젝트를 쓰게 둔다. 외부 저장소의 권한이 없으면 각 명령의 readback이 실패하며 안전하게 멈춘다. 사용자가 거부를 원하면 `head_project_key`가 있는 사이클에서 명시적으로 거부한다.
5. `--metadata-policy skip`을 라벨과 담당자 중 하나만 건너뛰는 필드별 정책으로 나눌 필요가 있는가? 권장: 나누지 않는다. GitLab 역할표를 0단계에서 확인한 뒤 필요하면 확장한다.
6. `--body-profile upstream`에서 저장소 PR 템플릿 제목 누락을 critical로 다루는가, warning으로 다루는가? 권장: critical. 외부 저장소는 템플릿 검사 봇이 흔하다. 오탐이 잦으면 warning으로 내린다.
7. 포크 원격 이름이 `origin`이 아닌 경우를 지원하는가? 권장: 1단계에서는 `origin`만 지원한다. 정리 경로가 `origin`을 읽는 구조라 다른 이름은 별도 작업이다.
8. 외부 봇이 PR 본문을 생성 직후 고쳐서 readback이 실패하는 경우를 면제하는가? 권장: 면제하지 않는다. 이 경우 intent가 남고 `execution reconcile`이 본문 해시 불일치로 보존한다. 실제 사례가 나오면 본문 비교를 표지 포함 여부로 완화하는 방안을 따로 설계한다.
9. 조직 소유 포크를 지원하는가? 권장: 1단계는 거부한다. 수요가 확인되면 REST `POST /repos/<base>/pulls`로 생성 경로를 바꾸는 별도 설계를 연다(5장 기각 대안).
10. 이 작업의 우선순위는 어디인가? 빈도가 0.56%이므로 같은 저장소 경로의 안정성 작업보다 낮게 둘 수도 있다. 0단계만 먼저 하고 결과를 본 뒤 1단계 착수 여부를 정하는 순서를 권장한다.

## 부록: 적용한 문서 규칙

이 설계가 직접 적용한 규칙과 읽은 파일이다.

- `.issueops/architecture/issueops.md`: "Timeout or error is ambiguity, not absence; retry remains blocked until `execution reconcile` proves one exact outcome." 응답이 모호한 포크 생성도 같은 pending intent 상태 기계를 쓴다(4장).
- `.issueops/architecture/issueops.md`: "No network call (`git fetch`, `ls-remote`, `push`, provider CLI or API readback) runs while a span is held." preflight는 span 밖, `BeginCreate` 앞에서 실행한다(3.3).
- `.issueops/architecture/issueops.md`: "CLI create와 CLI/MCP reconcile은 같은 request-scoped handler pair를 사용하고, handler가 없으면 legacy full-flow로 우회하지 않고 fail closed한다." 새 로직을 `issueopspublication` vertical 안에 두고, preflight 포트가 없는 provider는 포크 모드를 fail closed로 거부한다(3.1, 2.7).
- `.issueops/conventions/state-policy-and-hooks.md`: "외부 시스템의 어휘는 그 시스템의 정의에서 인용하고 출처를 코드에 남긴다" GitHub REST `head` 필터 정의, glab `--head` 정의, 권한표 임계값을 어댑터 주석에 인용한다(2.6, 2.7).
- `.issueops/testing/unit-and-contract.md`: "CLI/MCP contract를 의도적으로 바꾼 경우에만 golden 파일을 갱신한다." usage와 response 계약 golden은 1단계와 2단계에서 의도한 변경으로만 갱신한다(2.9).
- `.issueops/testing/issueops-execution.md`: publication vertical은 "frozen legacy oracle/new vertical differential로 create·reconcile의 result JSON, error text, record row, `external_intent_v1` row를 byte-for-byte 비교한다." 같은 저장소 경로가 바이트 단위로 같다는 성공 기준 5가 이 테스트에 기댄다(6장 1단계).
- `.issueops/adr/2026-11-03-no-enforcement-hook-for-out-of-cli-actions.md`: "포크 head를 create-pr이 지원하게 하는 일은 원격 쓰기 계약을 바꾸므로 별도 설계가 필요한 후속 작업이다." 이 문서가 그 별도 설계다.

## 부록: 현재 동작 근거 목록

| 항목 | 위치 |
|---|---|
| `create-pr` 플래그 정의 | `cmd/issueops/issueopscli/remotecmd/remote_publication.go:13-37` |
| `verify-artifact` 플래그 정의 | `cmd/issueops/issueopscli/remotecmd/remote.go:200-228` |
| 미리보기 조립, 본문 해석, 경고 | `internal/application/issueopsremote/publication_command.go:53-107`, `:119-145` |
| 본문 프로파일 분기 지점 | `internal/application/issueopsremote/template_body.go:44-90`, `:170-173` |
| 준비 단계 권위와 요청 조립 | `internal/application/issueopsremote/create_preparation.go:26-75`, `internal/domain/issueopsremote/create_preparation.go:84-105` |
| 생성·복구 서비스 | `internal/application/issueopspublication/create.go:43-70`, `reconcile.go:34-130` |
| 복구 판정 | `internal/domain/issueopspublication/decision.go:35-58` |
| 후보 검증 | `internal/domain/issueopspublication/candidate.go:45-67` |
| 영수증 기록과 표지 | `internal/application/issueopsremote/publication_journal.go:25-63`, `:123-160` |
| GitHub 생성·readback·복구 | `internal/adapter/provider/github/provider.go:125-166`, `:298-345`, `:377-410` |
| GitLab 생성·readback·복구 | `internal/adapter/provider/gitlab/provider.go:152-170`, `:217-275`, `:290-350` |
| 크래시 정산 | `internal/adapter/issueops/publication_settlement.go:62-104` |
| 생성 수명 락 | `internal/adapter/issueops/publication_settlement.go:21-33` |
| 라이브 검증 | `internal/application/remoteverification/verification.go:19-39`, `internal/adapter/outbound/remoteverification/fetch.go:16-55`, `:132-180`, `internal/domain/remoteverification/artifact.go:32-73` |
| 프로젝트 권위와 `origin` 관측 | `internal/domain/issueopsremote/issueops_remote_url.go:64-86`, `branch_prepare.go:25-41`, `internal/adapter/issueops/branch_preparation_environment.go:49-58` |
