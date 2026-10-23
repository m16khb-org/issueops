# 품질 규칙과 평가 기준

## Quality Rules

- Add the top status block before metadata. It should fit on one line when possible and contain only scope/status facts: date or period, target channel, source/basis, status, and exclusions if any. Use `::: {.callout}` only for Slack MCP Canvas-flavored Markdown; use `>` quote syntax for raw Slack Web API writes because the Web API Markdown support does not include callout blocks.
- Use the default UI recipe unless the user asks for a different Canvas style. A passable meeting Canvas should visibly use at least these UI affordances: top status block, 2-column metadata table, checklist action board, layer-cake headings, and a transcript code block. Long Canvases should also use one top-level divider before the audit appendix.
- Keep the metadata table narrow. The `Field`/`Value` table is allowed only for short values; long cells make Slack Canvas render a wide, hard-to-read table. Use summaries in the table and move detail to `### 메타데이터 메모`. A publish path should reject metadata rows that would exceed the width guard rather than posting a wide table.
- Put `TL;DR`, `결정사항`, and `액션 보드` above detailed discussion.
- Render `TL;DR` as 2-4 bullets when there are multiple independent takeaways. Use short paragraphs only when the meeting has a single clear outcome.
- Render `결정사항` as scannable multi-line bullets with separate `내용`, `근거`, `영향`, `결정자/동의자`, and `상태` fields. A long one-line decision that packs all fields into one sentence is a readability failure on Slack Canvas.
- Render `리스크/열린 질문` after `후속 확인` and immediately before `보정 및 원문 부록`. This keeps executable meeting flow above and uncertainty/audit material together near the appendix.
- Render every risk/open-question item as a titled multi-line bullet with separated `내용`, `확인 담당`, `확인 방법`, and `상태` fields. A one-line risk that packs all fields into one sentence is a Slack Canvas readability failure.
- Optimize for the actual Slack Canvas reading surface, not only Markdown validity. The meeting Canvas body should not include a meeting-index row by default. Provide verified Slack List row values outside the Canvas after the Canvas URL is known. Any table over 5 columns is a quality failure unless the user explicitly asks for export-oriented tabular data.
- A created or read-back meeting Canvas must not retain placeholder cells such as `미정`, `생성 후 인덱스 참조`, or `{Canvas 링크}` for the Canvas URL after the Canvas URL exists.
- Do not use layout columns for the default meeting minutes body. Layouts are useful for short dashboard summaries, but Slack Canvas does not support tables or callouts inside layouts, and long meeting minutes need predictable vertical scanning.
- Keep actions as checklist bullets so they remain readable on narrow Canvas widths.
- If an action is long-term tracking 대상, keep the meeting-time checklist item in Canvas and put the live tracker in `Tracking` as a Slack/GitLab/문서 링크.
- Use one accountable owner when possible. If the transcript implies a team owner but not a person, write `미정` and add an open question.
- If local background and meeting role identify a team lead owner for a follow-up or risk, use the confirmed role in executable fields. Do not leave team-lead-owned follow-ups as `참석자 1` while separately naming a confirmed team lead elsewhere. Speaker mapping can stay uncertain in the appendix, but owner/decision fields should use the best confirmed role.
- Prefer deliverables over vague verbs: `검토한다` is weak; `GitLab 이슈 본문에 검증 체크리스트를 추가한다` is strong.
- Include a due date only when the transcript states or clearly implies one. Otherwise use `미정`.
- Correction maps must contain the fields `원문 표현`, `보정 표현`, `근거`, `신뢰도`, `확인 방법`, but render them as bullets in Slack Canvas unless the list is short enough for a 5-column table to stay readable.
- If a mapping table approaches Slack Canvas's 300 cells limit, split it into sections such as `용어 보정 1`, `용어 보정 2`, `불확실 단어/문장 보정 1`. The 300-cell limit is not enough by itself; rendered readability still fails when the table is too wide.
- Preserve general meeting statements by default. Redact only security-sensitive raw strings such as secret/token/password/API key/private key values with `[민감정보 생략]`.
- Do not paste uncertain transcript-backed claims into `결정사항`; put them in `리스크/열린 질문` and the appendix.
- If Canvas update noise matters, mention that workspace/admin Canvas update-message settings may need review before bulk appendix or transcript updates.

## Forward Quality Rubric

Use a rubric before calling a meeting-minutes output good enough. Treat the first run as the baseline score and keep improving until the current output meets the pass line.

