# 인계 관찰과 writer 종료

외부 launcher로 인계할 때는 다음을 본문 계약으로 적용한다.

- build, golden, generator, formatter, fixture와 분류 불명 작업은 명령·cwd·출력 위치로
  공유 상태를 바꾸지 않는다는 점을 입증하기 전까지 writer다. 공유 상태 밖의 immutable
  입력만 읽는 독립 reader만 계속 실행할 수 있다. 소유자·핸들·입력 리비전·쓰기 범위·결과
  위치를 열거하고, writer와 자손의 실제 종료 및 대기 writer 0을 관찰한 뒤에만 release한다.
- 제출 전에 event/state subscription을 등록하고 staging observation을 성공적으로 기록한다.
  staging 기록이 실패하면 외부 launch를 실행하지 않는다. idle prompt나 launcher의 `done`은
  수신 증거가 아니다. 모호한 결과는 재제출하지 말고 기존 attempt를 reconcile한다.
- `issueops trace handoff-delivery --input "$OBSERVATION_JSON" --json`의 observation에는
  `schema_version: 1`, `attempt_id`, `lineage_id`, `lifecycle_id`, `prompt_sha256`,
  `material_sha256`, `request.durable_id`, `expected_owner_host`, `source_generation`,
  `created_at`, `updated_at`, `receipt`를 포함한다. `launcher`에는 name/version/path/runtime_id/
  machine_id/server_id, `target`에는 terminal_id/pane_id를 실제 관찰값으로 기록한다.
- 최초 `created_at`, `updated_at`, `call_staged.observed_at`은 같은 RFC3339Nano 시각이다.
  `call_staged`는 observed와 evidence `external_call_staged`; input_accepted,
  native_turn_observed, owner_claimed, ambiguous는 not_observed에서 시작한다.
  최초 durable_id는 비우고 native launcher가 반환한 실제 ID만 채운다.
- 이후에도 같은 attempt/lineage/created_at을 유지하고 native receipt 및 processIncarnation을
  보존한다. receiver의 PID·시작 시각·executable을 실제 process 관찰과 대조한다.
  owner-claim 증거를 수동으로 만들지 않는다. 늦은 결과는 격리하고 release 뒤의 과거
  callback을 source 수정이나 pass 기록에 사용하지 않는다.
- receipt 뒤에는 같은 producer를 다시 호출해 두 번째 observation을 기록하고,
  `observation.receipt.location`과 digest를 읽어 실제 audit frame을 확인한다.
  `target.process_incarnation` 및 `target.process`의 pid/started_at/executable을 실제
  receiver 조회값으로 채운다. PID·시작 시각·executable이 claim holder의 `session_process`와
  모두 맞아야 owner claim과 연결한다. staged 시점에 아직 없는 process를 추측하지 않으며,
  PID reuse-safe 상관관계를 확인할 수 없으면 입력 수락 증거만 남긴다.
- 첫 호출에는 retry ID를 넣지 않는다. 입력 수락의 evidence는 `launcher_receipt`, 실제
  native host의 새 turn만 `native_receipt`다. timeout/응답 유실은 ambiguous의 timeout/
  accepted_response_lost로 기록하고 자동 재전송하지 않는다. Herdr의 pane run/agent prompt
  성공은 `launcher_accepted` 입력 증거일 뿐이며 agent_prompt_stalled/herdr_wait_state는
  ambiguous이지 native turn이 아니다. owner_claimed는 current generation·expected host·
  단일 manual lineage·입력 수락·session_process 일치를 확인한 claim handler만 기록한다.
