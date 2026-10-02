# I7 catalog read map: SessionStart 문서 발견 IO 축소

기준 HEAD `92eaa00`. `PI_MODEL=claude-sonnet-5-5`, `PI_THINKING_LEVEL`은 export되지 않았다(빈 값).
소스만 읽은 설계 노트이며 테스트·빌드는 실행하지 않았다. 근거는 `issueops-04.md`와 현재 소스 재확인이다.

## 1. 현재 동작 (재확인 앵커)

- 배선: `cmd/issueops/issueopsapp/hook_facade.go:15`가 `projectdoc.DiscoverProjectDocs`를 `app.CatalogService.Discover`에 연결한다. production 호출자는 이것 하나다.
- `internal/adapter/projectdoc/catalog.go`
  - 상수 `:12-17`: 항목 64, raw 128, 파일 256KiB, 합계 2MiB.
  - `readProjectDocCatalogEntries :92-98`은 `dir.ReadDir(128)`을 한 번만 호출한다. 디렉터리 순서의 128개를 먼저 자르고 `sort.Slice :88`은 그 뒤에 돈다(cap-before-sort). 항목이 128개를 넘으면 선택 집합이 파일시스템 순서에 좌우된다.
  - `readProjectDocCatalogFile :100-123`은 파일 전체(최대 256KiB)를 `io.ReadAll`로 읽는다. 쓰는 값은 frontmatter `description`과 첫 H1뿐이다.
  - `:76`의 `ParseFrontmatter`(`internal/domain/projectdoc/meta.go:37`)는 전체 `Split`을 하고, `firstMarkdownTitle :150`이 같은 본문을 다시 파싱한다.
  - 생략은 모두 조용하다. oversize, 읽기 실패, 상한 도달 모두 `continue`/`break`이며 호출자는 개수를 모른다.
  - 필수 문서 우선순위가 없다. `requiredProjectDocNames`(`internal/domain/projectdoc/constants.go:5-17`, 11개)와 `optionalProjectDocNames`(`:19`, VCS.md·DESIGN.md)는 선택에 쓰이지 않는다. 앞쪽 `.md` 64개가 자리를 채우면 ARCHITECTURE.md가 빠질 수 있다.
- 최악 body read는 합계 상한 때문에 2MiB다(240KiB급 파일 8개).
- 유지할 안전 불변식: `os.OpenRoot :44`, `.issueops` symlink 거부 `:49-52`, `Type()` 필터 `:68`, `Lstat`+`fstat` regular 검사 `:101-112`.
- 기존 테스트(`catalog_test.go`): Skips Symlink/NonRegular(49), Rejects Symlinked Dir(70), BoundsEntriesAndContent(85-108), CapsRawDirectoryScan(110-130), UsesFrontmatterThenCanonicalMeta(12).
- 소비자: `internal/application/hookprompt/catalog.go:16-25`(`Build`), 렌더러 `internal/adapter/hookprompt/catalog.go:8`, `FormatProjectDocCatalog catalog.go:128`, DTO `internal/contract/hookprompt/types.go:13-18`. Omo 확장이 이 DTO를 `--json`으로 읽는다.
- context-only 유지: SessionStart 단독 정의 `internal/adapter/claude/install_hooks.go:86`, `internal/adapter/codex/install_hooks.go:79`(timeout 5). I7은 hook 정의·설치를 건드리지 않는다.

## 2. 결정: 최소 bounded 알고리즘

1. **이름 단계(본문 IO 0)**: `ReadDir(128)`을 EOF까지 배치 반복한다. 누적 raw 항목 상한은 `maxRawScan=1024`다. `Type()` 필터를 통과한 `.md` 이름만 모은다(메모리 ≤1024 문자열). 상한에 닿으면 `ScanTruncated`로 기록한다.
2. **결정적 선택**: 모은 이름을 정렬한 뒤 우선순위로 고른다. 필수(11) → 선택(2) → 나머지 사전순이고 합계는 64까지다. 필수는 64 cap에 밀리지 않는다(13<64). 최종 출력은 기존대로 `RelPath` 정렬이다. 순회 순서와 무관해져 1024 이하 디렉터리에서는 cap-before-sort가 사라진다. 1024 초과는 `ScanTruncated`로 표시하며 이 경우만 비결정이 남는다. 대안: 전체를 `ReadDir(-1)`로 읽기는 메모리가 디렉터리 크기에 비례하므로 채택하지 않는다.
3. **헤더만 읽기**: 선택된 파일마다 `Lstat`→`Open`→`fstat`을 하고 `Size()>256KiB`면 0 byte로 oversize 생략한다(기존 의미 유지). 통과하면 최대 `headerBytes=8192`만 읽는다. 잘린 헤더는 마지막 `\n` 뒤 부분 줄을 버려 UTF-8 경계를 보호한다. 기존 `ParseFrontmatter`와 `firstMarkdownTitle`을 헤더 문자열에 그대로 재사용하므로 domain 파일은 수정하지 않는다.
   - 동작 변화: frontmatter가 8KiB 안에서 닫히지 않으면 canonical fallback을 쓰고, H1이 8KiB 밖이면 Title이 빈 값이다. 대안으로 16KiB는 read 상한 1MiB로 늘어난다. 8KiB를 권장한다.
