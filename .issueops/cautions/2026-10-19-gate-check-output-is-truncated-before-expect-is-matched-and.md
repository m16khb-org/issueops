---
name: 2026-10-19-gate-check-output-is-truncated-before-expect-is-matched-and
description: Caution record for a solved false case or recurring risk.
---

# Gate CHECK output is truncated before EXPECT is matched, and gates check without --file runs every ledger

- Date: 2026-10-19
- Kind: `caution`
- Source: issueops-570
- Summary: 게이트 CHECK 명령이 긴 출력을 낸 뒤 기대 문자열을 찍으면 출력이 잘려 expect not matched가 되고, --file 없는 gates check는 저장소의 모든 원장을 실행한다.
- Context: #570에서 G7(go test ./...)과 G8(go test -race ./...)이 python 래퍼로 test-ok와 race-ok를 찍도록 적혀 있었다. gates check가 두 게이트를 기대 문자열 불일치로 판정했는데, 같은 명령을 직접 실행하면 종료 코드 0이었다. --file 없이 실행한 첫 gates check는 .issueops/issues 아래의 다른 이슈 원장까지 모두 실행해 총 게이트가 314개였다.
- Resolution: 임시 원장에서 3000줄을 출력한 뒤 기대 문자열을 찍는 CHECK를 실행해 확인했다. 하네스는 출력 앞부분만 보존하고 뒤를 자르므로(check_error에 <truncated>) 꼬리의 기대 문자열이 사라진다. 하위 프로세스 출력을 capture_output=True로 받고 판정 문자열만 찍도록 CHECK를 고쳤다. 게이트는 항상 --file로 이 사이클의 원장만 지정해 실행한다.
