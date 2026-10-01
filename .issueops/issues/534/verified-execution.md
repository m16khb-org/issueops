# #534 구현 검증 기록

Lifecycle: io-c63caa7ea2ee. Generation: 2. Branch: 534-selfverify-evidence.
Base: b917fc960b72bca3e616df1162c6b89d40a517ba.

## 의도 대조

네 named golden 테스트와 두 package의 실제 성공 이벤트를 확인하고, zero-match/skip/failure/invalid JSON을 실패로 판정한다. Binary drift는 정확히 한 binary_drift check의 healthy bool을 읽는다. 미빌드 skip, 무관한 doctor 경고, 실행 실패 및 timeout, generic runner와 doctor JSON exit code는 유지한다.

## 검증 근거

- RED: `/tmp/issueops-five-20261001/534/red.log`에서 no-match/skip/fail의 거짓 성공 및 실제 stale doctor의 거짓 성공을 재현했다.
- GREEN: `go test ./internal/application/selfverify ./cmd/issueops/selfworkflow/steps -count=1`이 통과했다. 실제 subprocess fixture와 missing/duplicate/malformed/null/wrong bool 및 JSON execution evidence 검사를 포함한다.
- 원장: G1은 focused 회귀 검사, G2는 네 실제 golden 테스트의 run/pass 및 두 package pass를 검사한다.
- 최종 battery와 독립 리뷰 결과는 같은 revision의 외부 evidence bundle에 보존한다. 게시·CI·execution 완료 영수증은 durable record 및 owner 최종 보고가 소유한다.

## Side effect

- 이 worktree에 구현·회귀 테스트·testing 문서·IssueOps 추적 자료를 추가한다.
- fixture는 t.TempDir와 격리된 ISSUEOPS_STATE_DIR를 사용하고 종료 시 Go testing이 제거한다.
- 이 issue branch를 push하고 draft PR을 게시하며 IssueOps record를 기록한다.
- 전역 설치·설정, source checkout 구현, main 병합, cleanup은 이 owner 범위 밖이다.

## 성능

성공한 full-suite의 golden 재사용 경로는 추가 subprocess 0회다. fallback은 두 package를 한 번의 좁은 go test -json으로 실행한다. 기존 doctor 실행 1회의 출력만 파싱한다. focused 실제 fixture를 포함한 GREEN 실행은 13.810초였다. hot path 성능 개선을 주장하지 않는다.

## 정리와 위험

정리 범위는 이번 diff다. 실제 관측 대상이 root/bin/issueops임을 주석에 반영하고 generic adapter를 유지했다. JSON 파싱 helpers는 application 내 결과 해석에만 사용한다. 출력이 잘린 경우 성공을 주장하지 않는다. 네 테스트의 좁은 실행은 기존 32KiB runner budget 안에서 확인한다.

## 승인 종료점

Draft PR 게시, 최신 pushed HEAD의 push/PR CI 성공, execution complete/released까지 진행한다. 상위 조정 세션이 main 병합·cleanup·io-update를 수행한다.

G2의 원래 /tmp verifier 경로는 workspace 정책이 거부했다. 외부 파일 접근 없이 동일한 test/run/package-pass/fail-skip 검사를 `python3 -c` CHECK로 원장에 넣어 재실행했다. EXPECT나 성공 조건은 완화하지 않았다.

정리 측정은 변경된 Go 파일과 미추적 evidence helper/test를 포함했다. 근사 SNR 0.997→0.997, 동일 7-line block 0→0, boilerplate ratio 0.021→0.021이었다. Golden 판정의 근사 branch 수는 17→15로 줄었다. 나머지 분기는 JSON 스트림·필수 테스트·package 검증이며 별도 추상화 없이 유지했다. 이 수치는 전역 코드 품질이나 런타임 성능의 증거로 사용하지 않는다.

첫 전체 race 검사는 verifyloop의 두 테스트에서 fake doctor가 빈 stdout을 반환해 실패했다. 같은 두 테스트로 RED를 재현한 뒤, 성공 fake에 명시적인 binary_drift healthy JSON을 넣었다. 테스트의 실패 주입 결과는 그대로 유지했다. 새 focused 검사와 최종 battery를 처음부터 다시 실행하며 실패 run의 부분 통과는 완료 증거로 재사용하지 않는다.

Root가 새 production source의 DDD inventory 누락을 알려 줬다. `TestDDDResponsibilityInventoryMatchesSource`의 실패를 먼저 확인하고 정규 `-update-ddd-inventory` 생성기로 evidence.go와 두 심볼을 application/T18에 등록했다. 생성 diff는 해당 항목 9줄뿐이다. `go test ./internal/architecture ./cmd/issueops/selfworkflow/verifyloop -count=1`이 통과했다. G1의 CHECK에는 이번 수정으로 영향받은 두 패키지도 포함했다. 원장은 기존 EVIDENCE를 외부에 보존한 뒤 다시 생성하여 실제 CHECK를 실행한다.

공유 호스트의 동시 full race 부하를 해소하기 위해 이전 battery 종료와 자손 회수를 확인해 heavy-quiesced.json에 기록했다. 최종 battery는 상위 조정 세션의 serialized-verification.py로 순차 실행하며, 해당 프로세스만 GOFLAGS=-p=8 및 GOMAXPROCS=8을 사용한다. 제품 timeout과 전체 검증 범위는 유지한다. 최종 runner는 미추적 Go 파일까지 gofmt에 포함하고 vet, 전체 race, native/Linux lint, local build, 실제 self-verify를 같은 변경 fingerprint에서 실행한다.
