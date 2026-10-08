# 요청자 의도 계약

- lifecycle: io-e66ba158272a
- issue: https://github.com/m16khb-org/issueops/issues/553
- intent_class: standard

## 원문 요청
issueops를 진행할 때 어떤 목적의 서브에이전트를 사용할 때는 어떤 모델을 사용하는게 좋겠다라는 기능이 있으면 좋겠어 claude code, codex 별로 설정할 수 있으면 좋겠고 io ... 같은 명령어 혹은 /io ... 같은 스킬같은것으로 설정할 수 있으면 좋겠어 그렇게 설정할 때 global로 설정하는 기능과 프로젝트 레포에 설정하는 local로 나뉘면 좋겠고, 그럼 이제 issueops 동작이나 에이전트를 사용할때 해당 목적의 작업을 할 때는 해당 모델로 실행할 수 있도록 자료 조사 후 모호한 부분 질문

## 해석
Requirements (confirmed, 2026-10-08 대화):
1) 역할(목적)별 모델·effort 설정을 host(claude, codex)별로 둔다. 역할은 implement, child-implement, plan-review, diff-review, review-escalate, research, reader-check 7개.
2) 설정 범위는 global(~/.config/issueops/agent-models.toml)과 local(메인 워크트리의 .issueops/agent-models.local.toml). local은 git에 커밋하지 않으며 io가 .git/info/exclude에 등록한다. 워크트리에서도 메인 워크트리의 local을 읽고, 없으면 global, 없으면 내장 기본값을 따른다. 명시 플래그가 최우선이며 병합 단위는 필드(model, effort)다.
3) 내장 기본값: claude implement opus-5-5/high, plan-review·diff-review opus-5-5/high(docs-only면 medium), research sonnet-5-5/medium, reader-check haiku-5-5/medium. codex implement gpt-6.1-sol/high, plan-review·diff-review gpt-6-astra/high(docs-only면 medium), research gpt-6-luna/medium, reader-check gpt-6-luna/low. child-implement는 implement를, review-escalate는 원래 리뷰 모델에 effort 한 단계 상향을 따른다. codex는 astra·sol·luna만 쓴다. omo는 범위 밖이며 기존 기본값을 유지한다.
4) 표면: io model show/set/unset/resolve CLI와 이를 감싸는 /io-model 스킬. resolve는 model, effort, 출처, 빈 컨텍스트 실행 argv, 실행 시점 주입용 에이전트 정의를 돌려준다. 카탈로그에 없는 모델은 경고만 하고 저장한다(미리 지정 허용). 잘못된 host·역할·effort는 에러다.
5) 적용: 하드코딩 상수(agentmodel) 대신 해석기를 execution prepare, child start, io next의 .review, owner 프롬프트 자리값에 연결한다. issueops가 띄우는 세션에는 역할 에이전트를 실행 시점에 주입한다(Claude --agents JSON, Codex -c agents.issueops-<role>.config_file=state dir 생성 파일). 사용자가 직접 띄운 세션의 스킬은 io model resolve의 argv를 쓴다. 이미 봉인된 owner binding은 설정 변경으로 바꾸지 않는다.

## 성공 기준
- io model set/unset/show가 global·local 파일을 쓰고 읽으며, 워크트리 안에서 실행해도 메인 워크트리의 local을 따른다는 것을 단위·통합 테스트로 확인한다.
- io model resolve가 flag > local > global > 내장 기본값 순서와 필드 단위 병합, docs-only 하향, round 3 이상 상향을 지키고 출처를 보고한다.
- execution prepare(owner 미지정), io next --json의 .review, owner 프롬프트가 해석기 값을 쓴다는 것을 테스트로 확인한다.
- Orca·direct owner 실행 명령에 Claude --agents와 Codex -c agents.issueops-<role>.config_file 주입이 들어가고, 실제 host에서 역할 모델로 서브에이전트가 실행되는 것을 claude(전체 model ID)와 codex(gpt-6-luna)로 1회씩 실측한다.
- local 파일이 .git/info/exclude에 등록되어 git status에 나타나지 않는다.
- /io-model 스킬과 issueops-review·issueops-remote-write 등 관련 스킬이 모델 이름을 복사하지 않고 io model resolve를 참조하며, validate-skill 검사를 통과한다.

## 비목표
- (없음)

## 제약
- (없음)

## 모호함
- (없음)

## 읽는 규칙
이 문서는 요청자 의도 계약이다. 원격 이슈 본문은 구현 계약이다. 두 문서가 충돌하면 구현을 시작하지 말고 충돌한 줄을 인용해 blocker로 보고한다.
