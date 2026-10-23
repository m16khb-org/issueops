# Slack 게시 절차

## Canvas Naming Rules

Always title individual meeting canvases with this exact pattern:

```text
YYYY-MM-DD [Topic] Title
```

Examples:

```text
2026-06-24 [배포] release/stg 배포 회고
2026-06-25 [테스트품질] 2468 parent 정리 회의
```

Use short topic labels such as `배포`, `테스트품질`, `장애`, `온보딩`, `AI`, `인프라`, `정책`, `리뷰`. If unclear, use `[확인필요]`.

Before calling Slack `create_canvas`, sanitize the API title separately from the human-readable title:

- Avoid literal `&` in the `title` parameter. Use `and`, `및`, or remove the ampersand. For example, send `Sample Platform R and R 및 이벤트 파이프라인 온보딩` as the Slack API title instead of `Sample Platform R&R 및 이벤트 파이프라인 온보딩`.
- Preserve the canonical human-readable title in the Canvas body, metadata, and Slack List row when needed.
- If `create_canvas` returns `Invalid text passed`, retry once with a sanitized title before assuming the body Markdown is invalid.

## Participant Access Grant

Meeting canvases are created with default "invited people only" access, so a freshly created Canvas can be creator-only until sharing is applied. The default sharing policy is **워크스페이스 공개 채널 구성원 열람**: give the Canvas public channel based `read` access so anyone in the workspace who can use that public channel visibility path can view it.

- Use `#sample-platform-team` as the default public-channel sharing target unless the user names another channel. The API call needs Slack channel IDs, so set `CANVAS_ACCESS_CHANNEL_IDS` (comma-separated `C...` IDs) before using the Web API helper.
- Granting access needs the Slack Web API `canvases.access.set` with `channel_ids` and `access_level: read`. Access level defaults to `read`; use `write` only when the user asks for collaborative editing.
- Do not pass `channel_ids` and `user_ids` together. Slack rejects mixed target types in one `canvases.access.set` call. Use `channel_ids` for the default workspace visibility path.
- The participant list remains required metadata: include it in the Canvas metadata, use it for owner/speaker correction, and record unresolved or ambiguous participant/speaker mappings in `참석자/화자 보정`. Do not silently convert generic speaker labels to real people.
- Use participant `user_ids` only when the user explicitly asks for restricted participant-only sharing or the public-channel target cannot be provided. In that restricted mode, only auto-resolve users on a single unambiguous match; on no match or multiple matches (동명이인), do not guess.
- The current MCP surface has no `canvases.access.set` tool. Create/read the Canvas via MCP when available, but run the access grant through the Web API path in `scripts/publish_meeting_canvas.py`. Requires a token with `canvases:write` and `lists:write` for the default published artifact (plus `users:read` only for restricted participant lookup); Lists/Canvas are paid-plan features.

## Slack List Registration

The default published artifact is not complete until both surfaces are done:

- Create/read back the individual meeting Canvas.
- Grant workspace-wide public-channel access through the target public channel.
- Inspect the existing Slack List convention.
- Create or update one Slack List row with the verified Canvas URL.
- Read back or otherwise verify the row/item ID.

After a successful Canvas create/read-back and List registration, report this separate binding summary in the final response or Slack recap, not inside the Canvas body:

```markdown
Slack List 등록 값
- 이름: {meeting title}
- Date: YYYY-MM-DD
- Topic: {Topic}
- Status: {Draft/Follow-up 필요/완료/확인 필요}
- Counts: 결정 N / 액션 N / 질문 N
- Meeting Canvas: {Canvas URL}
```

Rules:

- Do not skip Slack List registration for a published meeting Canvas. If List tooling or permission fails, verify the failure and report the blocker; only then provide the manual handoff fallback.
- Read/inspect the existing List fields and row convention first; preserve its field names, date format, status values, and title convention.
- Store the Canvas link in the List as the workspace docs URL: `https://{workspace}.slack.com/docs/{team_id}/{canvas_id}`. Do not hard-code a real workspace or team ID. Do not store `https://slack.com/canvas/{canvas_id}` in the List; that fallback can open the wrong surface or fail to match the team's existing List convention. If needed, call Slack `auth.test` to derive the workspace URL and team ID before creating or updating the row.
- Do not create an index Canvas as a substitute for the List.
- Do not put an index row preview in the meeting Canvas body by default.
- If the source has no explicit meeting date and the user did not provide one, set the List `Date` to the same value as `Last updated`.
- Use Slack's built-in List `이름` field as the meeting title, following the existing convention of `[Topic] 제목` without the `YYYY-MM-DD` prefix because `Date` lives in its own column. Do not create/delete/rename List fields unless the user explicitly asks for field administration.

## Slack Canvas Write Path

When using Slack tools, prefer direct Canvas creation after the Markdown is structurally validated:

1. Sanitize the Slack API title, especially by removing or replacing literal `&`, then create the complete Canvas in one `create_canvas` call when the body follows Slack Canvas formatting rules and contains the final readable structure.
2. Read the created Canvas back to verify rendered section order, the 2-column metadata table, absence of index-row sections, multi-line decision bullets, checklist actions, topic discussion, follow-up checks, risks/open questions, correction bullets, and transcript appendix.
3. If direct creation fails with `Invalid text passed`, retry once with a sanitized title if the first title contained punctuation such as `&`. If the sanitized title still fails or the failure is clearly a body formatting/length error, fall back to a skeleton Canvas. Read the created Canvas to capture section IDs, and then append or replace complete sections.

Do not leave the skeleton as the final artifact. A Canvas with only a 1-2 line summary or 2-3 actions is a failed forward-test for a long planning meeting. In live Slack verification, a full long meeting body with verbatim transcript returned `canvas_creation_failed: Invalid text passed`, while the skeleton-create plus section-replace path accepted complete readable sections. Therefore, direct create is the preferred path, and skeleton+replace is a verified fallback for long transcript bodies.

When replacing a header section ID, remember that Slack preserves the targeted header and replaces only that header's body content. The replacement body must not repeat the same heading. For example, replacing the `### 원문 전사본 전문` section should send only the transcript code block, not another `### 원문 전사본 전문` line. After each targeted replace, read the Canvas back and reject adjacent duplicate headings such as `### 원문 전사본 전문` repeated twice.

Do not repair a Slack Canvas metadata table through raw Web API cell-by-cell edits. Slack `canvases.sections.lookup` only identifies sections by headers or non-empty text, and raw table edits can leave unaddressable empty rows such as `|||` below metadata. If a metadata table is wrong or too wide after publish, either use an exact full-canvas `replace` only when the complete source Markdown/transcript is available, or create a clean replacement Canvas and update the existing Slack List row to the new Canvas URL. Never declare a Canvas ready while read-back still contains literal `::: {.callout}`, duplicate metadata tables, wide metadata cells, `|||` blank table rows, or placeholder Canvas/List fields.

For the meeting index, use tools/API by default after the Canvas is verified. First inspect the existing List convention, then create/update exactly one row and verify the resulting row/item ID. Return the List binding values with `이름`, `Date`, `Topic`, `Status`, `Counts`, and `Meeting Canvas`. Avoid destructive full-canvas replace unless the user explicitly asks for a full rewrite, the existing Canvas is already corrupted and must be repaired, or the just-created test Canvas is visibly wrong and must be corrected.

For List row repair, Slack `slackLists.items.update` uses `cells`, and each cell must include its own `row_id`; do not pass `row_id` as a top-level field. After updating the `meeting_canvas` cell, read the List row back and reject it unless `originalUrl` is the workspace docs URL pattern above.
