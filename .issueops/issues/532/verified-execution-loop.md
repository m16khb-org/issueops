# #532 실행 검증 보고

Cycle `io-7426b49dc042`, branch `532-policy-execution-bounds`, generation 2.

## 의도 대조

- 부모·자식 handshake 뒤 750ms timeout을 실행한다. 부모가 먼저 끝나도 상속 pipe를 포함해 1250ms 안에 반환하고, 자식의 예정된 완료 파일이 생기지 않는지 확인한다.
- stdout·stderr 각각 8MiB 및 32MiB를 동시에 출력한다. 스트림별 보관량이 64KiB이고 출력량에 비례한 할당이 사라지는지 확인한다.
- 가짜 token/password/credential/URL userinfo와 UTF-8이 수집 경계에 걸릴 때 불완전한 마지막 줄을 버린다. application에서 redaction한 뒤 기존 32KiB 응답 상한과 `<truncated>` 표시를 유지한다.
- 실제 정상·비정상·start failure·timeout·정책 거부와 env allowlist를 검증한다. 공개 DTO와 JSON schema는 바꾸지 않는다.

## RED → GREEN

기존 adapter에서 부모 생존 및 부모 조기 종료 모두 약 2.02초 뒤 반환했고 자식의 완료 파일이 생겼다. 조기 종료 사례는 `TimedOut=true`인데 error가 nil이었다.

동일 flood fixture의 측정값(두 스트림 합계):

| 출력량(각 스트림) | 이전 보관량 | 이전 TotalAlloc 증가 | 수정 뒤 보관량 | 수정 뒤 TotalAlloc 증가 |
|---|---:|---:|---:|---:|
| 8MiB | 16MiB | 83,895,944 bytes | 131,072 bytes | 397,392 bytes |
| 32MiB | 64MiB | 335,548,880 bytes | 131,072 bytes | 402,728 bytes |

독립 리뷰가 지적한 실행 직전 deadline 경계도 확인했다. 1ns timeout과 존재하지 않는 실행 파일을 조합한 RED 테스트는 start error로 실패했고, 시작 전 context 확인을 복원한 뒤 deadline error로 통과했다. 공개 실행 결과의 timeout exit code 124도 유지했다.

수정 뒤 wall time은 각각 14ms와 45ms였다. 이는 프로세스 시작과 drain을 포함한 한 번의 측정이며 RSS/OOM 측정이 아니다. drain IO는 O(N), 보관과 redaction은 O(K)다.

## 검증과 재현

`policy_run_bounds_test.go`에 실제 Go helper-process 시나리오를 둔다. helper argv를 사용해 ambient 환경변수 없이도 실행하며, 정상 출력과 일반 exit code를 기존 계약과 비교한다.

- G1: policy adapter/application/domain 테스트
- G2: policy adapter/application race
- G3: architecture fitness ratchet
- G4: CLI/MCP response golden

명령과 성공 출력은 같은 폴더의 `gates.md`를 따른다. 실패 원문과 전후 측정은 owner evidence bundle에 보존한다. 첫 gates 실행은 env allowlist에 HOME을 빠뜨려 Go cache 탐색이 실패했다. HOME을 명시적으로 허용했다. architecture 검사는 신규 심볼 책임 inventory 누락으로 실패했으며, 생성기로 policy adapter와 application 심볼만 갱신했다. 원장 전체를 다시 실행해 G1–G4가 모두 통과했다. 이는 production env 정책을 바꾸는 수정이 아니다.

병렬 full race 실행이 겹친 이전 최종 battery는 risk QA의 기존 600초 제한으로 실패했다. 실패 원문과 실제 프로세스 종료 증거를 보존했다. 검증 범위와 제품 timeout은 유지하며, 최종 battery만 root의 직렬 실행 wrapper를 사용한다. 이 실행에만 GOFLAGS=-p=8, GOMAXPROCS=8을 적용한다.

저장소 최종 battery와 독립 diff review, PR 및 CI는 실행 완료 영수증과 owner 결과 파일에 기록한다. 이 문서는 결과를 미리 pass로 표시하지 않는다.

## Slop 정리

중복된 completion 대기 루프를 하나로 합쳤다. 바깥 deadline 채널을 nil로 바꿔 timeout signal을 한 번만 처리하고, grace 뒤 reader를 닫아 copy goroutine을 회수한다. 정상 종료·비정상 종료·pipe drain 계약을 유지한다.

전체 변경 Go 파일의 비어 있지 않은 줄을 비교한 근사 측정에서 runner의 SNR은 0.929 → 0.936이었다. 반복된 동일 줄 수는 23 → 23이다. 이 수치는 주석과 동일 줄을 세는 근사치이며 코드의 의미나 실제 중복 블록 비율을 측정하지 않는다.

## Side effect와 범위

Unix에서는 새 process group을 만들고 deadline에 SIGKILL을 보낸다. 같은 그룹을 벗어난 프로세스까지 종료하는 sandbox는 아니며, 그 경우에도 reader 대기는 250ms 유예 뒤 끝낸다. non-Unix fallback은 직접 프로세스 종료와 pipe 대기 상한만 제공한다.

추적 파일은 policy 실행 경계·직접 회귀 테스트·운영 문서·이슈 자료다. 전역 설치·다른 네 작업·main 병합·cleanup은 이 owner가 수행하지 않는다. rollback은 이 PR revert다.

Cleanup receipt: 직접 부모는 `cmd.Wait`로 회수하고 stdout/stderr reader는 두 completion을 모두 수신한 뒤 반환한다. 테스트의 임시 파일은 `t.TempDir`이 제거한다. daemon, server, browser는 생성하지 않는다.

Verification mode: 프로세스 종료와 secret 노출 경계를 다루므로 실제 실행 및 race 검증을 사용한다. 독립 review는 `devils-advocate-review` 패턴으로 수행한다. Hooks는 이 작업이나 상태 전이를 수행하지 않는다.