4. **합계 상한 폐기**: 64×8KiB=512KiB가 구조적 상한이므로 `totalBytes` 로직과 `projectDocCatalogMaxTotalBytes`를 제거한다.
5. **생략 수**: `ProjectDocCatalogOmissions{Oversize, Unreadable, OverCap int; ScanTruncated bool}`를 신설한다(`internal/domain/projectdoc/catalog_types.go`). 이 구조체는 JSON 직렬화 대상이므로 snake_case tag를 붙인다. `DiscoverProjectDocs` 시그니처는 유지하고(기존 테스트·호출 보존) 새 `DiscoverProjectDocsReport(repoRoot) (entries, omissions, stats)`를 두며 기존 함수는 이를 감싼다.
   - `CatalogService.Build`는 새 선택 필드 `DiscoverReport`가 있을 때 `ProjectDocCatalogContext.Omitted`(`omitted,omitempty`, additive)를 채운다. 없으면 현 동작이다. 호출자 `hook_facade.go`가 이를 연결한다.
   - 모델용 compact 끝에 생략이 있을 때만 `; (N omitted)`를 붙인다. 생략이 없으면 출력은 바이트 동일해야 한다. user view도 같은 조건에서 한 줄을 추가한다.

## 3. 측정 가능한 증명

- 결정적 연산 수: 내부 `catalogStats{FilesOpened, BytesRead, DirBatches}`를 반환한다. 시계·sleep은 쓰지 않는다.
- 새 테스트(adapter `catalog_test.go`):
  - `TestDiscoverReadsBoundedHeader`: 240KiB 파일 9개를 만들어 `BytesRead ≤ 9*8192`임을 단언한다. 현재 코드는 같은 입력에서 약 2MiB를 읽는다(증명 대비값).
  - `TestDiscoverSelectionIndependentOfDirOrder`: 이름 200개를 서로 다른 생성 순서로 두 번 만들어 선택 집합이 같음을 단언한다.
  - `TestDiscoverRequiredDocsSurviveCap`: `a-00..a-69.md` 70개와 `ARCHITECTURE.md`를 만들면 ARCHITECTURE가 포함되고 OverCap이 올바르다.
  - `TestDiscoverReportsOmissions`: oversize 1, 비정상 읽기 1, cap 초과를 단언한다.
  - `TestDiscoverSymlinkRootConstraintsUnchanged`: 기존 symlink 테스트를 그대로 통과한다.
- 선택 벤치마크: `BenchmarkDiscoverProjectDocs`에 `b.ReportAllocs()`를 쓰고 B/op 전후를 기록한다. 결과는 성능 보증이 아니라 참고다.

## 4. 기존 테스트와의 모순 (보존)

- `TestDiscoverProjectDocsBoundsEntriesAndContent`(`catalog_test.go:85-108`)의 마지막 단언 "8개 후 합계 상한으로 중단"은 합계 상한을 가정한다. 새 설계에서는 240KiB 파일 9개가 모두 수용되므로 이 단언은 의도적으로 바꿔야 한다(읽기 byte 상한으로 대체). 약화가 아니라 불변식이 "내용 합계"에서 "읽은 byte"로 바뀐 것이며, 승인 여부는 reducer가 결정한다.
- `TestReadProjectDocCatalogEntriesCapsRawDirectoryScan`(110-130)은 단일 `ReadDir(128)`을 pin한다. 배치 순회로 바뀌면 이 헬퍼의 시그니처·의미가 변하므로 `maxRawScan`을 검증하는 테스트로 교체한다.
- 미결: `ProjectDocCatalogContext`에 `omitted`를 추가하면 response-contract golden(`cmd/issueops/contractgolden`, `cmd/issueops/issueopsapp`의 `TestResponseContractsGolden`)에 영향이 있는지 확인이 필요하다. 다른 노드(I4 등)가 같은 golden을 건드리면 충돌한다. 아직 golden 파일을 열어 확인하지 못했다.

## 5. 파일 소유권 (I7 단독)

- 쓰기: `internal/adapter/projectdoc/catalog.go`, `catalog_test.go`, `internal/domain/projectdoc/catalog_types.go`, `internal/contract/hookprompt/types.go`, `internal/application/hookprompt/catalog.go`와 `catalog_test.go`, `internal/adapter/hookprompt/catalog.go`, `cmd/issueops/issueopsapp/hook_facade.go`.
- 조건부: contract golden(위 미결). 다른 노드와 겹치면 reducer가 순서를 정한다.
- 비수정: `meta.go`, `constants.go`(필수·선택 목록은 `ProjectDocNames()`/`OptionalProjectDocNames()`로 읽는다), install/hook 정의.

## 6. 의존성

I7은 I1~I6, I8~I10과 코드 의존이 없다. 유일한 접점은 contract golden과 `DTO` 추가 여부다. API/DTO 변경이므로 `.issueops/OPEN_API_SPEC.md`의 api-doc 게이트 해당 여부를 구현 단계에서 확인한다.

## 7. 검증 명령

```bash
gofmt -l internal/adapter/projectdoc internal/domain/projectdoc internal/application/hookprompt internal/adapter/hookprompt internal/contract/hookprompt cmd/issueops/issueopsapp
go vet ./internal/adapter/projectdoc/... ./internal/application/hookprompt/... ./internal/adapter/hookprompt/...
go test ./internal/adapter/projectdoc/... ./internal/application/hookprompt/... ./internal/adapter/hookprompt/... ./internal/domain/projectdoc/... -count=1
go test ./cmd/issueops/hookcli/... -count=1
go test ./cmd/issueops/contractgolden -run Golden -count=1
go test ./cmd/issueops/issueopsapp -run TestResponseContractsGolden -count=1
go test ./internal/adapter/projectdoc -run xxx -bench DiscoverProjectDocs -benchmem -count=1
git diff --check -- .issueops/plans/agent-improvements-2026-10-02/catalog-read-map.md
```
