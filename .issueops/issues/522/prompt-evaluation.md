# Owner prompt 의미 평가

production buildExecutionOwnerArtifacts를 원본 코드와 candidate에 각각 실행하여 Codex/Claude의 실제 렌더링을 생성했다. 빈 컨텍스트 gpt-6-astra/xhigh 평가자에게 각 버전과 대응하는 review skill, supplied next observations를 제공했다. 네트워크·파일 mutation 없이 선택과 명령 완성을 평가했다. 실제 Claude 리뷰 5회를 실행한 결과가 아니라 owner 지침을 읽은 독립 평가자의 bounded 출력이다.

Input/output contract: exact ID io-69의 next.review, round, owner prompt와 review skill → run/stop, model/effort, 설정 출처, 라운드 이유, 최종 implementation-review 명령.

1. Codex docs-only: 원본은 runtime medium과 고정 xhigh가 충돌하여 stop(실패), candidate는 gpt-6-astra/medium을 실행하고 같은 값으로 기록(통과).
2. Claude default: 두 버전 모두 claude-opus-5-5/high, default tier와 네 렌즈, 동일 감사값(통과).
3. Codex contract와 schema-auth: 두 subcase 모두 gpt-6-astra/xhigh와 각 tier·네 렌즈를 전달하고 동일 감사값을 기록(두 버전 통과).
4. review.model 누락과 조회 실패: 리뷰와 기록을 중단하고 prepare fallback을 쓰지 않음(두 버전 통과; candidate는 실패 규칙을 명시함).
5. Claude 3라운드: 원본은 skill의 high→xhigh 상승과 고정 high 기록이 충돌하여 stop(실패), candidate는 같은 claude-opus-5-5 모델과 xhigh를 실행·기록(통과). 상승 이유와 실제 모델·effort는 finding 첫 줄에 남김.

원본 3/5, candidate 5/5. candidate의 리뷰 감사 인자는 모두 실제 선택값과 일치했다. query ID는 모두 io-69였으며, 멈춘 경우에는 리뷰 기록 명령이 없었다.

추가 override 2건도 통과했다. valid Claude default runtime에 사용자 reviewer model=named-reviewer/effort=low를 명시하면 실행·감사값 모두 named-reviewer/low다. owner_model=explicit-model/owner_effort=low만 명시하면 reviewer는 runtime claude-opus-5-5/high를 유지한다. 가상 이름은 supplied 평가 입력이며 설치된 모델이라고 주장하지 않는다.

Test suite: 위 5개 입력과 override 2건. 실제 렌더링과 평가 세부 자료는 ignored artifact 및 /tmp/io-814b092d660e에 보존한다. main owner의 실제 구현 리뷰 실행과 durable implementation-review 기록은 별도로 남긴다.
Adversarial cases: 누락/실패 next 응답, prepare/runtime 불일치, owner override 오인, 라운드 상승 후 감사값 불일치.
One-variable iteration: runtime review 소비와 감사값 입력 계약을 함께 변경하고 동일한 다섯 입력을 원본/수정본에 평가함.
Privacy/tool truth: hidden reasoning을 요구하지 않고 supplied observations와 실제 존재하는 next/record command만 사용함.
