---
name: meeting-notes
description: 'Use to turn meeting transcripts, Clova Note exports, Slack huddle notes, or AI summaries into Korean minutes, Slack Canvas documents, action tables, Slack List entries, or reusable minutes prompts.'
---

# Meeting Notes

## Overview

Turn meeting transcripts, Clova Note exports, Slack huddle notes, and AI summaries into action-oriented Korean meeting minutes. Treat the input as raw evidence, then normalize names, technical terms, decisions, action items, unresolved questions, and audit evidence into Slack Canvas-ready Markdown.

## Core Rule

Do not produce a chronological transcript summary unless the user explicitly asks for one. Meeting minutes should preserve what matters after the meeting: decisions, owners, deadlines, risks, follow-up locations, and the evidence needed to audit corrections later.

Decision statements must be certain and attributable. 결정사항에는 불확실한 내용을 넣지 않는다. Put uncertain transcript-backed claims in `리스크/열린 질문` and in the correction appendix instead.

## Required Meeting Inputs

Before producing or creating meeting minutes, require both:

- A participant list from the user or source metadata. This participant list remains required metadata for the final Canvas and for speaker/owner correction; it is not the default Canvas access target.
- The meeting transcript text that will be preserved under `원문 전사본 전문`.

Input collection order is sequential. If either the participant list or transcript is missing, stop and ask for the missing input before drafting, creating, or indexing the meeting artifact, but ask in this order:

1. When no participant list is present, ask only for the participant list. Do not ask for the transcript in the same response.
2. After the participant list is provided, confirm the received participants, then ask for the meeting transcript text.
3. When both inputs are present, continue with the normal Meeting Notes workflow.

Do not infer the required participant metadata solely from generic speaker labels such as `참석자 1`; speaker labels can supplement the correction appendix, but they do not satisfy the required participant list. A final meeting Canvas must not be created from fallback placeholder content.

## Canvas UI Pattern Examples

Detailed snippets live in `references/canvas-ui-examples.md`; load that file only when drafting or debugging a Canvas body. Do not overfit the UI proof Canvas; it was used only to verify which Slack blocks survive read-back. Apply the patterns to the meeting's content density and user goal.

- Status block: use `::: {.callout}` only for MCP Canvas creation after read-back confirms it renders; use `> 회의일 ...` for raw Slack Web API writes.
- Metadata table: use `|Field|Value|` as the two-column vertical metadata table for the team's current Canvas convention. Keep each value short enough to fit narrow Canvas widths; long participant lists, source details, access notes, and tracker links belong in `### 메타데이터 메모`.
- TL;DR bullets: use 2-4 bullets for multi-topic meetings.
- Action checklist: use checklist bullets under `## 액션 보드`.
- Short evidence quote: use only a short quote that clarifies a decision.
- Audit divider: use one `---` before the correction appendix.
- Transcript block: preserve the transcript in a top-level `text` code fence marker such as ```` ```text ````; do not put transcript/code blocks inside layout columns.
- Layout columns: only for short dashboard-style summaries. Do not use them in the default meeting minutes body because tables, callouts, and transcript/code blocks are not supported inside layouts.

## Default Slack Target

- 기본 대상 채널: `#sample-platform-team`.
- Default artifact: create the individual meeting Canvas for `#sample-platform-team`, grant workspace-wide view access, and register the meeting in the existing Slack List using that List's current convention.
- Slack List registration is automatic for published meeting Canvases. Inspect the existing List convention first, then create/update exactly one row through the verified Web API path. Use Slack's built-in `이름` field for the meeting title and do not create a duplicate index List. Manual handoff is a fallback only when API/tool permission is actually unavailable after verification.
- 다른 채널 can be used when the user names one. Treat the named channel as a channel override for the meeting Canvas.
- When drafting only, write the target as metadata and do not claim a Canvas was created.
- After a Canvas is created and read back, register or update the existing Slack List row, then report the List row/item ID separately. If List registration is blocked by verified API/permission limits, provide a manual index-binding handoff instead. This handoff is not part of the Canvas body unless the user explicitly asks for it.

Do not claim to have created, shared, pinned, attached, or updated a Slack Canvas unless a Slack tool call or user confirmation proves it. If only drafting text, call it Canvas-ready content.

## Local Background

Before resolving terms, services, participants, or speaker labels, check for `skills/meeting-notes/background.local.md`. That file is gitignored on purpose; use `skills/meeting-notes/background.local.example.md` as the tracked template and keep team-specific names, service names, and private operating context out of `SKILL.md`.

