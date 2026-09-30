# #515 새 본문 10건 측정과 독자 검토 정책 결정

## 목적과 승인 범위

- 기존 사이클 `io-fdb9f8c080b3`, 이슈 https://github.com/m16khb-org/issueops/issues/515 를 이어간다.
- 사용자가 후속 작업 진행을 승인했다. 공개 본문 10건의 지표와 독립 독자 답변을 분석하고 ADR로 결론을 남긴다.
- 종료점은 검증된 Draft PR 게시, 최신 HEAD의 push·PR CI 성공, 원격 아티팩트 확인, execution complete와 권한 해제다. 머지·cleanup·설치 갱신은 하지 않는다.
- 브랜치 `515-readability-followup`, base `main`, base SHA `48a3c42606c49248cca5feb31edb5141ac3f075c`다. `$SOURCE_ROOT`는 원본 checkout, `$WORKTREE`는 execution prepare가 만든 canonical worktree다. source checkout은 수정하지 않는다.
- 독자 검토를 게시 조건으로 구현하거나 기존 표본 본문을 고치지 않는다. 비공개 저장소 자료를 사용하지 않는다.

## 적용되는 결정과 주의사항

- `.issueops/CONSTITUTION.md` 제2장 안전·정확성: 원격 공개 본문과 보존된 의도 자료를 확인하고 추정과 사실을 구분한다. 없는 기준선·의도를 만들어내지 않는다.
- `.issueops/ARCHITECTURE.md` 의존 방향과 ownership: 문서 연구이므로 production 계층과 실행 정책을 변경하지 않는다. canonical worktree의 단일 담당 세션만 쓴다.
- `.issueops/CONVENTIONS.md` canonical single-owner pointers: 새 결정은 ADR 한 곳에 쓰고 `.issueops/ADR.md`는 링크만 추가한다. 원격 한국어는 fluent-korean을 적용한다.
- `.issueops/CAUTIONS.md` Universal summary: 공개 자료에 실제 사용자 홈 절대 경로를 넣지 않는다. 게이트 CHECK는 argv 하나이며 복합 검사는 스크립트로 감싼다. 실패한 검증 배터리의 부분 통과를 조합하지 않는다.
- `.issueops/adr/2026-09-24-issue-and-pr-bodies-are-human-documents.md`: 본문은 사람용 문서다. 독자 검토는 지금 권장 절차이며 이번 작업이 런타임 게이트를 추가하지 않는다. 이 결정의 결과만 후속 ADR에 남긴다.
- `.issueops/TESTING.md` 최소 완료 기준과 `testing/self-verification.md`: 문서 변경에도 전체 go test, 빌드, docs, inspect, self-verify가 필요하다. 동일 revision·환경의 최종 배터리로 검증한다.
- `docs/superpowers/specs/2026-09-24-readable-issue-pr-design.md` 조사 범위·전후 측정·독자 검토: 공개 기준선 80건의 기존 집계만 사용한다. 비공개 자료와 존재하지 않는 기준선 중앙값은 사용하지 않는다.
- `.issueops/adr/README.md`의 결정 작성과 `skills/project-docs-update/SKILL.md`의 문서 반영 절차를 따른다.

## 재사용하는 기존 구현

- `internal/domain/artifacttemplate/template.go:522-573`의 bodySections와 SummarySection은 fenced code 밖 H2 절을 파싱한다. H1 문서 제목은 첫 절로 세지 않으므로 #518은 자격이 있다.
- `internal/domain/artifactreadability`의 코드 블록·inline code 제외 규칙을 조사해 코드 밖 64자리 hex 집계에 재사용한다. 새 production 검사·서버·저장 스키마는 만들지 않는다.
- `skills/issueops-remote-write/references/readable-body.md`의 독자 검토 네 질문을 그대로 쓴다.
- `.issueops/issues/<issue>/intent.md`와 기존 spec, 공개 이슈 범위를 의도 비교 근거로 사용한다. PR은 Closes 링크로 실제 이슈와 연결한다. 의도 파일이 없으면 공개 범위와의 비교임을 밝히고 원래 의도 확인 불가를 별도 집계한다.
- 준비 세션이 API로 확보한 공개 snapshot bundle의 `frozen-sample.json`, `candidates-remote.json`, `pr516.json`을 입력으로 쓴다. handoff의 evidence directory에서 복사하고 digest를 확인한다. 임시 디렉터리 실제 경로는 공개 계획·보고서에 넣지 않는다.

