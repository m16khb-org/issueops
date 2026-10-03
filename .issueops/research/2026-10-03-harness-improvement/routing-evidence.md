# 문서 라우팅 구현·측정 증거

환경: Go 1.27.1, darwin/arm64, Apple M4. 같은 benchmark와 입력으로 변경 전후
각각 5회 실행했다. 다른 조사 에이전트가 실행 중이었으므로 wall time은
통제된 성능 보장으로 해석하지 않는다.

```sh
go test ./internal/domain/projectdoc -run '^$' \
  -bench '^BenchmarkRouteDocsForTask$' -benchmem -benchtime=100ms -count=5
go test -race ./internal/domain/projectdoc ./internal/application/projectdocs -count=1
```

| 입력 | 전 ns/op 중앙값 | 후 ns/op 중앙값 | 전→후 B/op | 전→후 allocs/op |
|---|---:|---:|---:|---:|
| general | 4543 | 3875 | 1800 → 1704 | 57 → 51 |
| implement test performance refactor ci pr | 6619 | 7869 | 4192 → 2536 | 62 → 50 |
| 긴 Unicode 혼합 task | 51286 | 37790 | 27312 → 2536 | 68 → 50 |

긴 입력은 `strings.Repeat("문서검토 implementation-helper ", 32) + "design ci"`다.
할당량은 각 5회 모두 동일했다. 긴 입력의 B/op는 약 90.7%, 복합 입력은 약
39.5% 감소했다. 복합 입력의 시간 중앙값은 증가했으므로 모든 입력에서 빨라졌다고
주장하지 않는다. 순수 함수의 할당 감소와 반복 scan 제거가 검증된 효과다.

## 동작 검증

- 기존 라우팅 테스트, Unicode와 숫자 단어 경계, 구두점 구분, 복합 task의
  중복 제거와 첫 등장 순서 테스트가 통과했다.
- 두 패키지 race 검사 exit 0:
  `internal/domain/projectdoc` 1.520s,
  `internal/application/projectdocs` 1.890s.
- LSP 디렉터리 진단: 21개 파일, error 0.
- 전역 cache와 새 dependency는 추가하지 않았다.

최종 전체 검증과 독립 리뷰는 별도 완료 보고서에서 확인한다.
