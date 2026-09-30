# #521 품질 분석 제품 코드 선정 검증

- Lifecycle: io-dd786760d2a9, direct / Codex, generation 2
- Worktree: `/Users/habin/workspace/issueops.worktrees/521-quality-production-source-scope`
- Branch: `521-quality-production-source-scope`
- Base: `768546a219b082f5f7ad7f78f85664747eca9b74`
- 최종 HEAD, Draft PR URL, 완료 receipt는 이 보고서를 커밋한 뒤 durable execution completion에 기록한다. 보고서에 자신의 commit SHA를 넣지 않는다.
- 인계 자료와 계획 digest, 실행 선택 기록 및 native turn receipt를 대조했고 released generation 1의 exact replace → claim 체인으로 generation 2를 인수했다.

## 의도 대조

| 기준 | 관측 결과 | 증거 |
|---|---|---|
| tracked 및 nonignored untracked 제품 코드 보존 | tracked-but-ignored, 신규 코드, 공백·개행 파일명과 삭제 파일 경계를 검사했다. | `TestProductionSourceGitScope`, `TestProductionSourceExcludedRoot` |
| evidence/testdata/generated/symlink 제외 | 두 collector가 같은 fixture에서 같은 제품 파일과 SNR을 반환했다. 일반 generated 문구는 보존했다. | `TestProductionSourceGitScope`, `TestProductionSourceNonGitAndMissingGit` |
| non-Git, Git worktree, nested root 지원 | 일반 디렉터리와 Git 미설치 일반 디렉터리, `.git` 파일 worktree, nested root가 통과했다. | `TestProductionSourceGitWorktree`와 위 tests |
| 오류 표시 보존 | 손상 index, marked Git workspace의 Git 미설치, 파일 읽기 오류는 warning/error로 남았다. 정상 branch 부분 결과를 보존했다. stdout/stderr, 출력 제한 및 deadline도 검사했다. | `TestProductionSourceGitFailureDoesNotFallback`, `TestProductionSourceReadErrorPreservesPartialBranches`, `TestProductionSourceGitCommandBoundaries` |
| 실제 application/CLI 계약 유지 | 실제 composition root와 CLI 호출에서 제품 코드만 집계했고 손상 index의 collection error와 nonzero gate를 관측했다. | `TestQualityProductionSourceWiringAndCLI` |
| 실제 저장소 오염 제거 | canonical root에 ignored 재현 파일을 주입한 전후 새 분기 목록과 SNR이 동일했다. 기존 scanner는 재현 함수를 포함했다. | `/tmp/io-quality-dd786760d2a9/scanner-surface.json` |

## RED → GREEN → SURFACE → CLEAN

- RED: `go test ./internal/adapter/outbound/quality -run '^TestProductionSource' -count=1 -v`는 exit 1이었다. ignored/evidence/generated/symlink 포함과 Git 오류 은폐를 재현했다. 원 출력은 `/tmp/io-quality-dd786760d2a9/red.txt`에 있다.
- GREEN: 동일 named regression tests가 통과했다. 이후 추가한 읽기 오류, bounded Git 및 상대 root tests도 통과했다.
- SURFACE: 실제 CLI wiring 테스트와 canonical root 주입 관측을 수행했다. 주입 파일과 scanner helper를 제거하고 부재를 확인했다.
- 테스트 fixture에서 발견한 실패: 한 줄에 반복한 if 문에 구분자가 없어 coverage 빌드가 실패했다. 독립 `go test -cover`로 parser 오류를 재현해 fixture를 고쳤다. 이후 수집 성공과 repository health gate가 별개임을 확인하고 각각 단언했다. 제품 코드의 오류 표시를 완화하지 않았다.
- Architecture gate는 새 production helper의 책임 원장 항목 누락으로 실패했다. generator로 해당 파일의 adapter 책임과 다섯 symbol만 등록했다. dependency baseline이나 다른 책임 항목은 바꾸지 않았다.
- 독립 리뷰에서 제외 디렉터리 자체를 root로 전달하면 경계가 사라지는 결함을 발견했다. named RED를 저장한 뒤 Git에서는 가장 가까운 저장소 기준 root 경로, non-Git에서는 root의 제외 구성요소를 함께 검사했다. 정상 src 하위 root와 제외 경로 안의 별도 Git 저장소도 검증한다. 이전 최종 배터리는 중단했고 해당 프로세스 그룹의 종료를 확인했다. 부분 결과는 재사용하지 않는다.
- CLEAN: 단일 호출 `sourceCollectionError` wrapper를 제거하고 `errors.Join`을 직접 사용했다. 물리 줄 기준 heuristic SNR 0.9860 → 0.9860, 중복 7줄 window 0 → 0, 단일 호출 wrapper 1 → 0, boilerplate 비율 0.0919 → 0.0920이다. 이 값은 동작 기여도를 판단하는 수치가 아니다.

