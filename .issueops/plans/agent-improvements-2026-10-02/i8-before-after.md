# I8 동일 workload 전후 측정

기준: 변경 전 HEAD `92eaa00143841f964c285dacdd41aedbeeecdf2e`의 임시 archive와
현재 작업 트리. 같은 외부 `go test -overlay` benchmark 파일을 양쪽에 적용했다.
저장소 파일을 바꾸지 않았고, 같은 record를 같은 Store.Update callback으로 갱신했다.
`GOMAXPROCS=1`, `-benchtime=200x -count=3`, 같은 Go toolchain과 장비를 사용했다.

실행: `mon_WXRAKNJKH1GCHY5E`, 양쪽 exit 0.

| observer | 변경 전 ns/op 3회 | 변경 후 ns/op 3회 | 전후 allocs/op |
|---|---|---|---|
| nil | 471536, 251792, 244318 | 323824, 286101, 262367 | 118 → 128 |
| noop | 257361, 247258, 242100 | 258506, 256448, 258489 | 121 → 135 |
| JSON actionable | 253080, 261329, 245459 | 267756, 260742, 256340 | 124 → 138 |

추가 할당은 각각 10, 14, 14회였다. 중간값은 nil 251792→286101 ns/op,
noop 247258→258489, JSON 253080→260742다. SQLite I/O와 공유 장비 부하의
변동이 있으므로 이 수치를 일반적인 지연 보장이나 속도 개선으로 해석하지 않는다.
이번 변경은 관측 정확성과 context 보존을 추가하며 추가 비용이 있다.

정확성은 별도로 실제 SQLite 테스트, 일반·race 테스트, 취소·가시성·CAS·경합
회귀 테스트로 검증했다. `mon_MXCWTN0ZH9RKNKVS`가 exit 0이며,
`mon_M7YY7JZEJBT70B2B`에서는 write 시작 epoch를 지운 변이를 새 테스트가 거부했다.

재실행용 임시 자료:
- 기준 source: `/tmp/issueops-i8-baseline.cLlVsW`
- 공용 benchmark: `/tmp/issueops-ten-improvements-01a0fa31/i8_comparison_test.go`
- 전후 overlay: 같은 디렉터리의 `i8_before_overlay.json`, `i8_after_overlay.json`
