---
name: 2026-11-04-derive-ddd-source-ownership-instead-of-recording-it
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Derive DDD source ownership instead of recording it

- Date: 2026-11-04
- Kind: `adr`
- Source: omp follow-up
- Summary: 책임 원장 파일(ddd_responsibility_inventory.json)에서 Go 소스의 path·owner·task·symbols 기록을 빼고, 테스트가 실행할 때마다 소스에서 계산한다. 원장에는 사람이 검토하는 policies와 non-Go artifacts만 남긴다.
- Context: 원장 파일은 662KB였고 그중 sources 1470행과 심볼 7918개가 471KB를 차지했다. owner와 task는 dddOwner가 경로로 정하고 symbols는 AST 선언 목록이라, 이 부분은 전부 소스에서 계산되는 값이다. 그런데도 production 함수를 추가하거나 이름을 바꿀 때마다 -update-ddd-inventory로 재생성해야 했고, 2026-10 이후 이 파일을 바꾼 커밋이 237개다. 재생성 diff는 기계가 만든 값이라 리뷰어가 확인할 판단이 없었다. 또 dddOwner는 맞는 경로 규칙이 없으면 inbound로 분류하므로, 새 internal 하위 폴더가 생겨도 재생성만 하면 원장 검사를 통과했다.
- Decision: 1) 원장 파일은 artifacts와 policies만 저장하고 sources는 저장하지 않는다. sources가 남아 있는 원장은 테스트가 거부한다. 2) policy의 source_path·source_symbol·target_symbol 검사는 실행 중에 계산한 소스 목록을 기준으로 한다. 3) cmd/와 internal/testsupport/ 밖의 Go 소스가 layer 규칙(contract·domain·application·port·adapter·architecture·composition)에 맞지 않으면 테스트가 실패한다. 새 폴더는 dddOwner에 경로 규칙을 추가해야 한다. 4) non-Go artifacts는 개수가 적고 owner를 사람이 정하므로 원장 비교를 유지하며, -update-ddd-inventory는 artifacts만 다시 쓴다.
- Consequences: 원장 파일이 662KB에서 155KB로 줄었고, Go 함수 추가·이름 변경·분해에는 원장 재생성이 필요 없다. 2026-09-30 ADR의 '새 production 파일·심볼은 책임 원장에 배정한다'는 규칙은 경로 기반 dddOwner와 3)의 실패 조건으로 지킨다. 심볼 단위 diff를 리뷰에서 보는 기능은 사라지며, policy가 가리키는 심볼의 존재 여부는 계속 검사한다. 기각한 안: 심볼만 빼고 path·owner·task를 남기는 안은 경로 규칙의 결과를 다시 복사하는 것이어서 택하지 않았다. 원장을 gzip으로 압축하는 안은 diff를 읽을 수 없게 만들어 택하지 않았다.
