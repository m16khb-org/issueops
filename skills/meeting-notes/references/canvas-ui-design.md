# Canvas UI 설계 세부 규칙

## Canvas UI/UX Principles

Slack Canvas is a fully formatted surface for information that does not fit in a normal message. Design meeting Canvases for scanning first, then audit depth.

Use these proven patterns:

- **Top status block UI:** after the title, add one short scope/status block. The syntax depends on the write path: Slack MCP Canvas-flavored Markdown may use `::: {.callout}` after read-back confirms it renders as a callout UI, but raw Slack Web API `canvases.create`/`canvases.edit` does not support that callout syntax and renders it literally. When using `.zshrc` token or `scripts/publish_meeting_canvas.py`, use a Web API-safe quote line such as `> 회의일 YYYY-MM-DD · 대상 #sample-platform-team · Source synthetic transcript · Follow-up 필요`.
- **Progressive disclosure:** put the answer first (`TL;DR`, decisions, actions), then topic detail, then follow-up, then uncertainty and appendix. Readers should understand the meeting without reaching the transcript.
- **Layer-cake headings:** headings must be meaningful on their own. A reader scanning only headings should see the meeting flow.
- **Chunk dense facts:** split long decision/action/risk lines into short nested bullets with labels. Do not pack decision, evidence, owner, impact, and status into one sentence.
- **Use tables only where comparison helps:** keep metadata as a compact two-column table with short cell values; avoid wide tables for decisions, risks, corrections, or meeting indexes because Slack Canvas tables expand to the longest cell and compress columns aggressively. If a metadata value is long, put a short summary in the table and move the detail to `### 메타데이터 메모`.
- **Use status blocks sparingly:** one scope/status block near the top, optional warning block only for high-impact caveats. Do not wrap normal body sections in callouts.
- **Prefer numerals and concrete labels:** counts, dates, owners, status, and tracker names should be visible without reading full prose.
- **Mirror source-specific UI when useful:** if a source Canvas or user-provided example has a clearer pattern, extract the pattern and adapt it, but do not copy irrelevant formatting.

## Slack Canvas UI Block Palette

Use Slack Canvas UI blocks intentionally. The default meeting-minutes pattern is not plain Markdown; it is a readable Canvas document composed from a small, verified block palette.

| Block | Use For | Meeting-Minutes Rule |
|---|---|---|
| Callout/status quote | Scope, status, high-impact caveat | Use one top status block before metadata. Use MCP `::: {.callout}` only on the MCP path; use `>` quote syntax on raw Web API paths. |
| 2-column vertical table | Metadata fields | Use `Field` / `Value`; do not make metadata a wide horizontal table. |
| Narrow table | Short comparisons | Use only for metadata-like comparisons that genuinely scan better as rows. Avoid action/risk/correction mega-tables. |
| Checklist bullets | Action items | Use `- [ ]` with owner, deliverable, due date, and tracker in one scannable line. |
| Heading hierarchy | Section scanning | Use only `#`, `##`, `###`. Make headings meaningful without reading body text. |
| Slack emoji codes in headings | Visual anchors | Use sparingly for major sections only, such as `:dart:` or `:white_check_mark:`. Do not decorate every bullet. |
| Layout columns | Short side-by-side summaries | Optional only for dashboard-style summaries. Do not put tables, callouts, or transcript/code blocks inside layouts. |
| Horizontal rule | Separate execution area from audit appendix | Use one top-level `---` before `보정 및 원문 부록` when the Canvas is long. Do not put dividers between every section. |
| Block quote | Short verbatim evidence | Use only for a short source quote that clarifies a decision. Do not use block quotes for normal summary text. |
| Code block | Verbatim transcript | Use a top-level `text` code block only for `원문 전사본 전문`. |
| Links and Slack references | Trackers, source, channel, people | Prefer labeled links. Use Slack Canvas reference syntax only when IDs are known. |

Default meeting Canvas UI recipe:

1. Title.
2. Top status block with date/target/source/status.
3. 2-column metadata table.
4. TL;DR as 2-4 bullets or short sentences.
5. Decisions as bold titled bullets with separated fields.
6. Actions as checklist bullets.
7. Topic discussion with `###` sections.
8. Follow-up and risks near the appendix.
9. One divider, then correction maps and verbatim transcript.

## Canvas Anti-Patterns

Reject these outputs before they reach Slack:

- Do not use one dense paragraph for TL;DR in multi-topic meetings. Use 2-4 bullets so a reader can scan the outcome.
- Do not use tables for decisions, actions, risks, or corrections by default. Use titled bullets and checklists; tables are only for metadata or short comparisons.
- Do not put long values into the metadata table. Slack Canvas table width follows the longest cell, so `Participants`, `Source`, `Tracking`, and `Access` must be summaries such as `플랫폼담당 외 4명`, `붙여넣은 화자분리 전사본`, `Slack List / issue tracker / 런북`, and `#sample-platform-team`; put exact long detail below in `### 메타데이터 메모`.
- Do not put callouts, tables, or code blocks inside layout columns. Slack Canvas does not support those nested combinations reliably.
- Do not create a skeleton Canvas and stop there. If a fallback skeleton is created, it must be followed by section updates until the full readable meeting Canvas is present.
- Do not hide uncertainty in decisions. Keep uncertain speaker mappings, term mappings, policy details, and architecture assumptions in `리스크/열린 질문` or the correction appendix.
- Do not put meeting-index maintenance inside the meeting Canvas. The Canvas is for meeting content; Slack List binding happens separately as an automatic existing-List registration for published meeting Canvases, with manual handoff only after verified API/permission failure.
- Do not replace the verbatim transcript with representative excerpts unless a hard Slack/API limit forces a degraded output and the output is explicitly labeled `원문 전사본 발췌`.
