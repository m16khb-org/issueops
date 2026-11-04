---
name: 2026-11-04-shared-git-primitives-and-accepted-duplicates
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Shared git primitives and accepted duplicates

- Date: 2026-11-04
- Kind: `adr`
- Source: omp dupl follow-up
- Summary: 남은 production 중복 후보 5쌍 중 3쌍은 정책이 없는 공유 outbound 패키지 gitraw·workspacefingerprint로 합치고, ratchet 예외는 import하는 어댑터를 이름으로 한정한다. 나머지 2쌍(extensionhost/opencode Dependencies, publicationActor 역변환)은 경계를 지키는 코드이므로 합치지 않는다.
- Context: dupl -t 100으로 production 중복 후보 5쌍이 남았다. 모두 dependency ratchet이 금지하는 capability 경계를 넘는다. (a) lease 교체 snapshot(internal/adapter/issueops)과 reseed inventory(outbound/issueopslease)가 작업 트리 지문을 쓰는 코드를 각자 갖고 있었다. 두 지문은 바이트 단위로 같아야 비교되는데, 실제로 한쪽은 열린 파일 identity를 알 수 없을 때 통과시키고 다른 쪽은 실패시키도록 갈라져 있었다. (b) reseedGitRaw와 preflight.GitCmdRaw는 git을 실행하고 종료 코드를 분류하는 같은 코드였다. (c) extensionhost.Dependencies와 opencode.Dependencies는 필드 열 개가 같은 구조체다. (d) publicationActor(inbound)와 publicationNativeActor(application)는 서로 반대 방향의 계약 변환이다.
- Decision: 1) outbound/gitraw를 만든다. Run(dir, args...)이 git을 한 번 실행해 종료 코드와 가공하지 않은 stdout/stderr를 돌려준다. preflight.GitCmdRaw와 reseed inventory가 이를 쓴다. 2) outbound/workspacefingerprint를 만든다. Digest, Absent, ReadIndex, ReadDiffs가 지문 형식의 유일한 writer이고, git 실행기는 호출자가 주입한다. 두 어댑터가 이를 쓰며 지문 바이트는 바뀌지 않는다. 한쪽이 갈라져 있던 열린 파일 identity 검사는 더 엄격한 issueops 쪽 규칙(목록 시점 identity가 있으면 열린 파일 identity도 있어야 하고 같아야 한다)으로 통일했다. 3) 예외 규칙: isGitRawEdge는 outbound/gitraw를 internal/adapter/preflight와 internal/adapter/outbound/issueopslease에만, isWorkspaceFingerprintEdge는 outbound/workspacefingerprint를 internal/adapter/issueops와 internal/adapter/outbound/issueopslease에만 허용한다. 두 패키지는 gitbin과 osfile 밖의 프로젝트 패키지를 import하지 않으며, TestSharedGitPrimitivesHaveNarrowConsumers가 허용 importer, 거부 importer, 서로 바꾼 importer, 패키지 자신의 import를 검사한다. 4) 합치지 않는다: extensionhost/opencode의 Dependencies는 각 어댑터가 주입받을 함수 집합을 직접 선언하는 구조이고 agy·claude·codex도 필드 집합이 서로 다른 자기 Dependencies를 가진다. 열 개 중 일부가 같다는 이유로 하나를 공유하면 호스트 어댑터가 함께 바뀌어야 한다. publicationActor와 publicationNativeActor는 publication 계약과 issueops 계약을 잇는 경계 변환이다. 공유 함수를 두려면 publication 계약이 issueops 계약을 import해야 하는데, 이는 vertical 계약 분리를 깬다. 두 변환은 각자 컴파일러가 필드 변경을 잡아 준다.
- Consequences: dupl -t 100의 production 중복은 3쌍에서 2쌍으로 줄었고 남은 2쌍은 위 4)의 의도된 것이다. 작업 트리 지문 형식이 한 곳에 있어 lease 교체와 reseed가 서로 다른 값을 내는 일이 없다. 새 예외는 ratchet의 금지 edge 0 규칙을 풀지 않고 이름으로 적은 importer에만 열린다. gitraw나 workspacefingerprint를 쓰려는 세 번째 adapter가 생기면 isGitRawEdge, isWorkspaceFingerprintEdge와 테스트의 허용 목록을 같은 review에서 함께 바꾸고 근거를 남겨야 한다. gitbin에 Run을 더하는 안은 모든 adapter에 열려 있는 gitbin의 의미(실행 파일 이름만 정한다)를 넓히므로 택하지 않았다.
