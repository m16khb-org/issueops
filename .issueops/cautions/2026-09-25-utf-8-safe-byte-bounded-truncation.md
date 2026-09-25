---
name: 2026-09-25-utf-8-safe-byte-bounded-truncation
description: Caution record for a solved false case or recurring risk.
---

# UTF-8 safe byte-bounded truncation

- Date: 2026-09-25
- Kind: `caution`
- Source: issueops-docs io-34938e479083 (#514)
- Summary: 사람이 보거나 record·게이트 원장에 남는 문자열을 바이트 상한으로 자를 때 s[:n]을 직접 쓰면 한글·이모지가 글자 중간에서 잘려 JSON에 U+FFFD가 생긴다. internal/domain/policy의 TruncateBytes·TailBytes를 쓰고, 글자 수 계약이면 TruncateRunes, 바이트 상한 캡처 버퍼의 문자열은 TrimIncompleteRune으로 끝을 정리한다.
- Context: #513 보고에서 preview 출력이 UTF-8 글자 중간에서 잘려 한글이 깨지는 문제가 후속 과제로 남았다(#514). 저장소 전체를 조사하니 진단·미리보기·감사 기록·게이트 원장 EVIDENCE를 바이트로 자르는 지점이 20개(18개 파일) 있었고, stderr 캡처 버퍼 두 곳(providerutil 4096바이트, remoteverify 2048바이트)도 상한에서 글자를 가른 채 진단으로 넘겼다. webfetch.TruncateContent는 스키마가 글자 수(max_chars)라고 하는데 바이트로 잘랐다.
- Resolution: 경계 판정을 internal/domain/policy/text_bound.go 한 곳에 두었다. domain/policy는 foundation owner라 contract 외 internal 패키지를 import할 수 없으므로 새 패키지를 만들지 않고 여기에 둔다(표준 라이브러리 unicode/utf8만 사용). 상한 값·접미사·표지·len(x) > limit 비교·레드액션 순서는 그대로 두고 자르는 식만 바꾼다. 캡처 버퍼는 truncated일 때만 String()이 끊긴 마지막 글자를 버리고 Write와 stdout 바이트는 바꾸지 않는다. 뒤쪽을 자르면서 표지에 뺀 바이트 수를 적는 곳(commandstep.TailWithBudget)은 실제로 남긴 꼬리 길이로 표지를 계산한다. 제외 대상: git 리비전·해시·상태 코드처럼 ASCII만 오는 값, 상한을 넘으면 실패로 처리해 잘린 내용을 쓰지 않는 캡처 버퍼와 LimitReader, 문자열이 아닌 목록 절단.
- Evidence:
  - internal/domain/policy/text_bound.go
  - internal/domain/policy/text_bound_test.go
  - 20개 지점별 회귀 테스트 *_utf8_test.go (이름에 UTF8Safe 포함)
  - go test <17개 패키지> -run UTF8Safe -count=1 -v: 수정 전 20개 FAIL, 수정 후 24개 PASS
  - .issueops/issues/514/artifact/plan.md의 '제외한 후보' 표
  - .issueops/issues/514/gates.md G1~G14
- Alternatives / rejected options:
  - 새 텍스트 패키지: foundation owner인 domain/policy가 import할 수 없어 기각(internal/architecture/dependency_test.go:1127-1129)
  - 상한을 글자 수로 바꾸기: 바이트 상한 계약과 출력 크기 방어가 바뀌므로 진단용은 바이트 상한 유지, webfetch만 스키마대로 글자 수
