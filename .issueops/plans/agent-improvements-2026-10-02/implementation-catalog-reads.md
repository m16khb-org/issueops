# I7 구현 증거: 결정적 bounded 문서 카탈로그 읽기와 생략 수 표시

상태: I7 범위 구현·검증 완료. 전체 저장소·호스트 검증은 주장하지 않는다.
기준 HEAD: `92eaa00143841f964c285dacdd41aedbeeecdf2e`, 작업 트리는 다른 노드의 변경과 공존한다.
모델 환경: `PI_MODEL=claude-sonnet-5-5`, `PI_REASONING_LEVEL=high`. 두 변수 모두 export된 값을 그대로 기록했고 환경을 바꾸지 않았다.

## 변경 파일

| 파일 | 변경 |
|---|---|
| `internal/domain/projectdoc/catalog_types.go` | `CatalogOmissions{Oversize,Unreadable,OverCap,HeaderTruncated int; ScanTruncated bool}`(snake_case, omitempty), `CatalogStats{FilesOpened,BytesRead,DirBatches}`, `Any()`·`Summary()` 추가 |
| `internal/adapter/projectdoc/catalog.go` | 아래 알고리즘으로 재작성. `DiscoverProjectDocsReport` 신설, `DiscoverProjectDocs`는 entries wrapper. `FormatProjectDocCatalogOmissions` 추가. `projectDocCatalogMaxRawEntries`·`projectDocCatalogMaxTotalBytes` 제거 |
| `internal/adapter/projectdoc/catalog_test.go` | 새 회귀 테스트 추가, 불변식이 바뀐 두 테스트 교체(아래) |
| `internal/contract/hookprompt/types.go` | `ProjectDocCatalogContext.Omitted *projectdoc.CatalogOmissions` (`omitted,omitempty`, additive) |
| `internal/application/hookprompt/catalog.go` | `CatalogService`에 `DiscoverReport`, `FormatCompactOmission`, `FormatUserOmission` 선택 필드 추가. `Build`가 생략이 있을 때만 `Omitted`와 두 view 접미를 채움 |
| `internal/application/hookprompt/catalog_test.go` | 생략 없음 바이트 동일, 정확한 count/JSON, docs 없이 생략만 있는 경우 테스트 |
| `internal/adapter/hookprompt/catalog.go`, `catalog_test.go` | `RenderProjectDocCatalogOmissions` 추가·테스트 |
| `cmd/issueops/issueopsapp/hook_facade.go` | 서비스를 `DiscoverProjectDocsReport`와 두 omission formatter에 연결 |

설치·hook 정의, golden, `meta.go`, `constants.go`, openwiki는 수정하지 않았다.

## 동작

- 디렉터리 이름을 EOF까지 128개 batch로 읽는다. 필수 11개 → 선택 2개 → 나머지 사전순으로 상위 64개만 정렬된 bounded 슬라이스에 보존한다(메모리 batch+64개). 나머지는 `over_cap`으로 센다. 생성·순회 순서와 무관하다.
- 선택된 파일만 `Lstat`→`Open`→`fstat`한다. 256KiB 초과는 본문 0 byte로 `oversize`. 열기·읽기 실패는 `unreadable`. 통과하면 최대 8KiB header만 읽는다. 잘린 header는 마지막 개행까지만 해석한다.
- header 안에서 frontmatter가 닫히지 않으면 canonical 설명으로 fallback하고, H1이 밖이면 Title은 빈 값이다. 이런 truncated 문서는 `header_truncated`로 센다.
- 읽기 byte 상한은 64×8KiB다. 합계 2MiB 상한 로직은 제거했다.
- directory read 오류는 부분 결과를 유지하고 `scan_truncated=true`다.
- 기존 안전 불변식(`os.OpenRoot`, `.issueops` symlink 거부, symlink·비정규 `.md` 제외, `Lstat`+`fstat` regular 검사)은 그대로다. symlink·비정규 항목은 문서가 아니라고 보고 omission으로 세지 않는다.
- 생략이 없으면 compact/user view와 JSON은 기존과 바이트 동일하다(테스트로 고정). 생략이 있으면 compact 끝에 `; omitted: over_cap=77`, user view에 한 줄, JSON에 `"omitted":{...}`가 붙는다. 문서가 하나도 없고 생략만 있으면 주입하지 않지만(`should_inject=false`) `--json`에는 `omitted`가 남는다.

