# 최종 리뷰 F2 수정: API-doc 파일 workspace 경계와 loop record 조회

기준: `final-review.md` F2, 잔여 위험 목록의 server-cwd fallback. 기준 HEAD `92eaa001`, 미커밋 작업 트리.

## 결과

- F2 닫힘. capability 범위 호출(HTTP, 그리고 `authority_file`을 준 stdio)에서 `api_doc_review`의
  `diff_file`, `prompt_file`, `files`, `result_file`과 `api_doc_static_check`의 `files`는 심볼릭 링크 해석 뒤
  검증된 scope root 안에 있어야 한다. 밖이면 파일을 읽기 전에 `authority_invalid`로 거부한다.
- Native CLI(`issueops api-doc ...`)와 capability 없는 stdio 경로는 바꾸지 않았다. 검사 지점이
  `NewRequestScope`뿐이고, 이 함수는 capability 호출에서만 쓰인다.
- Record-root loop 조회는 `newLoopStore().ReadExisting(id)`로 record만 읽는다. `newLoopService()`와
  `os.Getwd` identity를 만들지 않으며, 알 수 없는 id가 loop store 디렉터리를 만들지도 않는다.

## 변경

| 파일 | 내용 |
|---|---|
| `cmd/issueops/mcpcli/mcp_request_scope.go` | `confinedFileArgs` 표(도구별 cwd 기준/root 기준 파일 인자), `confineRequestFiles`, `confineFile`, `resolveExistingAncestor`. `NewRequestScope`가 scope 확정 직후, credential을 읽기 전에 호출한다. |
| `cmd/issueops/issueopsapp/mcp_facade.go` | `RecordLoop` 조회를 `newLoopStore().ReadExisting`으로 교체. |
| 새 테스트 | `cmd/issueops/mcpcli/mcp_request_confine_test.go`, `cmd/issueops/issueopsapp/mcp_http_apidoc_boundary_test.go`, `cmd/issueops/issueopsapp/mcp_record_roots_loop_test.go` |

설계 선택:

- gates의 `WorkspaceFileStore`처럼 `os.OpenRoot(scope.WorkspaceRoot)`를 쓴다. 입력을 가장 깊은 기존 조상까지
  `EvalSymlinks`로 풀어 `/tmp`와 `/private/tmp` 같은 별칭을 맞춘 뒤, `filepath.IsLocal`로 root 상대 이름을 만들고
  `root.Stat`로 `..`/심볼릭 링크 탈출을 `os.Root`가 판정하게 한다. 존재하지 않는 대상만 통과시키며(이후 읽기가
  실패한다), 밖을 가리키는 dangling symlink는 `os.Root`가 거부한다.
- `files`와 `result_file`은 어댑터가 repo(=scope root)에 붙여 읽으므로 root 기준으로, `diff_file`과
  `prompt_file`은 `scopedArguments`가 cwd 기준으로 절대화하므로 cwd 기준으로 검증한다. 인자 재작성은 하지 않았다.
- 모든 `pathArgs`에 일괄 적용하지 않고 API-doc 두 도구만 opt-in했다. `issueops_execution`의
  `claim_token_file` 등은 workspace 밖 state 경로를 쓸 수 있어 일괄 적용하면 회귀한다.
- `mcp_tool_authority.go`와 `mcp_request_dispatch.go`는 쓰기 범위 밖이라 건드리지 않고, 표를 scope 파일에 두었다.
- 검사는 credential 검증보다 앞선다. 그래서 grant가 없거나 만료된 호출도 밖 경로에는 `authority_invalid`(경로 사유)를
  받는다. 파일은 어느 경우에도 읽지 않는다.

## RED

수정 전 `go test ./cmd/issueops/issueopsapp -run 'TestMCPHTTPAPIDocFilesStayInsideAuthorizedWorkspace|TestMCPRecordRoots' -count=1`
(로그 `/tmp/f2-red/red.log`):

- `TestMCPHTTPAPIDocFilesStayInsideAuthorizedWorkspace`의 밖 파일 17개 서브테스트가 모두 실패했다.
  `prompt_dotdot_to_real_sibling`, `prompt_absolute`, `prompt_through_symlink_dir`, `prompt_symlink_file`,
  `prompt_absolute_through_symlink_dir`, `diff_*`는 `REVIEW_OUTSIDE_SCOPE_MARKER_7581`을 담은 prompt를
  `isError=false`로 돌려줬다. `result_*`와 `prompt_dotdot`은 밖 파일을 읽다 구조화되지 않은 protocol error로
  실패했다. `files_*`, `static_files_*`는 밖 파일을 받아들였고, `files_symlink_file`은 밖 컨트롤러 내용을
  prompt에 실었다.
- `TestMCPRecordRootsUnknownLoopDoesNotCreateLoopStore` 실패: `record lookup created the loop store`.

## GREEN

