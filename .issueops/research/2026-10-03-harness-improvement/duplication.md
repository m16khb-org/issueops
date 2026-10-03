# 반복 계산과 중복 조사

GPT-6 Luna 조사 결과를 메인 에이전트가 저장했다. 조사 노드는 편집 도구가 없어
본문을 반환했다. 아래 메커니즘은 변경 전 소스 기준이며 라우팅 변경의 실제
측정·테스트 결과는 [routing-evidence.md](routing-evidence.md)에 있다.

## 채택: 문서 라우팅의 요청 내 반복 계산

`internal/domain/projectdoc/route.go`의 기존 `hasTaskToken`은 토큰마다
`strings.FieldsFunc`로 같은 입력을 반복 분해했다. `appendRouteDocsUnique`도
각 add에서 이전 결과 전체로 seen map을 다시 만들었다.

개선 범위는 해당 순수 함수와 인접 테스트·benchmark다. 토큰과 중복 집합을
호출당 한 번 만들고 Unicode 경계, exact token, 첫 등장 순서와 reason을 보존한다.
전역 cache는 필요하지 않다.

```sh
go test ./internal/domain/projectdoc -run '^$' \
  -bench '^BenchmarkRouteDocsForTask$' -benchmem -benchtime=100ms -count=5
go test -race ./internal/domain/projectdoc ./internal/application/projectdocs -count=1
```

## 보류: policy built-in catalog 공유

`internal/domain/policy/catalog.go`의 `BuiltinCatalog`는 호출마다 command와
subcommand map을 만든다. `internal/application/policy/service.go`의 평가 경로와
`internal/adapter/policy/policy_catalog.go`의 광고 경로에서 호출한다.

그러나 `Catalog.Apply`가 map에 workspace override를 적용하므로 단순 공유는
workspace 간 오염을 만든다. 요청별 복제 또는 overlay 비용까지 측정하지 않았고,
사용자 workload의 병목이라는 근거도 없다. 이번 구현에는 포함하지 않는다.
workspace별 override는 계속 평가마다 읽는다.

## 근거 범위

Luna는 projectdoc, policy, application/policy, adapter/policy의 focused 테스트
통과를 보고했다. 메인 에이전트가 직접 재실행한 증거는 routing-evidence의
두 패키지 race 검사다. policy 경로는 수정하지 않았다.

공식 Go 진단 가이드: https://go.dev/doc/diagnostics.
프로파일은 관측 대상과 overhead를 구분해야 하며, microbenchmark를 실제
서비스 throughput 또는 메모리 절감으로 확대 해석하지 않는다.