## 의도적으로 바꾼 기존 테스트

- `TestDiscoverProjectDocsBoundsEntriesAndContent`의 마지막 단언("240KiB 8개 뒤 합계 상한으로 중단")은 합계 상한 폐기에 따라 `TestDiscoverReadsBoundedHeader`로 대체했다. 불변식이 "내용 합계"에서 "읽은 byte ≤ 파일 수×8KiB"로 바뀐 것이며 약화가 아니라 contract §4 I7의 결정이다. 같은 입력(240KiB 9개)을 모두 수용하고 `BytesRead`를 단언한다.
- `TestReadProjectDocCatalogEntriesCapsRawDirectoryScan`(단일 `ReadDir(128)` 고정)은 batch 순회로 의미가 바뀌어 `TestScanProjectDocNamesMarksReadErrorAsTruncated`와 batch 수 단언(`TestDiscoverSelectionIndependentOfCreationOrder`)으로 대체했다.

## RED / GREEN

RED (구현 전, 새 테스트만 추가한 상태):

```text
go test ./internal/adapter/projectdoc ./internal/application/hookprompt ./internal/adapter/hookprompt -count=1
FAIL ... [build failed] x3   # undefined: DiscoverProjectDocsReport, projectDocCatalogHeaderBytes,
                              # projectdoc.CatalogOmissions/CatalogStats, CatalogService.DiscoverReport,
                              # RenderProjectDocCatalogOmissions ...
EXIT=1
```

GREEN (요구된 검증 명령, 구현 후):

```text
gofmt -l internal/adapter/projectdoc internal/domain/projectdoc internal/application/hookprompt internal/adapter/hookprompt internal/contract/hookprompt cmd/issueops/issueopsapp/hook_facade.go   -> 출력 없음
go test ./internal/adapter/projectdoc ./internal/domain/projectdoc ./internal/application/hookprompt ./internal/adapter/hookprompt ./cmd/issueops/hookcli -count=1
ok issueops/internal/adapter/projectdoc / internal/domain/projectdoc / internal/application/hookprompt / internal/adapter/hookprompt / cmd/issueops/hookcli
EXIT=0
```

추가로 실행한 검사:

| 명령 | 결과 |
|---|---|
| `go vet` (projectdoc, domain/projectdoc, application/adapter hookprompt, contract/hookprompt, hookcli) | exit 0, 출력 없음 |
| `go test -race` (adapter/projectdoc, application/hookprompt, adapter/hookprompt) | ok |
| `go test ./cmd/issueops/contractgolden -run Golden` | ok (골든 영향 없음: `omitted`는 additive·omitempty) |
| LSP 진단 (변경 패키지 전부) | error 0. 힌트(`rangeint`, `SplitSeq`)와 기존 `writestring` 경고만 있고 수정하지 않음 |

테스트는 시계·sleep 없이 `BytesRead`/`FilesOpened`/`DirBatches` 카운터와 fake `ReadDir`로 결정적이다. 권한 거부 테스트는 root에서 skip한다.

## 실제 사용 증거 (임시 fixture, 실제 `hook session-start --json`)

fixture: `.issueops`에 `0-00..0-69.md`(70), `pad-00..59.md`(60), 240KiB `big-00..08.md`(9), 257KiB `zz-oversize.md`, `ARCHITECTURE.md` = 적격 141개(>128 batch). HEAD를 `git archive`로 빌드한 old 바이너리와 작업 트리 new 바이너리를 같은 fixture로 실행했다(둘 다 exit 0, 각각 두 번 실행해 출력 동일).

