# 계획 검토 기록

## 1차: 수정 요청

- CI install에 stdio를 명시하는 방향은 install과 self-verify를 통과시키지만, 실패 원인을 user systemd 세션 부재로 단정한 것은 확인되지 않았다. 임시 HOME에 unit을 쓰는데 user manager는 자기 HOME에서 찾으므로 세션이 있어도 같은 실패가 날 수 있고, load가 systemctl 출력을 버려 원인이 로그에 남지 않는다.
- readability 후보는 이슈가 제기한 CPU 전용 회귀에 대한 결론이 빠졌고, 분리로 결론 낸 두 후보에는 설계 이슈도 방향도 없어 성공 기준 2를 채우지 못한다.

## 2차: 수정 요청

- 진단 출력과 CI의 stdio 지정이 한 커밋에 묶여 있어, CI에서 install을 부르는 유일한 곳이 stdio가 되면 PR CI에서 supervisor 경로가 돌지 않고 원인 확인 게이트의 증거가 생기지 않는다. 에러 문구 변경은 공개 JSON 계약·테스트 비교·비밀값 노출과 무관하고 유지 결론들은 근거를 갖췄다.

## 3차: 통과

- 3라운드는 claude-opus-5-5에 effort를 xhigh로 올려 빈 컨텍스트 세션에서 실행했다. 진단 커밋을 먼저 단독 push해 그 run이 끝난 뒤에만 stdio 커밋을 올리는 순서로 바뀌어, 실패 로그에서 원인 증거를 얻을 수 있게 됐다.
- 브랜치와 main의 최근 CI는 install 직전까지 통과하므로 진단 run도 install에 도달하고, concurrency 그룹이 ref 단위라 draft PR run이 브랜치 run을 취소하지 않는다. 하네스는 CI 상태를 gate로 읽지 않아 중간 실패가 이후 단계를 막지 않는다.

## 4차: 수정 요청

- 4라운드는 구현 중 사용자가 추가한 리뷰·재계획 상한 3→5(P3) delta만 claude-opus-5-5, effort xhigh로 빈 컨텍스트 세션에서 검토했다. 경계 정의(revise 다섯 번째 허용·여섯 번째 거부, regress 4회면 다섯 번째 허용)는 코드와 맞고, 3라운드부터의 effort 상승은 스킬 문장에만 있어 코드와 충돌하지 않는다.
- G14의 -run 패턴은 원장 spec 안에서 백슬래시 파이프가 리터럴로 해석돼 테스트를 하나도 실행하지 않고 통과한다. python으로 감싸고 실행된 테스트 이름과 상수 값까지 확인해야 한다.
- revise 상한 3은 2026-09-08 적대적 리뷰 처리량 ADR의 Decision (4)가 정했는데 대체 대상에서 빠졌다. 2026-07-02 ADR도 감사 기록과 사람 결정 에스컬레이션은 유효하므로 전체 대체가 아니라 부분 대체로 표기하고, 색인 행은 project_docs_revise로 고친다.
- README.en.md의 at most three unwaived revise, The fourth is refused 문장이 파일 목록과 G15 검사에서 빠졌다. side effect 문장은 3회에서 멈춰 있던 record만 풀린다고 좁혀야 하고, 작업 트리의 revise_round_cap_test.go는 다시 대입되는 written 때문에 SA4006 lint가 난다.

## 5차: 통과

- 5라운드는 4라운드 지적의 해소만 claude-opus-5-5, effort xhigh로 빈 컨텍스트 세션에서 delta 검토했다. G14 대체 검사는 세 패키지에서 여섯 번째 거부와 regress 경계 테스트를 실제로 실행했고, 상한 상수 5도 확인했다.
- 2026-09-08과 2026-07-02 ADR의 상한 숫자만 부분 대체하고 색인은 project_docs_revise로 고친다는 계획이 저장소 관례와 맞는다. README.en.md와 영어 금지 문구가 G15에 들어갔고, side effect 문장과 SA4006도 해소됐다.
- 권고: G16은 날짜만으로 찾으면 기존 superseded 행 때문에 헛통과하므로 행 머리의 날짜와 record 경로로 행을 특정해야 한다. 원장은 EVIDENCE가 모두 pending이므로 G1~G16으로 다시 만들면 된다.

## 6차: 수정 요청

생략: 설치된 main 빌드의 revise 상한이 아직 3이라 네 번째 비-waived revise가 거부됐다. 사용자가 2026-10-07에 이 사이클(#550)에서 상한을 5로 올리기로 했고 작업 트리에는 그 변경이 들어 있다. 이 waive는 지적을 넘기는 뜻이 아니다. 지적은 고치고 7라운드 delta 리뷰로 해소를 확인한다.

- 6라운드는 CI에서 드러난 pr-review rg 부재 수정과 G16·G17 보강만 claude-opus-5-5, effort xhigh로 빈 컨텍스트 세션에서 delta 검토했다. grep fallback은 명시 glob 네 개만 rg와 맞췄고 .git 디렉터리와 바이너리 파일을 빼지 않아, git checkout에서 reflog와 커밋 메시지 행이 정의 행을 15행 상한 밖으로 밀어낼 수 있다. -I와 --exclude-dir=.git을 더하고 fixture로 고정해야 한다.
- -wF 조합, 파일당 -m 3, 출력 형식은 BSD와 GNU grep에서 rg와 맞는다. skip이나 CI rg 설치보다 fallback이 낫고, 스킬 digest·golden에 걸리지 않으며, G16·G17은 실제로 검증한다. fail-fast 때문에 Python 뒤의 self-verify 단계도 이번에 CI에서 처음 돈다는 점을 계획에 적기를 권한다.

## 7차: 통과

- 7라운드는 6라운드 지적의 해소만 claude-opus-5-5, effort xhigh로 빈 컨텍스트 세션에서 delta 검토했다. grep fallback에 -I와 --exclude-dir=.git이 들어가 6라운드 CHECK가 CLEAN을 내고, 두 옵션을 지운 복사본에서는 새 단언이 바이너리와 .git 행을 각각 잡아 실패한다.
- BSD grep 2.6.0과 GNU grep 3.6 모두 .git, node_modules, 중첩 dist, 바이너리, min, lock 파일을 빼고 파일당 3행으로 끊는다. 예외 처리는 rg가 없을 때만 grep으로 넘어가며, rg가 있을 때의 동작은 그대로다. G17은 테스트 3개 실행을 확인한다.
- 비차단: grep fallback은 rg보다 느리지만 기호당 30초 상한이 있고 rg가 없을 때만 돈다. 이슈 디렉터리 최상위 plan.md 사본은 phase 전환 전까지 예전 내용이므로 docs·PR 단계 전에 맞춰야 한다.
