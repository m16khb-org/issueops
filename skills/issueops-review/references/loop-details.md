# 루프 세부 규칙

`SKILL.md`의 `## 루프 규칙`에서 호출한다.

- 첫 라운드는 대상 전체를 검토한다. 수정 뒤의 라운드는 **delta 리뷰**다. 대상 전체를
  다시 읽히지 않고 직전 지적, 각 CHECK의 실행 결과, 대상의 delta, 영향받은 계약만
  새 컨텍스트 서브에이전트에 넘긴다. 판정과 기록할 finding은 그 서브에이전트가 정하고
  호출자는 받은 verdict를 그대로 기록한다. CHECK가 전부 통과했다는 사실은 delta 리뷰의
  입력이지 호출자가 `pass`를 정할 근거가 아니다.
  호출자는 다음 규칙을 delta 리뷰 프롬프트에 넣는다. delta 리뷰어는 직전 지적마다 해결됨 또는 남음만
  판정하고, 남음이면 남은 위치와 이유를 적는다. 반증 자료로 철회된 지적은 해결됨으로 적고 반증 근거를
  붙인다. 새 필수 결함은 delta와 영향받은 계약에서만 찾는다. 직전 라운드에 나온 지적을 다른 말로 다시
  내지 않는다.
- delta 리뷰를 띄우기 **전에** 호출자가 수정한 대상을 직접 점검한다. 실제 사이클에서 3~5라운드까지
  간 필수 결함의 절반 가까이가 이 점검으로 잡히는 종류였다(수정이 남긴 옛 표현, 게이트 형식, 틀린 처방).
  1. 수정이 바꾼 값(경로, 명령, 이름, 숫자, 상태, 규칙 문장)마다 옛 값을 대상 전체에서 검색해 남은
     곳을 모두 고친다. 바뀐 항목을 참조하는 절(단계 순서, 게이트, 하위 호환, 수용 기준)을 다시 읽어
     서로 맞는지 본다.
  2. 수정이 새로 단언하는 사실(명령·플래그 존재, 설치된 버전의 동작, `파일:줄`)을 명령으로 확인한다.
     리뷰어가 처방한 수정도 확인 전에는 가설이다. 처방이 틀렸으면 근거와 함께 다른 수정을 쓴다.
  3. 게이트를 고쳤으면 `issueops gates lint --plan <대상> --cwd <워크트리> --json`이 problems 없이
     끝나야 한다.

  세 점검의 결과(고친 곳, 확인한 명령과 출력, lint 결과)를 delta 리뷰 입력에 함께 넣는다.
- 구조나 범위가 바뀌면 전체 리뷰를 다시 띄운다. 필수 검증 공백은 그 확인 결과와
  영향받은 계약을 delta 리뷰에 포함한다. CHECK 형식이 없다는 이유만으로 전체를 반복하지 않는다.
- 같은 대상의 수정·재리뷰는 최대 5라운드다. 3라운드부터 5라운드까지는 상향된 리뷰 모델로
  띄운다. model과 effort는 다음 명령의 결과를 그대로 쓴다. `review-escalate` 설정이 있으면 그
  값이, 없으면 원래 리뷰 모델에 effort 한 단계 상향이 나온다. architecture와 plan 대상이면 `--role plan-review`다.

  ```bash
  issueops model resolve --host "$HOST" --role diff-review --tier "$TIER" --round "$ROUND" --json
  # .model, .effort, .argv
  ```

  사용자가 reviewer 모델을 이름으로 지정했으면 그 값을 `--model`로 함께 넘긴다. 실제로 넘긴 두
  값을 `--reviewer-model`·`--reviewer-effort`(diff) 또는 finding 첫 줄(plan)에 적는다. 그 안에
  통과하지 못하면 남은 결함과 시도한 수정을 보고한다.
  - 서브에이전트 도구는 effort를 받지 않는다. 그러므로 `.argv`로 빈 컨텍스트 세션을 띄운다.
    Claude는 `.argv`에 `--allowedTools
    "Bash Read Grep Glob Skill"`을 붙이고 프롬프트를 표준 입력으로 넘긴다. 출력 파일은 ignored 영역
    `.issueops/issues/<n>/review/`나 워크트리 밖에 둔다. 워크트리 안 미추적 파일은
    fingerprint에 들어가 봉인을 깬다.
  - Fable 5는 명시적 수동 지정 전용이다(`.issueops/adr/2026-09-24-claude-role-models-opus-5-plans-and-reviews-sonnet-5-impleme.md`).
