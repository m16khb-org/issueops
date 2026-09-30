---
name: 2026-09-30-reader-review-remains-recommended-after-ten-public-bodies
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Reader review remains recommended after ten public bodies

- Date: 2026-09-30
- Kind: `adr`
- Source: IssueOps #515
- Summary: 공개 본문 10건의 지표와 독립 reader 결과에 따라 독자 검토를 권장 절차로 유지한다.
- Context: #516 머지 뒤 첫 H2가 요약인 공개 본문 #518–#527을 측정 전에 고정했다. H2 중앙값 4.5개, 본문 길이 중앙값 1,525.5자, 코드 밖 64hex 0개다. 원래 산식과 PR/MR 8건의 공개 출처를 찾지 못했으므로 재구성 측정이며 정확한 전후 증감률은 N/A다.
- Decision: 독자 검토를 권장 절차로 유지한다. 2026-09-24의 사람용 본문 결정에서 권장한 독자 검토를 재확인하며 게시 필수 게이트를 추가하지 않는다. 의도가 보존된 7/10 본문의 Q1–Q3 21개는 일치했고 3/10은 원래 의도 unknown이다. 10/10 reader가 용어·재독을 지적했으며 #520 Q2는 정보 부재로 답하지 못했다. 단일 모델의 용어 지적만으로 게시를 차단할 근거는 부족하다.
- Consequences: production 코드·공용 스킬·표본 원문은 유지한다. Q4와 완료 상태 설명을 권장 검토에서 확인한다. 단일 모델, 반복된 issue/PR 표본, 대조군 부재, 원래 산식 부재, #527 원격 drift를 한계로 둔다. 인간 독자와 실제 수정 전후, 오탐·소요 시간·비용을 갖춘 별도 후속 연구가 확보되면 필수화를 재검토한다.
- Evidence:
  - [연구 보고서와 지표](../../docs/research/2026-09-30-readability/README.md)
  - [고정 원문](../../docs/research/2026-09-30-readability/frozen-sample.json)
  - [사전 의도 기준](../../docs/research/2026-09-30-readability/pre-reader-rubric.json)
  - [질문별 독자 비교](../../docs/research/2026-09-30-readability/reader-comparison.json)
  - [독립 독자 10건](../../docs/research/2026-09-30-readability/readers/)
  - [공개 기준선 출처와 N/A](../../docs/research/2026-09-30-readability/baseline-provenance.json)
  - [원격 drift](../../docs/research/2026-09-30-readability/remote-comparison.json)
  - [기존 사람용 본문 결정](2026-09-24-issue-and-pr-bodies-are-human-documents.md)
- Alternatives / rejected options:
  - 게시 조건으로 의무화: 원래 의도 unknown 3건, 상관된 10개 본문과 인간 독자·효과·비용 자료 부재로 기각한다.
  - 독자 검토 폐지: 10/10의 용어·재독 지적과 #520의 필요성 정보 부재를 발견하는 유용성이 있어 기각한다.
