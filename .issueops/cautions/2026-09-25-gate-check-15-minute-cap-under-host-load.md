---
name: 2026-09-25-gate-check-15-minute-cap-under-host-load
description: Caution record for a solved false case or recurring risk.
---

# Gate CHECK 15-minute cap under host load

- Date: 2026-09-25
- Kind: `caution`
- Source: issueops-docs io-34938e479083 (#514)
- Summary: 게이트 CHECK 한도는 명령 정책이 900초로 막으므로(--timeout-seconds가 그보다 크면 timeout_exceeds_15m으로 거부) go test ./...·race처럼 긴 게이트는 머신 부하가 높으면 실패 테스트 없이 check timed out으로 미충족된다. 부하가 가라앉은 뒤 같은 원장으로 gates check --write를 다시 실행하면 미충족·pending 게이트만 다시 돈다.
- Context: #514(io-34938e479083) 원장 1차 run이 1시간 52분 걸렸고 G10(go test ./... -count=1)과 G11(go test -race ./... -count=1)이 900초를 넘겨 미충족이었다. 같은 시간대 load average는 100~290(코어 8개)이었고, 원인은 사이클과 무관한 상주 프로세스였다. 원장 run 전 단독 self-verify도 내부 race 테스트의 10분 한도에 걸려 실패했고, 끊긴 go test의 테스트 바이너리가 부모 없이 남아 부하를 더했다.
- Resolution: 한도를 늘리지 않는다(정책이 거부한다). EXPECT를 완화하거나 게이트를 abandon하지 않는다. 부하(1분 평균, 다른 go 테스트 프로세스 유무)를 관측해 가라앉은 뒤 같은 원장으로 다시 실행한다. gates check는 미충족·pending 게이트만 다시 실행하므로 이미 충족된 게이트는 재실행되지 않는다(internal/adapter/gates/check.go needsRun). 1차와 재실행 사이에 코드가 바뀌지 않았음을 보고에 적는다. 시간 초과로 끊긴 실행이 남긴 go-build 테스트 바이너리는 cwd로 자기 워크트리 것임을 확인하고 종료한다. 부하 원인이 사용자 환경이면 사용자에게 알리고 판단을 받는다.
- Evidence:
  - internal/adapter/policy/policy_evaluate.go:74 addDeny("timeout_exceeds_15m")
  - internal/adapter/gates/check.go needsRun: !gate.Checked || EvidencePending
  - #514 원장 1차 run: 12/14 충족, G10·G11 check_error 'check timed out'
  - --timeout-seconds 2700 재시도: G10·G11 'check denied by policy: timeout_exceeds_15m'
  - 부하 1분 평균 8.3에서 재실행: G10·G11만 실행돼 26분 만에 충족, 최종 14/14