Use local background as high-confidence correction candidates for product names, team rosters, explicitly listed aliases, confirmed roles, and domain terms. Product and service names from local background are preferred over phonetically similar unknown terms when the meeting context matches the product/service domain; for example, if local background lists `{product}`, then a matching transcript variant should resolve to `{product} staging`, not to an invented project name. If the local background explicitly gives an alias such as `{alias}: {name}`, apply that alias consistently in decisions, actions, topic summaries, and correction maps. Standalone Korean honorific-like words may be name misrecognitions when they phonetically match exactly one roster member; for example, `프로님` can resolve to `{sample_name} 님` when `{sample_name}` is in the local roster or alias list. But `{name} 프로님` should be treated as a title/honorific, not as the alias target, unless the local background explicitly says otherwise. If the local background confirms a role such as `{name}: 팀리더`, do not write `{name} 팀리더 추정`; write the confirmed role directly. Do not silently convert a generic speaker label such as `참석자 N` to a real name from the roster alone. A real-name speaker mapping still needs transcript evidence, user-provided context, an explicit alias entry, a unique phonetic roster match for a standalone name-like expression, or a clear meeting role; otherwise write it as `{이름} 추정` or keep `참석자 N` and record the uncertainty in `참석자/화자 보정`.

## Workflow

1. Identify the requested output: full minutes, Slack Canvas meeting doc with existing Slack List registration, Slack recap, action table, or prompt.
2. Confirm required inputs before drafting or writing:
   - If no participant list is present, ask only for the participant list and stop.
   - If a participant list is present but the transcript is missing, confirm the received participants, ask for the meeting transcript text, and stop.
   - If both participant list and transcript are present, continue with the normal Meeting Notes workflow.
3. Resolve Canvas target:
   - Use `#sample-platform-team` by default.
   - Apply a user-provided 채널 override when present.
   - Record target channel, access assumption, source, and last-updated metadata in the output.
   - Use the required participant list as Canvas metadata and correction evidence; use the public-channel target for the default workspace access grant.
4. Read optional local background, then read the transcript and build correction maps:
   - technical terms
   - uncertain words or sentences
   - participants and speaker labels
5. Extract outcomes before narrative:
   - TL;DR
   - decisions
   - action items
   - risks/open questions
   - follow-up checks
6. Render the meeting in reader order: executive summary, decisions, actions, topic discussion, follow-up checks, then risks/open questions immediately before the correction appendix.
7. Group discussion by topic, not by timestamp, unless exact timeline matters.
8. Produce the requested Korean artifact. When creating an individual meeting Canvas, return the verified List binding values and row/item ID separately after read-back verification.
9. Grant Canvas access through the workspace public-channel visibility path by default (see `Participant Access Grant`). Report unresolved or ambiguous participant/speaker mappings instead of guessing.
10. Preserve the full transcript verbatim in the audit appendix unless the user explicitly excludes it. Only redact security-sensitive strings.

## 세부 문서 (해당 작업 때만 읽는다)

- Canvas 본문을 설계하거나 UI 블록을 고를 때, 또는 산출물을 Anti-Patterns로 점검할 때: [Canvas UI 설계 세부 규칙](references/canvas-ui-design.md)
- 회의 Canvas 본문을 쓸 때(섹션 구조와 필드 순서는 이 템플릿을 그대로 따른다): [Meeting Canvas 템플릿](references/canvas-template.md)
- Canvas 제목 지정, 접근권한 부여, Slack List 등록, Slack 쓰기 경로(create_canvas 실패 대응 포함)를 다룰 때: [Slack 게시 절차](references/slack-publishing.md)
- 산출물을 완료로 보고하기 전 품질 규칙과 92/100 루브릭으로 점검할 때: [품질 규칙과 평가 기준](references/quality-rubric.md)

Canvas를 게시하기 전에 Slack 게시 절차와 품질 규칙을 반드시 읽고 따른다. 핵심 금지: Slack 도구 호출이나 사용자 확인 없이 Canvas를 만들었거나 공유했다고 주장하지 않는다. 전사본 전문은 원문 그대로 한 번만 넣는다. 결정사항에는 불확실한 내용을 넣지 않는다.

## Slack Recap

Use this when the user asks for a channel message to accompany the Canvas:

```markdown
회의록 Canvas입니다: {Canvas 링크}

- 대상 채널: #sample-platform-team or {channel override}
- 핵심 결론: {1문장}
- 결정사항:
  - {결정}
- 액션 아이템:
  - {담당}: {할 일} ({기한})
- 확인 필요:
  - {열린 질문}
```
