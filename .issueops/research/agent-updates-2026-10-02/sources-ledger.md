# 근거 색인과 읽는 순서

조회 기준일: 2026-10-02.
기본 60개 조사와 추가 4개 조사, 영역별 4개 검증 집계를 수행했다.
초기 수집의 DNS 실패 2개는 같은 DAG에서 재시도해 완료했고,
지적된 릴리스 문장 하나는 집계 노드만 amend해 바로잡았다.

개인 경로는 익명화했다. `$REPO_ROOT`는 해당 checkout의 root, `$HOME`는
사용자 home을 뜻한다. 오류 출력도 개인 경로 부분만 이 표기로 바꿨으며
명령·버전·오류 종류·코드 위치의 의미는 유지했다.

## 근거 우선순위

1. 버전이 고정된 원문과 실제 코드·실행 증거.
2. 메인의 교정·재현과 검증 집계.
3. 원래 영역별 조사 노트.

수집 노트에 남은 초기 추정·미해결 항목을 최종 권고로 해석하지 않는다.
외부 URL을 언급했다고 모두 접근·검증에 성공한 것은 아니다.
각 노트는 실패한 URL·명령·단일 공급자 출처를 구분한다.
특히 Claude v2.1.286의 per-turn/idle CPU 문장은 공식 API에서 확인되지 않아
최종 결론에서 제외했다.

## 검증 집계

- [Claude·Codex](digest-1.md)
- [Omo·다른 에이전트](digest-2.md)
- [공통 프로토콜·issueops](digest-3.md)
- [Claude.dev·Streamable HTTP](digest-additions.md)

## 메인의 직접 관측과 교정

- [실제 MCP 51개 도구·30,888 bytes](mcp-baseline.md)
- [trace JSONL 누락 대조 재현](trace-jsonl-reproduction.md)
- [현재 SDK의 구조화 출력 지원](mcp-output-verification.md)
- [Codex 정식 릴리스](codex-release-verification.md)
- [Codex MCP 연결 지원](codex-mcp-verification.md)
- [MCP 2025/2026 개정판 경계](mcp-revision-correction.md)
- [Gemini 정식 버전 교정](release-corrections.md)
- [Claude 사용량 문서 전체 확인](telemetry-verification.md)
- [Claude.dev mods·effort 후속 조사](claudedev-followup.md)
- [실제 self-verify 실패](self-verify-result.md)
- [주장·반증 목록](claim-graph.md)
- [모델 영수증·검증 원장](verification-ledger.md)
- [후속 질문](expansion-log.md)
- [Fable 5.1 검토 계약](final-review-contract.md)
- [Fable 5.1 실제 검토와 반영표](fable-5.1-review.md)
- [검토 전 문서·범위 검사](pre-review-checks.md)

## 60개 기본 조사

| 범위 | 조사 노트 |
|---|---|
| Claude 릴리스·메모리·MCP | [01](claude-01.md), [02](claude-02.md), [03](claude-03.md) |
| Claude SDK·subagent·hook | [04](claude-04.md), [05](claude-05.md), [06](claude-06.md) |
| Claude telemetry·격리·CLI·plugin | [07](claude-07.md), [08](claude-08.md), [09](claude-09.md), [10](claude-10.md) |
| Codex 릴리스·app-server·SDK | [01](codex-01.md), [02](codex-02.md), [03](codex-03.md) |
| Codex context·MCP·multi-agent | [04](codex-04.md), [05](codex-05.md), [06](codex-06.md) |
| Codex 권한·스킬·관측·복구 | [07](codex-07.md), [08](codex-08.md), [09](codex-09.md), [10](codex-10.md) |
| Omo 버전·DAG·모델 | [01](omo-01.md), [02](omo-02.md), [03](omo-03.md) |
| Omo 동시성·monitor·MCP cache | [04](omo-04.md), [05](omo-05.md), [06](omo-06.md) |
| Omo context·로그·eval·취소 | [07](omo-07.md), [08](omo-08.md), [09](omo-09.md), [10](omo-10.md) |
| Gemini·Copilot·Cursor | [01](other-01.md), [02](other-02.md), [03](other-03.md) |
| OpenCode·Aider·Cline | [04](other-04.md), [05](other-05.md), [06](other-06.md) |
| Continue·OpenHands·Goose·ACP | [07](other-07.md), [08](other-08.md), [09](other-09.md), [10](other-10.md) |
| MCP Tasks·progress·출력 | [01](shared-01.md), [02](shared-02.md), [03](shared-03.md) |
| OTel·Anthropic cache·OpenAI cache | [04](shared-04.md), [05](shared-05.md), [06](shared-06.md) |
| Context·Skills·평가·durability | [07](shared-07.md), [08](shared-08.md), [09](shared-09.md), [10](shared-10.md) |
| issueops MCP·SQLite·설치 | [01](issueops-01.md), [02](issueops-02.md), [03](issueops-03.md) |
| issueops hook·관측·process | [04](issueops-04.md), [05](issueops-05.md), [06](issueops-06.md) |
| issueops readiness·lease·검증·skills | [07](issueops-07.md), [08](issueops-08.md), [09](issueops-09.md), [10](issueops-10.md) |

## 사용자 추가 조사

- [Claude.dev 운영 주체·게시일·원출처](claudedev-sources.md)
- [Claude.dev 6개 핵심 글](claudedev-content.md)
- [최신 Streamable HTTP 명세와 client/SDK 지원](streamable-spec.md)
- [Astra의 stdio/HTTP 실행 주체 경계 검토](streamable-boundary.md)

## 실행 기록

- 수집 DAG: `dag_337baed6-3a8d-4b9f-b88b-36efbb29b096`.
- 추가 DAG: `dag_fef1e2f8-dae7-4ba3-9023-94bce6150f86`.
- 수집과 추가 조사 모두 최종 0 failed, 0 skipped로 종료됐다.
- 이 종료 상태는 문서 내용의 정답을 뜻하지 않는다. 메인은 산출물을 읽고
  근거·로컬 링크·변경 범위를 확인했으며 실제 오류도 교정했다.
