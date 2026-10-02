# Omo: context compaction, memory, and skill loading

Retrieval date: **2026-10-02**. Scope: read-only investigation; no installation, configuration, authentication files, or session transcripts inspected.

## Provenance and method

Canonical Omo project: [code-yeongyu/oh-my-openagent](https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.8), established by `/Users/habin/node_modules/omo-ai/package.json:2-26`: installed `omo-ai` **5.1.8**, depending on `@code-yeongyu/senpi` **2026.10.1-2**. The engine manifest identifies its separate canonical repository, `code-yeongyu/senpi`.

For local anchors below, **R** means `/Users/habin/.omo/agent/runtime/a1700c8985bbd0c8-ef3ba637dc3d/dist`. These describe this installed engine, not every Omo edition.

Source fan-out: official release notes/metadata, installed package provenance, and targeted engine implementation reads. Source index: [v5.1.8 notes](https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.8), [v5.1.9 notes](https://github.com/code-yeongyu/oh-my-openagent/releases/tag/v5.1.9), and [release metadata](https://api.github.com/repos/code-yeongyu/oh-my-openagent/releases?per_page=5); all retrieved 2026-10-02. Metadata gives publication times **2026-10-01 15:00:33 UTC** and **2026-10-02 01:13:10 UTC**, respectively. v5.1.9 names engine **2026.10.1-3**; it is published, but not the inspected installation.

Claim verification: official claims remain single-source; installed code supplies direct implementation evidence, not independent corroboration. No independent benchmark or runtime experiment was performed.

## Findings

### 1. Claude compaction has a provider-resident context boundary

**Official, single-source; high certainty about the release statement.** v5.1.8 says manual `/compact` on a Claude subscription replaces the resident Claude transcript with the summary, making the next request use smaller context. v5.1.9 separately reports recovery when a lost Claude session rejects a re-sent conversation as too long.

**Applicability:** IssueOps visibility should distinguish a completed local checkpoint from provider-context replacement and recovery. **Counterevidence/limit:** neither statement establishes universal behavior across providers, automatic compaction, or skill preservation. No measured latency or token-saving percentage follows from these notes.

### 2. Skill discovery limits disk reads before loading bodies

**Direct installed-code evidence; high certainty.** `R/core/skill-discovery.js:4-14,78-118` reads an 8,192-byte prefix for frontmatter, falling back to the remaining file when the closing delimiter is absent. Hidden directories, `node_modules`, and `.git` are skipped. `R/core/skills.js:126-170` stops recursion at a directory containing `SKILL.md`.

**Applicability:** retain metadata-first discovery when considering IssueOps startup optimization; measure discovery separately from body loading. **Counterevidence:** embedded executable assets fall back to whole-file reads when file descriptors are unavailable; oversized frontmatter also defeats the prefix-only path. This is reduced potential I/O, not a measured speedup.

### 3. The prompt carries metadata and shared root aliases, not skill bodies

**Direct installed-code evidence; high certainty.** `R/core/skills.js:268-323` formats names, descriptions, and locations under a shared `rN` root table and instructs the model to read matching files. `disable-model-invocation` skills are excluded. `R/core/skills.js:341-385` deduplicates canonical file paths and emits winner/loser diagnostics for name collisions.

**Applicability:** expose discovery collisions and loaded-body status separately; preserve shared-path encoding before adding another cache. **Counterevidence:** metadata visibility does not mean the body has been read, and diagnostics do not guarantee the user sees a warning. Token savings depend on actual path lengths and skill counts.

### 4. Post-compaction restoration is bounded label recovery

**Direct installed-code evidence; high certainty.** `R/core/extensions/builtin/compaction/restoration-tracker.js:16-34,79-176` tracks file operations and `skill`/`load_skill` names. Skill content is the name; file content is the path plus operations. Selection prioritizes modified files over skills over read-only files. Defaults cap selection at ten items, with a budget limited by 50,000 tokens, 15% of context, and remaining context after reserve.

`R/core/extensions/builtin/compaction/index.js:673-703,921-923` prepares restoration after accepted compaction and consumes it before the next agent start. **Applicability:** report selected/omitted labels and budget exhaustion. **Counterevidence:** this mechanism does not restore full bodies; reading `SKILL.md` records a file label. The payload has `display: false`, and previously restored labels are excluded (`restoration-tracker.js:49-75,131-150`).

### 5. Persistent-memory recall improvements are separate from compaction

**Official, single-source; medium certainty about operational benefit.** v5.1.8 documents character-wise Chinese/Japanese matching and optional synonym/multilingual query expansion. Expansion defaults off; added terms have lower weight, while notes matching every original query word remain first.

**Applicability:** treat recall quality and compaction continuity as separate measurements, including multilingual queries. **Counterevidence:** the notes provide neither Korean-specific results nor evidence that persistent recall preserves dynamically loaded skills.

## EXPAND

- **EXPAND-CONTINUITY:** controlled repeated-compaction test for read-loaded skill bodies, omitted labels, and reloading correctness.
- **EXPAND-VISIBILITY:** inspect existing UI/log exposure of restoration budget, collision diagnostics, and provider replacement outcomes.
- **EXPAND-PERFORMANCE:** benchmark prefix discovery and multilingual recall with fixed fixtures; no performance percentage is established here.

Access boundary: no cited source was inaccessible. Retrieval limitations: oversized release/API and feature-document tool output was truncated; public release metadata was recovered through a structured read-only fetch. One JSON parse failed on truncated API output; an existence probe returned ENOENT for the not-yet-created report. No shell command failed. Tests/build are not applicable to this prose-only evidence lane.
