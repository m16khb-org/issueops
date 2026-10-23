# GREEN 실패 처리

같은 focused test가 **두 번** GREEN에 실패하면 세 번째를 추측으로 시도하지 않는다.
[`issueops-debugging`](../../issueops-debugging/SKILL.md)으로 실패 명령을 그대로 재현하고
원인을 격리한 뒤 최소 수정을 넣는다. 이 카운터는 [`verified-execution`](../../verified-execution/SKILL.md)의
"같은 기준 3회 실패 → 목표 종료"와 **같은 카운터**이며, 세 번째 실패가 그 종료다. 그때는
4단계를 멈추고 진단과 시도한 수정을 보고한다. 진단 결과는 verified-execution report의
실패 항목에 적는다.