## 성능 영향

- 사용자-facing 실행 경로는 바뀌지 않는다. 10개 본문에 대한 일회성 선형 집계이며 전체 문자 수에 비례한다.
- 독자 검토는 정확히 10개 새 컨텍스트를 사용하며 각 호출은 본문 하나만 읽는다. API 재조회는 표본 drift 확인에 한정한다. 반복 호출·대규모 추가 표본 수집은 하지 않는다.
- 비용과 latency는 실제 관측 가능한 호출 수·시각·모델을 기록한다. 토큰 비용을 관측하지 못하면 임의 금액으로 환산하지 않는다.

## 하위 호환성과 side effect

- 변경은 공개 연구 결과·재현 자료, ADR, ADR 색인과 IssueOps 작업 문서로 제한한다. CLI JSON, MCP schema, record, golden, Go core, shared skill 동작은 변경하지 않는다.
- 새 ADR을 governed project-docs 흐름으로 반영한다. 기존 계약 golden이 실제로 실패할 때만 원인을 확인하고 필요한 최소 갱신을 한다. unrelated 재생성은 금지한다.
- public snapshot 원문에 민감한 내용이나 실제 로컬 사용자 경로가 있으면 먼저 검토한다. 원문 SHA와 계산 입력을 훼손하는 임의 정규화는 하지 않는다. 공개 원문이 정책상 그대로 커밋 불가하면 ignored evidence에 원본을 보존하고 공개 보고서에는 링크·해시·재현 방법과 제한을 명시한다.
- 새 표본 body는 수정하지 않는다. #515만 현재 선행 조건과 범위를 반영해 preview/CAS/readback으로 이미 갱신했다.
- 문서 커밋을 되돌리면 변경을 롤백할 수 있다. 공개 원격 댓글 추가나 구현 게이트 변경은 없다.

## 표본과 측정 기준

1. 선행 구현 PR #516의 merge 시각은 `2026-09-24T07:43:42Z`다. 그 뒤 게시된 공개 이슈·PR을 `(created_at, number)` 순으로 정렬하고, 첫 H2가 `## 요약`인 첫 10건을 선정했다.
2. 고정 표본은 #518, #519, #520, #521, #522, #523, #524, #525, #526, #527이다. #516/#517은 merge 전 게시, #528은 첫 10건 이후라 제외했다. 초기에 H1까지 첫 제목으로 보려던 기준은 실제 template.go의 H2 계약을 확인해 측정·독자 호출 전에 정정했다. 원래 후보 목록과 정정 사유를 함께 남긴다.
3. snapshot은 현재 관측 본문이다. 초기 게시 당시 원문으로 단정하지 않는다. 수집 시각, created/updated 시각, title/body, body UTF-8 SHA256을 보존한다. 같은 snapshot으로 10건 모두 지표와 독자 검토를 수행한다.
4. 계산 전 기존 조사 스크립트/기록을 좁게 찾아 실제 산식을 확인한다. 발견되지 않으면 '원래 스크립트 미보존, 공개 기술에 맞춰 재구성'을 명시한다. H2 절 수(코드 fence 제외), Unicode 문자 수(본문 전체), 재구성 산식의 case-sensitive substring 용어 횟수(execution/lease/봉인/generation, 코드 포함), 첫 H2 요약 여부, 코드 밖 경계가 분명한 64자리 hex 개수를 정의하고 재현 가능하게 남긴다. 기준선과 의미가 달라지는 추가 지표는 별도 열로 구분한다. 원래 산식을 뒷받침하는 기록이 없으면 동일 방법 검증은 충족했다고 주장하지 않고 재구성 측정임을 명시한다. 원래 수치와 나란히 제시할 수 있어도 정확한 전후 증감 비교는 N/A로 둔다.
5. 공개 기준선: 네 본문의 완료 구간44–95%, 본문80건 raw 용어합 execution287/lease213/봉인205/generation197. 설계 문서의 “PR/MR 8건”은 공개·비공개 중 어느 표본인지 확인되지 않았다. 원래 공개 표본8건 목록과 계산 근거를 찾은 경우에만 공개 실측 기준선으로 사용하고, 없으면 그 실측값은 N/A로 둔다. 과거 템플릿의13절 요구는 설계상의 계약 사실로만 별도 표시한다. 전후 전체 합만 직접 비교하지 말고 분모를 표시하고 본문당 빈도도 제시하되 산식 동등성이 확인되지 않으면 증감률은 제시하지 않는다. 없는 문자 수·hex·요약 비율 기준선은 N/A로 둔다. 필요하면 완료 구간 비율도 같은 snapshot에서 보조 측정하되 이전 표본의 선정 차이를 밝힌다.
6. 첫 H2 요약 비율은 선택 조건이라 100%가 되며 개선의 독립 증거로 해석하지 않는다. 연결된 이슈·PR은 같은 작업의 반복 관측이고 10개의 독립 실험이 아니다. 인과 효과·유의성·인간 가독성 개선을 단정하지 않는다.