## 성능 관측

동일 canonical root에서 기존 scanner와 새 scanner를 한 번씩 실행했다. 기존 scanner는 1,242개 파일·5,105개 함수, 새 scanner는 1,238개 제품 파일·5,096개 함수를 관측했다. 기존 638ms, 새 745ms였고 새 scanner의 첫 관측은 960ms였다. 기존 값에는 ignored scanner helper와 주입 파일도 포함된다. 이 관측은 초기 수정 후 결과이며, 독립 리뷰 후 root 경계 수정의 전체 검증은 최종 battery에서 수행한다. 한 번의 관측이므로 성능 개선률을 주장하지 않는다. 두 collector가 각각 후보를 읽는 기존 실행 구조를 유지하며 cache나 공유 상태는 추가하지 않았다.

## Side effect와 경계

- 파일: 이 worktree의 quality adapter, 관련 테스트·책임 원장, 품질 기준 문서와 #521 산출물만 수정했다.
- Git: 파일 열거는 읽기 전용이다. 새 제품 파일을 tracked-only로 제한하지 않는다. Git metadata가 있는 경로에서 오류가 나면 filesystem fallback으로 ignored 파일을 다시 읽지 않는다.
- 원격: 승인된 이슈 브랜치 push와 Draft PR 발행만 수행한다. merge와 cleanup은 별도 경계다.
- Durable state: generation 2의 phase·검토·publication·completion 기록을 남긴다. 공유 설치와 source checkout은 수정하지 않는다.
- API/DTO/schema 및 DB 변경이 없어 API 문서·스키마 실측 gate는 해당하지 않는다.
- 롤백: 이 PR revert이며 runtime migration은 없다.

## 문서와 최종 검증

품질 기준은 `.issueops/operations/cli-and-mcp.md`에 반영한다. MCP route는 성공했으나 read가 module 경로를 거부해 직접 읽기·SHA 대조 후 최소 문단을 추가하는 fallback을 사용한다.

Focused gate 결과는 같은 폴더의 `gates.md`에 CLI가 기록한다. 최종 battery의 명령별 종료 코드와 전체 출력은 `/tmp/io-quality-dd786760d2a9/final-battery/`에 보존한다. 필수 항목은 gofmt, self-verify의 실제 test/build/golden/docs/inspect evidence, 그리고 self-verify가 실행하지 않은 Go vet/race 검사다. `--llm-eval=false`를 명시하며 전역 설치·업데이트는 수행하지 않는다. 최종 판정과 리뷰는 durable record 및 completion verification에서 확인한다.

Success criteria: 위 표의 포함·제외·오류·실제 표면 기준을 테스트와 직접 관측으로 대조했다.
Evidence artifact: gates.md 및 worktree 밖의 RED/GREEN/SURFACE/최종 battery 원 출력.
Cleanup receipt: 테스트 fixture는 t.TempDir로 회수했고 canonical 주입 fixture와 helper를 제거했다. QA 서버나 새 실행 세션은 생성하지 않았다.
Verification mode: 제품 파일 선정과 오류 계약을 바꾸므로 full verification 및 fresh-context 적대 diff review를 적용한다.
Skipped checks: API/DB/UI는 변경 대상이 없어 해당하지 않는다. 설치·업데이트는 사용자 지시에 따라 실행하지 않는다.

최종 결과: self-verify(seed 100, target 95, llm-eval=false)는 26/26 단계 통과, 최소 goal score 100, termination_eligible=true였다. 실제 risk QA 명령 `go test -race ./... -count=1 && go vet ./...`가 통과해 전체 Go 테스트와 vet를 함께 확인했다. gofmt 출력은 비었고 build·golden·docs·inspect 단계도 통과했다. 독립 diff review에서 발견한 root 경계 결함을 수정한 뒤 fresh-context delta review는 pass였으며 필수 잔여 finding은 없다. 최종 publication과 completion은 이 보고서를 커밋한 뒤 읽은 HEAD로 봉인한다.
