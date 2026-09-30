# 공개 추적 문서 경로 정규화 검증

이슈: https://github.com/m16khb-org/issueops/issues/527
사이클: `io-a7461598e205` · 브랜치: `527-public-material-paths` · generation 2

## 의도 대조

공개 plan·intent·spec·plan-review의 최초 생성과 전이 재생성에서 경로를 정규화한다. 봉인 원본과 resume 계약은 보존한다. record fallback, 외부 tracked plan skip, 동일 bytes no-op, 기존 review URL 가림을 검사한다. 새 정규화의 URL 보존에는 root 뒤 줄 번호로 URL cursor가 정체된 재현과 apostrophe query 회귀를 포함한다.

## 실행과 증거

- 기존 전이·외부 plan 보호 baseline이 통과했다. 새 전이 테스트는 원본 경로가 공개 plan/intent/spec에 남는 assertion으로 실패했다. 초기 fixture의 stale review digest 오류는 테스트 입력과 digest를 맞춘 뒤 다시 실행해 실제 RED를 확보했다.
- domain·application·adapter·architecture 검사가 통과했다. 초기 fallback fixture의 타입 오류와 새 함수의 architecture inventory 누락을 수정했다.
- 게이트 spec의 regex 구분자와 ledger 구분자가 충돌해 G1은 같은 세 패키지 전체 검사로 등록했다. 수용 범위는 줄이지 않았다.
- 최종 검증과 독립 리뷰의 실제 판정은 `artifact/evidence/`와 IssueOps record에 남긴다. 발행 후 CI와 remote readback은 같은 디렉터리의 완료 증거로 보존한다.

## Side effect

`TrackedMaterials.contents`가 완성한 공개 사본 bytes만 정규화한다. 파일 mode, warning, 외부 plan skip과 no-op 쓰기는 기존 Write가 담당한다. 원본 file·mode·hash와 record·owner prompt·resume token을 고치지 않는다. source checkout·다른 worktree·전역 설치·host 설정에는 쓰지 않는다.

## 성능

합성 문서 1,560 bytes는 63,530 ns/op, 1,560,000 bytes는 88,938,830 ns/op였다(Apple M1 Pro, GOMAXPROCS=4). 실제 결과는 `artifact/evidence/benchmark-delta.txt`에 있다. 문서 생성 경계에서만 변환하며 추가 파일 관측·원격 조회·cache를 만들지 않는다. 대형 문서에서 처리량은 17.54 MB/s였다. 이전에는 복사만 수행했으므로 성능 개선이라고 해석하지 않는다.

## 정리와 문서

최종 diff에서 dead code·중복·불필요한 추상화·경계 위반을 찾지 못했다. 진행 중 텍스트와 검증 예정이라는 임시 문구를 관측 결과와 증거 경로로 바꿨다. 코드의 불변식을 설명하는 주석은 유지했다. 추가 코드 정리는 필요하지 않았다.

변경 Go 파일의 추가 nonblank 228줄 중 주석은 4줄이다. 근사 SNR은 최초 정리 전 0.9826, URL 경계 수정 후 0.9825이며 boilerplate 비율은 0.0263이다. 불변식 주석은 유지했고 코드 품질 개선을 주장하지 않는다. 측정 명령은 `python3 artifact/metrics.py artifact/evidence/metrics-before.json`이며 실제 경로는 이슈 폴더 기준이다. 임시 fixture는 테스트의 TempDir cleanup으로 삭제된다. 별도 서버·세션·컨테이너는 만들지 않았다. 공개 문서 정책은 `.issueops/CONVENTIONS.md`의 파생물 규칙에 반영했다.

독립 전체 리뷰는 compact JSON의 인접 필드와 공백 없는 Markdown 참조 URL에서 경계 결함을 지적했다. 세 입력 모두 실제 함수에서 RED를 확인했다. URL 시작 경계를 regex 안에서 확인하고 double quote에서 span을 끝내도록 수정해 전체 domain matrix가 GREEN이 됐다. 기존 apostrophe query와 root 줄 번호 회귀도 통과했다. 이전 코드의 진행 중 배터리는 취소했고 소유 프로세스와 자손의 종료를 확인했다. 수정 후 최종 배터리는 첫 항목부터 다시 실행한다. delta 리뷰와 최종 실행 결과는 `artifact/evidence/` 및 완료 record에 남긴다.

## 완료 상태

완료 판정은 최신 HEAD의 push/PR CI 성공, remote readback, execution complete/released 증거를 대조한다. 이 보고서는 공개 구현·검증 범위를 설명하고 최종 SHA와 원격 상태는 IssueOps 완료 record가 소유한다. merge/cleanup은 상위 조정 세션의 범위다.