## 작업과 수용 기준

### T1 공개 원문과 의도 근거를 고정한다

담당은 native 구현 세션이다. 입력 snapshot과 SHA를 검증하고 연구 evidence 디렉터리에 보존한다. PR↔issue 의도 매핑을 만들고 독자 결과를 보기 전에 각 의도에서 문제·변경·필요성·완료 판단 기준을 발췌한다. 기존 원문이 없으면 확인 불가로 표시한다. 원래 baseline 산식/스크립트와8건 표본의 공개 출처 검색 결과도 남긴다. 찾지 못하면 해당 empirical baseline은 N/A이며 옛 템플릿13절 계약과 분리한다.

- 검증: 선정 순서와 제외 이유, 10개 유일 번호, body SHA, merge 후 created_at, 실제 first-H2 규칙이 재계산으로 일치한다.
- 실패 시나리오: snapshot body 한 글자를 바꾼 임시 사본은 SHA 검증이 실패해야 한다. 원본은 변경하지 않는다.

### T2 지표를 재계산 가능한 결과로 남긴다

본문10건의 지표표, 분모를 표시한 합계/중앙값, 가능한 baseline 비교를 연구 보고서에 쓴다. 짧은 재현 스크립트 또는 standalone 명령을 같은 연구 폴더에 둔다. production용 추상화와 새 범용 테스트 suite는 만들지 않는다. 원격 본문을 다시 읽어 snapshot의 hash와 비교하고 drift가 있으면 원본 결과와 최신 상태를 구분해 기록한다.

- 검증: 새로운 프로세스에서 고정 snapshot을 재계산한 결과가 저장된 지표표와 정확히 같다.
- 실패 시나리오: 코드 fence 안의 가짜 H2와 64hex, inline code의64hex는 해당 제외 지표에 세지 않고, 코드 밖64hex와 실제H2는 센다. 연구 스크립트의 작은 임시 fixture로 확인한다.

### T3 같은 10건을 독립 독자에게 읽힌다

AGENTS의 독립 검증 패턴과 기존 독자 검토 계약에 따라 각 본문마다 빈 컨텍스트 subagent 한 개를 띄운다. 각 reader에게 제목과 정확한 snapshot body, 아래 네 질문, '본문만 읽고 도구나 외부 자료를 쓰지 말 것'만 준다. 계획, 의도, 정답, 앞선 reader 답변, 정책의 기대 결론은 넘기지 않는다.

1. 무엇이 문제이고 무엇이 바뀌는가?
2. 왜 지금 필요한가?
3. 끝났는지 어떻게 확인하는가?
4. 모르는 용어나 두 번 읽은 문장이 있는가?

독자별 질문·답변 원문·모델/시각·입력해시를 보존한다. 답1–3을 사전 의도 기준과 비교한다. 명시한 사실과 반대, 범위/완료 경계를 바꾼 답은 불일치다. 단순한 문장 차이는 불일치가 아니다. 본문에 정보가 없어 답할 수 없는 경우와 모르는 용어(Q4)는 각각 별도로 세며 억지로 match나 mismatch로 넣지 않는다. 본문당 불일치 유무와 질문별 근거를 모두 남긴다. 의도 부재는 unknown이며 match분모에서 제외하고10건 전체 분모도 표시한다.

- 검증: 서로 다른 새 컨텍스트10개, 각 snapshot1개, 네 답변 각각 존재, 10개 모두 비교 근거와 판정이 있다.
- 실패 시나리오: reader가 도구를 쓰거나 다른 본문/의도 정보를 제공받으면 해당 호출은 무효임을 기록하고 같은 immutable 입력으로 새 reader를 한 번만 재실행한다. 기대 답에 맞추려는 재호출은 금지한다.

### T4 ADR과 검증된 Draft PR로 마무리한다