- Baseline: record the first output's baseline score before edits. A weak but structurally valid first pass is expected to be around 25-70/100.
- Pass line: a reusable meeting Canvas must score at least 92/100 before it is treated as ready for `#sample-platform-team`.
- Do not fabricate meeting dates from the transcript. If the source has no explicit meeting date and the user did not provide one, set `Date` to the same value as `Last updated`. Do not add a separate open question just to confirm the meeting date unless the user asks for calendar reconciliation.
- 실명이 전사본만으로 확정되지 않으면 speaker labels remain uncertain. Use `참석자 1(팀 리드 추정)` or similar wording, then put the exact mapping in `참석자/화자 보정`.
- 용어 보정 후보를 먼저 작성, then use only high-confidence corrected terms in decisions/actions. Low-confidence terms stay in `불확실 단어/문장 보정` and open questions.
- Verbatim full transcript handling: if the user provides transcript text, `원문 전사본 전문` must contain that pasted transcript verbatim in one top-level `text` code block, preserving speaker labels, timestamps, wording, omissions, order, and line breaks as much as the Canvas surface allows. Do not summarize, normalize, translate, or substitute representative blocks under `원문 전사본 전문`.
- The pasted transcript must appear exactly once. Do not append the transcript once during skeleton creation and then append or replace it again during section repair. Before declaring a Canvas ready, compare the `원문 전사본 전문` section against the source transcript by marker count or hash: the transcript start marker, transcript end marker, and full transcript body must each occur once.
- If a hard Slack/API limit prevents inserting the complete transcript after trying the skeleton-create plus targeted section update path, do not label the appendix `원문 전사본 전문`. Rename it to `원문 전사본 발췌`, state that the full source is linked in `Source`, and include the exact source location. This is a degraded output and must be called out before treating the Canvas as ready.
- A passable output must create/read back the meeting Canvas, register it in the existing Slack List, and provide separate List binding values that use Slack's built-in `이름` field as the title plus the verified row/item ID. The Canvas body itself should focus on meeting content, not index maintenance.
- A passable created/read-back output must not contain Canvas URL placeholders such as `미정`, `{Canvas 링크}`, or `생성 후 인덱스 참조` after the Canvas URL exists.
- A passable output must not leave known team-lead follow-ups or risks as `참석자 1` when local background and meeting role evidence identify the owner as `{sample_team_lead} 팀리더`.
- A passable output must render risks/open questions as separated fields, not dense one-line records containing `확인 담당`, `확인 방법`, and `상태` in the same bullet.
- A passable output must contain exactly one `### 원문 전사본 전문` heading and one top-level `text` transcript code block under it. Duplicate adjacent transcript headings are a Slack Canvas write-path failure.
- A passable output must include a compact top status block before metadata. For MCP Canvas creation this may be `::: {.callout}` only when read-back confirms it renders as callout UI. For raw Slack Web API creation or repair, it must be a `>` quote line; visible literal `::: {.callout}` is a failed Canvas.
- A passable output must keep the metadata table narrow: no long participant/source/tracking/access cells, no duplicate metadata tables, and no malformed read-back rows such as `|||`, `||Value|`, or `|Date||`. Put long detail in `### 메타데이터 메모`.
- A passable output must follow the Canvas UI recipe: top status block, vertical metadata table, TL;DR bullets for multi-topic meetings, multi-line decision bullets, checklist actions, meaningful `###` topic headings, one divider before the audit appendix, correction bullets, and a transcript code block.
- A passable output must keep section order coherent for readers: `TL;DR` -> `결정사항` -> `액션 보드` -> `주제별 논의` -> `후속 확인` -> `리스크/열린 질문` -> `보정 및 원문 부록`.
- A passable output must show care for readability: decisions use multi-line bullets, not one dense sentence containing decision, evidence, impact, owner, and status.
- A passable output must not contain one 12-column index table, a secondary index detail table, or any table wider than 5 columns for Slack Canvas. Preserve detail fields in the meeting Canvas metadata and Slack List registration values, not in an index table embedded in the Canvas.
- A passable output must have enough substance for the input length. For a 30+ minute planning transcript, `TL;DR` plus sections must cover the major topics; a two-line meeting summary is a failed output.
- Re-run `python3 scripts/meeting_notes_quality_rubric.py` after prompt/skill changes when this repo's fixture is available.
