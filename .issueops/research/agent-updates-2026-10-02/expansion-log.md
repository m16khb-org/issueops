# 후속 조사 목록

각 항목은 첫 수집에서 나온 질문이다. 같은 공급자의 다른 문서를 독립 증거로
추가하는 작업은 별도 조사로 세지 않는다. 실제 효과 측정은 구현 제안의 검증
실험과 현재 조사의 사실 확인을 구별한다.

| ID | 질문 | 출발 근거 | 담당·처리 |
|---|---|---|---|
| E01 | 전체 MCP catalog의 실제 크기는 얼마인가 | claude-03의 하위 목록 일반화 | 완료: 메인의 실제 stdio 호출로 51 tools / 30,888 bytes 확인, mcp-baseline.md |
| E02 | v2.1.286의 per-turn 성능 문장이 어느 릴리스에 속하는가 | claude-01 vs 공식 API | 후속 검증: 현재 주장은 unresolved로 두고 최종 결론에서 제외 |
| E03 | 호스트 추적의 TRACEPARENT를 issueops가 소비하는가 | 공식 monitoring 원문 | issueops-05에 전달, hook 밖의 최소 상관관계 개선 여부 확인 |
| E04 | Codex experimental context manager가 정식 버전에서 실제 사용 가능한가 | codex-04의 공식 설정 문서와 포럼 충돌 | 후속 wave에서 버전 고정 소스·실행 옵션 확인, 포럼만으로 권고하지 않음 |
| E05 | Codex host turn의 cache/usage가 지원되는 외부 표면에 노출되는가 | codex-04 | codex-09·issueops-05의 기존 조사와 합쳐 중복 제거 |
| E06 | Claude --bare·plugin source 변경이 현재 issueops 경로에 영향을 주는가 | claude-01, claude-10 | 로컬 host adapter 조사 결과와 대조 |
| E07 | upstream catalog의 Git skill 수가 운영 문서와 다른가 | claude-10 | 완료: 메인이 configs/upstream.json에서 plugin 4개, Git skill 2개를 확인. operations/hosts.md:89-94는 skill 1개로 서술. 단순 문서 drift이며 설치 결함으로 확대하지 않음 |
| E08 | Streamable HTTP가 native actor 계보를 보존할 수 있는가 | 사용자 추가 요청과 현행 in-process ADR | 추가 DAG의 streamable-boundary, Astra 배정 |
| E09 | Claude.dev 글은 누구의 원문이며 다른 공식 글과 독립적인가 | 사용자 추가 요청 | 추가 DAG의 claudedev-sources/content |
| E10 | cache·compaction 최적화를 실제 host 비용으로 어떻게 측정할 것인가 | claude-02·03, codex-04 | 최종 실험 설계: byte/token 구분, latency·총비용·정확성 동시 비교 |
| E11 | 기존 trace/status가 작은 duration과 pagination을 이미 제공하는가 | codex-01 | 로컬 관측·history 조사 결과로 확인, 없는 기능이라고 미리 가정하지 않음 |

## 실험으로 남길 항목

실제 계정별 Claude `/usage`, 상세 beta trace, sandbox on/off, SDK prewarm 비교는
공식 기능의 존재와 달리 비교 workload·옵션·권한 조건을 정해야 한다.
이번 조사는 사용자 설치·설정을 바꾸지 않는다. 필요한 조건과 성공 기준을
보고서에 제시하되 측정하지 않은 절감률은 쓰지 않는다.

## 복구

`claude-08`, `claude-09`의 DNS 오류는 조사 결론이 아니다.
첫 DAG settle 뒤 실패 노드만 재시도하고 이미 성공한 작업은 재실행하지 않는다.

## 최종 처리

위 표는 수집 중 생성한 질문 목록이다. 조사 완료 시 상태는 다음과 같다.

- E01, E07: 직접 관측·파일 대조로 종료.
- E02: 같은 공식 API body를 다시 받아도 문장이 없었다. 집계 노드를 amend해
  unsupported 문장을 제거했고 최종 결론에서 제외했다.
- E03, E05: 공식 host usage/TRACEPARENT 표면은 확인했으나 현재 core writer
  존재는 입증되지 않았다. 없는 기능을 있다고 쓰지 않고 읽기 전용 adapter
  제안과 검증 조건으로 종료했다.
- E04: live 설정 문서는 experimental mode를 사용 가능하다고 보장하지 않는다.
  보고서에서 이 기능의 도입을 권고하지 않는 것으로 종료했다.
- E06: bare·plugin source 변경을 확인했지만 현재 설치 경로의 live 실패를
  주장하지 않는다. host별 compatibility 검증 후보로 분류했다.
- E08: native mutation stdio 유지, 별도 수요가 있을 때만 선별 read-only HTTP
  검토라는 Astra 분석을 원문·코드와 대조했다.
- E09: Anthropic 운영 약관, author/date, 같은 원문 redirect를 확인했다.
- E10: 성능 효과를 가설로 남기고 제안별 측정·수용 기준을 최종 보고서에 적었다.
- E11: 기존 span/readiness/review metrics를 확인하고 중복 구현 목록에서 제외했다.

추가 feed lead 두 개(mods, effort)는 [후속 조사](claudedev-followup.md)에서
본문을 읽고 종료했다. 실험·구현 제안은 조사 결과이지 미완료된 구현 약속이 아니다.
DNS 실패 2개는 retry로 완료했고 기본 DAG 최종 상태는 63/63이다.