효과와 한계를 근거로 '게시 조건으로 둔다' 또는 '권장 절차로 유지한다' 하나를 결정한다. 단일 모델의10건·전후 aggregate만으로 인과성이나 필수화 비용 대비 이익을 확정할 수 없으면 권장을 유지한다. 의무화 결론이어도 구현은 별도 후속 이슈이고 이번 코드 범위에 넣지 않는다. ADR은 결정/근거/대안/한계/참고표를 갖추고 governed docs route/read/validate/confirm 절차로 색인에 연결한다.

- 검증: ADR 결정 문구, 색인 링크와 실제 파일,10건 결과·독자 비교에 대한 링크를 확인한다. 독립 diff리뷰가 표본 편향/의도 비교/분모/범위와 근거를 검토한다.
- 실패 시나리오: 잘못된 링크·11건·존재하지 않는 baseline·누락reader가 있으면 완료하지 않는다.
- 커밋: Conventional Commit + Lore. 연구 자료와 결정이 한 흐름이므로 review 가능한 문서 커밋으로 묶는다.

## 게이트와 최종 배터리

작업공간에 단일 gates 원장을 만들고 다음 CHECK를 실제 산출물 경로에 맞춰 argv형태로 등록한다. 연구 검증용 script 이름은 구현 시 결정하고 아래 기능을 한 명령으로 재현한다.

- G1: 고정 표본과 hash 재계산 | CHECK: 연구 verification script의 snapshot 모드 | EXPECT: snapshot verified: 10
- G2: 지표 재계산 | CHECK: 같은 script의 metrics 모드 | EXPECT: metrics reproduced: 10
- G3: reader독립성·4문답·의도 비교 evidence | CHECK: 같은 script의 evidence 모드 | EXPECT: reader evidence verified: 10
- G4: ADR 결론·색인·상대링크 | CHECK: docs검사 및 연구 script decision 모드 | EXPECT: decision linked
- G5: 실제 원격 재조회 비교 | CHECK: 원격snapshot drift 검증 명령 | EXPECT: remote comparison recorded: 10
- G6: 최종 문서 배터리 | CHECK: 한 실행 스크립트로 아래 명령을 순차 실행 | EXPECT: document battery passed

최종 문서 배터리는 `./bin/issueops self-verify --seed=100 --target-score=95 --llm-eval=false --json`의 완전한 step 결과로 전체 go test, build, docs, inspect 요구를 확인한다. `.issueops/testing/self-verification.md:59-65`에 따라 같은 revision·환경의 self-verify가 이미 실행한 test/build/docs/inspect를 별도 최종 책임으로 중복 실행하지 않는다. self-verify 실행용 바이너리가 없으면 먼저 작업공간 안에 빌드한다. local/CI 누락을 피하려고 `python3 -m unittest discover -s scripts -p '*_test.py'`도 같은 배터리에 넣는다. Python은 Homebrew3.14를 PATH앞에 명시하고 Go는 프로세스 범위 `GOFLAGS=-p=2 GOMAXPROCS=4`로 동시성을 제한한다. 검사를 생략하지 않는다. 실패하면 전체를 처음부터 재실행하고 실패 로그를 보존한다.

문서검사와 finalreview 뒤 파일을 고치면 필요한 evidence와 봉인을 갱신한다. latestHEAD의 push 및 PR CI가 모두 통과해야 verify-artifact와 execution complete로 끝낸다. GitHub의 과거 green이나 이전HEAD green으로 대체하지 않는다.

## 의존 관계와 인계

T1 → T2/T3(immutable 입력 공유, reader만 독립) → T4. 다른 후속 작업의 코드와 독립이다. 메인 구현자는 직접 연구·문서·검증을 수행하고 reader/reviewer만 필요한 독립 컨텍스트를 사용한다.

계획 검토 통과 후 execution prepare direct로 worktree를 준비한다. Orca가 ready면 설치된 public CLI로 기존 canonical worktree에서 Codex gpt-6.1-sol/high 새 세션 한 개에 인계한다. writer가0이고 자손 작업이종료됨을 확인해 인계자료를 봉인하고 현재 세션을 release한다. Codex의 지원되는 dangerous bypass flag를 유지한다. recipient는 자기 native actor로 생성된 exact recovery 명령을 따라 claim하고 최신 HEAD와 계획·입력digest를 확인한다. 중복 세션·중복 프롬프트를 만들지 않는다.