- `go test ./cmd/issueops/mcpcli -run 'TestRequestScope|TestScopedArguments'` ok. 새 표 테스트
  `TestRequestScopeConfinesAPIDocFilesToWorkspace`는 안쪽 파일, 안쪽 심볼릭 링크, 중첩 cwd, 없는 안쪽 파일을
  통과시키고 `..`, 절대 경로, 디렉터리/파일/dangling 심볼릭 링크 탈출을 `authority_invalid`로 거부한다.
  gates 같은 비대상 도구는 동작이 변하지 않는다. 이 표 테스트는 production 변경 전에 따로 RED를 실행하지
  않았다. RED 증거는 위 통합 테스트다.
- `go test ./cmd/issueops/issueopsapp -run 'TestMCPHTTPAPIDoc|TestMCPRecordRoots'` ok. 밖 파일 17종 전부
  `authority_invalid`에 marker 없음, 안쪽 상대/절대/`..`로 되돌아오는 경로/안쪽 심볼릭 링크/`files`는 성공,
  서버 cwd는 비어 있다. loop 테스트는 cwd를 삭제한 상태에서도 record repo를 반환한다.

## 검증 명령

| 명령 | 결과 |
|---|---|
| `gofmt -l $(변경·신규 .go)` | 출력 없음 |
| `go vet ./...` | 출력 없음(통과) |
| `go test -race ./cmd/issueops/mcpcli/...` | ok (mcpcli, argmap, resources) |
| `go test -race ./internal/adapter/outbound/apidoc/... ./internal/application/apidoc/... ./cmd/issueops/apidoc/...` | 모두 ok |
| `go test -race ./cmd/issueops/issueopsapp -run 'MCP\|HTTP\|APIDoc\|Api'` | 1건 실패: `TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions` (`authority credential was revoked or never issued`) |

위 실패는 이번 변경의 결과가 아니다. final-review F1(resume/reseed가 grant가 없는 다른 DB에서 guard를 확인)을 다루는
테스트로, 다른 노드가 소유한 sqlstore/authority/issueopslease 수정이 들어와야 통과한다. 이번 변경이 닿는 api_doc와
record-root 테스트는 모두 통과했다.

## 실제 확인 (격리 HTTP 서버)

`go build -o /tmp/f2-apidoc-bin ./cmd/issueops`(저장소 `bin/issueops` 미변경). HOME, CODEX_HOME, XDG_CONFIG_HOME,
`ISSUEOPS_STATE_DIR`을 모두 `/tmp/f2-real` 아래로 격리하고, 비어 있는 `/tmp/f2-real/server-cwd`에서
`mcp --http --addr 127.0.0.1:0`을 띄웠다. repo A에는 `api.diff`, `inside-prompt.md`,
밖 파일을 가리키는 `linked-prompt.md` 심볼릭 링크를 두고, 형제 `/tmp/f2-real/outside-prompt.md`에는
`REAL_OUTSIDE_MARKER_7581`을 넣었다. grant는 `mcp authorize`로 repo A에 대해 발급했고(살아 있는 bash 세션이
조상), 요청은 bearer를 단 실제 JSON-RPC `tools/call` POST다.

| 호출 (`api_doc_review`, `repo`=repo A) | 결과 |
|---|---|
| `prompt_file: ../outside-prompt.md` | HTTP 200, `authority_invalid: prompt_file is outside the authorized workspace`, outside marker 없음 |
| `prompt_file: <outside 절대 경로>` | 같은 거부, marker 없음 |
| `prompt_file: linked-prompt.md` (밖으로 가는 심볼릭 링크) | 같은 거부, marker 없음 |
| `diff_file: ../outside-prompt.md` | `authority_invalid: diff_file is outside the authorized workspace`, marker 없음 |
| `prompt_file: inside-prompt.md` | 정상 처리(`verdict: pending`), prompt에 inside marker 포함, outside marker 없음 |

서버 cwd(`/tmp/f2-real/server-cwd`)는 끝까지 비어 있었고, 서버와 세션 프로세스는 종료했다.

처음 몇 번의 실제 확인 시도는 `mcp authorize`가 조상 receipt를 받지 못해(`comm`이 `/bin/bash`, 시작 시각은
RFC3339 UTC 필요) 실패했다. 테스트 스크립트 문제였고 production 동작과 무관하다. 안쪽 파일 호출은 세션 프로세스가
살아 있어야 grant 검증을 통과한다.

## 잔여 위험

- 검사(`os.Root.Stat`)와 이후 읽기(`os.ReadFile`) 사이에 TOCTOU가 남는다. 같은 OS 사용자가 그 사이에 링크를 바꾸는
  경우다. 읽기 자체를 `os.Root`로 옮기려면 reviewfiles/mcp_facade의 읽기 경로를 바꿔야 하고 native CLI의
  심볼릭 링크 동작이 달라질 수 있어 이번 범위에서 제외했다.
- 검사 표(`confinedFileArgs`)는 도구를 직접 나열한다. 파일을 직접 읽는 새 workspace 도구가 생기면 표에 추가해야
  한다. `mcp_tool_authority.go`의 `toolAuthority`로 옮기는 편이 낫지만 쓰기 범위 밖이라 후속 과제로 남긴다.
- F1 테스트(`TestMCPHTTPCapabilityReachesResumeAndReseedDomainDecisions`)는 다른 노드의 수정을 기다린다.