```text
OLD: docs=64 omitted=(없음)   first5=0-02,0-03,0-06,0-07,0-09 ...   # 디렉터리 순서 128개를 먼저 자른 임의 집합, 0-00/0-01이 빠져도 알 수 없음
NEW: docs=64 omitted={"over_cap":77}
     first5=0-00..0-04 last3=0-61,0-62,ARCHITECTURE.md              # 필수 문서 우선, 사전순 상위
     compact_tail=...ARCHITECTURE.md=System structure, component boundaries, and responsibilities.; omitted: over_cap=77
     user_view_tail=...⚠ 일부 문서가 목록에서 생략됨: over_cap=77
```

`--host claude` SessionStart 출력도 같은 경고 줄을 포함한다. 읽기 byte 상한(9×240KiB 입력에서 `BytesRead ≤ 9×8192`)은 fixture 실행이 아니라 `TestDiscoverReadsBoundedHeader`로 단언했다. 옛 코드의 byte 수는 직접 측정하지 못했다(`dtruss` 등은 sudo가 필요). contract의 "최악 2MiB"는 코드 읽기 근거이며 실측이 아니다. 벤치마크는 실행하지 않았다.

## 남은 통합 문제 (범위 밖)

1. **DDD 인벤토리 골든이 오래됨.** `go test ./internal/architecture -count=1`이 `TestDDDResponsibilityInventoryMatchesSource`에서 실패한다("production file or symbol has no recorded responsibility"). `internal/architecture/testdata/ddd_responsibility_inventory.json`은 공유 골든이라 수정하지 않았다. 내 새 production 심볼이 원인에 포함된다: `DiscoverProjectDocsReport`, `FormatProjectDocCatalogOmissions`, `scanProjectDocNames`, `projectDocNameSelector`(+메서드), `readProjectDocCatalogHeader`, `buildProjectDocCatalogEntry`, `firstMarkdownHeading`(옛 `firstMarkdownTitle`·`readProjectDocCatalogEntries`·`readProjectDocCatalogFile` 제거), `CatalogOmissions.Any/Summary`, `RenderProjectDocCatalogOmissions`. 다른 노드(trace 등)도 심볼을 바꾸므로 Z-integration이 마지막에 한 번 갱신해야 한다.
2. **`cmd/issueops/issueopsapp` 테스트가 빌드되지 않는다.** I6 노드의 `issueops_next_git_observation_test.go`가 아직 없는 `newNextGitObservation`을 참조한다. 그래서 `issueopsapp`의 hook 관련 테스트와 `TestResponseContractsGolden`은 실행하지 못했다. 같은 hook 배선은 `go build ./cmd/issueops`와 실제 바이너리 실행, `hookcli` 테스트로 확인했다. C 노드 완료 뒤 해당 패키지를 재실행해야 한다.
3. **API/DTO 게이트.** `ProjectDocCatalogContext.Omitted`는 `--json`·Omo 확장이 읽는 DTO에 추가된 필드다. `issueops api-doc static-check`와 review는 실행하지 않았다(HTTP endpoint 변경이 아니라 hook JSON이므로 Z가 필요 여부를 판단). Omo 확장은 알 수 없는 필드를 무시해야 한다는 가정이며 확인하지 않았다.
4. `HeaderTruncated`는 "문서가 8KiB보다 크고 window에 H1이 없거나 frontmatter가 닫히지 않음"으로 정의했다. frontmatter는 닫혔지만 H1이 window 밖인 문서도 센다.
5. `DirBatches`는 엔트리를 반환한 `ReadDir` 호출 수이며 마지막 EOF 호출은 세지 않는다.

임시 산출물(`/tmp/i7-evidence`)은 이 보고서 작성 뒤 삭제했다.
