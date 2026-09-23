---
name: 2026-09-23-gates-init-spec-pipe-segment
description: Caution record for a solved false case or recurring risk.
---

# gates init spec은 따옴표 안의 pipe(`|`)도 segment 구분자로 자른다

- Date: 2026-09-23
- Kind: `caution`
- Source: project-docs-update; 2026-09-20 병렬 인계 도그푸딩 보고서 #509·#510과 HEAD e97d33cd 재현
- Summary: `issueops gates init`은 `--gate` spec을 따옴표와 관계없이 모든 `|`에서 나누기 때문에, CHECK에 넣은 `python3 -c` 코드나 EXPECT 정규식에 `|`가 있으면 원장 파일을 만들지 않고 거부한다.
- Context: 2026-09-20 병렬 인계 도그푸딩 사이클 #509와 #510은 각각 G4 CHECK에 Python 집합 합집합 연산자 `|`를 썼다가 첫 `gates init`에서 `created=false`, `gate_count=0`으로 거부됐고, 두 세션 모두 spec을 한 번씩 고쳐 다시 실행했다. `internal/adapter/gates/init.go`의 `renderGateSpec`은 spec을 `strings.Split(spec, "|")`로 나누며, 따옴표나 escape를 해석하지 않는다. `|` 뒤 조각은 `CHECK:`, `EXPECT:`, `EVIDENCE:` 중 어느 것으로도 시작하지 않으므로 `unknown segment` 오류가 나는데, 이 오류 문구는 `|`를 원인으로 지목하지 않는다. 따옴표 밖 셸 파이프(`git status | wc -l`)도 같은 `unknown segment` 오류로 끝나고, `&&`를 썼을 때만 나오는 `wrap the sequence in one script or python3 -c` 안내는 출력되지 않는다. 그 안내를 따라 `python3 -c`로 감싼 코드에 Python의 `|`를 쓰면 같은 분할에 다시 걸린다. `.issueops/CAUTIONS.md` 요약의 #484 항목은 따옴표 밖 제어 연산자만 다루므로, 따옴표 안의 `|`는 안전하다고 읽힐 수 있다.
- Resolution: spec 어디에도 리터럴 `|`를 넣지 않는다. 집합 합집합은 `set.union(a, b)`로 쓴다. 두 사이클은 이 방식으로 원장을 만들었고, HEAD e97d33cd 빌드로 만든 재현 원장도 `gates check`에서 `met`가 됐다. Python 코드 안의 다른 `|` 용법(비트 OR, dict 병합, 정규식 alternation)도 같은 분할에 걸리므로 `|`가 없는 형태로 바꾼다. EXPECT에 alternation이 필요하면 CHECK 안에서 `s in ('ok', 'pass')`처럼 판정해 `ACCEPTED` 같은 고정 토큰 한 줄을 출력하고, EXPECT에는 그 토큰을 적는다. 셸 파이프가 필요한 검사도 스크립트나 `python3 -c` 하나로 옮기되, 옮긴 코드에 `|`가 남지 않았는지 확인한다. `gates init`이 `unknown segment`로 거부하면 spec에 들어 있는 `|`부터 찾는다. HEAD e97d33cd 기준 파서에는 `|`를 escape하는 문법이 없다.
- Evidence:
  - .issueops/issues/509/dogfood-report.md, .issueops/issues/510/dogfood-report.md: 첫 `gates init`이 G4의 Python 집합 합집합 `|` 때문에 `created=false`, `gate_count=0`으로 거부된 기록과 `set.union(...)` 교정
  - internal/adapter/gates/init.go `renderGateSpec`: `parts := strings.Split(spec, "|")`
  - 재현(HEAD e97d33cd 빌드, 임시 git 디렉터리): `CHECK: python3 -c "print(sorted({'a'} | {'b'}))"` -> exit 1, `created=false`, `gate_count=0`, `unknown segment`
  - 재현: `EXPECT: /ok|pass/` -> exit 1, `unknown segment "pass/"`
  - 재현: `CHECK: git status | wc -l` -> exit 1, `unknown segment "wc -l"`; 비교용 `CHECK: true && true` -> `CHECK must be one argv command ... wrap the sequence in one script or python3 -c`
  - 우회 확인: `set.union({'a'}, {'b'})`를 쓴 CHECK -> `created=true`, `gate_count=1`, `gates check` state `met`, EVIDENCE `['a', 'b']`
  - 우회 확인: CHECK 안에서 판정해 `ACCEPTED`를 출력하고 `EXPECT: ACCEPTED` -> `gates check` state `met`
  - .issueops/CAUTIONS.md Universal summary #484 항목: 따옴표 밖 `&& || ; |`만 다룬다
