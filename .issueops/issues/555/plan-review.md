# 계획 검토 기록

## 1차: 수정 요청

- 이슈 완료 기준은 Claude 등록 단계에서 Explore를 제외한다고 적었지만 계획은 훅 안에서 제외하고, G5는 general-purpose만 확인해 제외 동작과 카탈로그 동일성을 원장 증거로 남기지 못한다.
- 로컬 self-verify의 native integration 검사는 실제 HOME의 hooks.json을 새 기대 설정과 비교하므로 SubagentStart가 없는 기존 설치에서 실패하는데, 계획에 self-verify 증거를 얻는 방법과 SessionStart만 담은 성공 픽스처 갱신이 없다.
- 롤백 시 이전 바이너리의 activation readback이 예상 밖 issueops 그룹에서 먼저 실패하므로, 설정 파일에서 subagent-start 그룹을 먼저 지운 뒤 io update를 실행하는 순서를 적어야 한다.
- worktree에서 원본 checkout을 찾는 기능은 install.ResolveStableNativeRoot에 이미 있어, 레포 이름을 구하는 새 gitdir 파서를 만들면 같은 판정을 두 곳이 소유하게 된다.
- 추적 템플릿 configs/codex/hooks.json·configs/claude/hooks.settings.json, 최상위 usage와 usage golden, state-policy-and-hooks.md 42행, bootstrap 워크플로 렌더 문구, README 두 개와 stability-audit 스킬의 SessionStart 단독 서술이 범위에서 빠졌다.

## 2차: 수정 요청

- 2차 delta 리뷰: D1과 D3는 해결됐다. 새로 넣은 G11은 verify 단계에서 커밋과 push가 없어 통과할 수 없고, 원장 미완료로 사이클이 verify에 갇힌다.
- 레포 이름 fallback을 application 계층에서 path/filepath로 처리하라는 지시는 application_must_not_import_implementation 아키텍처 규칙을 어긴다.
- SessionStart 하나만 등록한다는 규범 문장이 conventions·operations·CAUTIONS·architecture 문서와 설치기 주석에 더 남아 있는데 T5 목록과 G10 정규식에서 빠졌다.

## 3차: 수정 요청

- 3차 리뷰는 claude-opus-5-5에 effort xhigh로 올려 실행했다(3라운드 상승 규칙). N1과 N2는 해결됐고 G10 범위 안의 N3도 해결됐다.
- child-host smoke 스크립트(scripts/verify-child-host-smoke.sh)가 관리 이벤트 집합을 SessionStart 하나로 고정하고 subcommand를 session-start로 하드코딩해, 설치기가 SubagentStart를 쓰면 smoke가 항상 실패한다. 스크립트 테스트 픽스처와 testing 문서 70행, G10 검사 범위(scripts, .issueops/testing)도 함께 빠졌다.

## 4차: 수정 요청

- 4차 리뷰는 claude-opus-5-5에 effort xhigh로 실행했다(3라운드 이후 상승 규칙). child-host smoke 스크립트의 R1 수정은 해소됐다.
- smoke 테스트의 source 활성화 픽스처 writeManagedSurfaceFixture가 SessionStart만 쓰므로, 계획대로 스크립트 contract만 넓히면 변경 전 검사가 모든 시나리오에서 실패해 G11이 통과할 수 없다. 리뷰어가 저장소 밖 복사본에서 재현했고 이 픽스처를 고치면 통과함을 확인했다.

## 5차: 통과

- 5차 리뷰는 claude-opus-5-5에 effort xhigh로 실행했다(3라운드 이후 상승 규칙). smoke 테스트의 두 픽스처(fakeInstallScript, writeManagedSurfaceFixture)를 모두 고치도록 한 수정이 반영됐다.
- 리뷰어가 저장소 밖 복사본에 계획의 smoke 변경만 적용해 G11 세 조건(subagent-start 계약 2건, session-start 하드코딩 0건, ChildHostSmoke 테스트 통과)과 hostprobe 패키지 전체 테스트 통과를 확인했다. source 픽스처 수정만 되돌리면 테스트가 실패해 그 수정이 필요하다는 것도 확인했다.
- 스크립트의 dry-run·digest·Claude 계측 단계와 Go 쪽 Codex smoke 투영은 관리 이벤트 집합 변화에 영향을 받지 않아, 계획 범위 밖에서 G7이나 G11이 깨질 지점은 없다.
