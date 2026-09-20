---
name: 2026-09-20-native-host-probe-evidence-and-bounded-output
description: Caution record for a solved false case or recurring risk.
---

# Native host probe evidence and bounded output

- Date: 2026-09-20
- Kind: `caution`
- Source: dogfood regression repair
- Summary: 설치·mock 통과만으로 실제 호스트 검증기의 출력 형식과 hook 증거를 인증하면 안 된다.
- Context: Codex 0.155.1, Claude Code 2.1.272, Omo 5.0.0-0.beta.22의 실제 9-case 검증에서 경고 오집계, 배열 응답, model 없는 SessionStart, null diagnostics와 긴 JSONL 출력 때문에 완료 판정이 모두 실패했다. ExecRunner의 embedded bytes.Buffer.ReadFrom이 Write의 출력 제한을 우회하는 문제도 실제 subprocess 회귀 테스트로 확인했다.
- Resolution: 알려진 Codex hook-trust 안내만 도구 수에서 제외하고 다른 오류는 거부한다. Claude 배열 응답은 content로 정규화하며, private SessionStart 영수증과 stream의 실제 모델 증거를 별도로 요구한다. 실패 hook는 성공으로 기록하지 않는다. 정상 null diagnostics는 빈 배열로 정규화한다. Omo는 표시용 message_update/tool_hook_status만 즉시 제외하고 개별 이벤트와 보존할 증거 모두에 크기 제한을 둔다. 프로세스 출력 Writer에서 ReadFrom 우회를 차단한다. 잘못된 JSON·초과 출력·추가 도구 호출·영수증 부재는 계속 거부한다.
- Evidence:
  - internal/adapter/hostprobe/runner_test.go
  - internal/adapter/hostprobe/process_group_unix_test.go
  - internal/adapter/hostprobe/omo_output_test.go
  - internal/adapter/toolconformance/benchmark_test.go
  - cmd/issueops/hookcli/hook_catalog_test.go
  - go test -race ./internal/adapter/hostprobe ./internal/adapter/toolconformance ./cmd/issueops/hookcli ./internal/adapter/projectdocs -count=1
  - 2026-09-20 candidate live conformance: 9 attempts, 9 completed, environment_failures=0, transport_failures=0, schema_drift_observations=0
