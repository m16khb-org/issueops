# Operating Documentation Architecture

This directory defines how `.issueops/` operating knowledge is divided,
navigated, and validated, and how repository-owned skill documentation is
audited when it is included in the requested scope.

## Design goals

- Keep required project-doc entrypoints stable.
- Give every rule and decision one canonical owner.
- Make targeted context retrieval possible without loading unrelated history.
- Keep indexes concise and detailed modules independently reviewable.
- Preserve all current information while moving it; do not summarize away
  operational constraints.

The measured starting point and responsibility analysis are in
[`AUDIT.md`](AUDIT.md). The machine-readable ownership contract is
[`manifest.json`](manifest.json).
The current cross-skill/project-doc review is
[`quality-audit-2026-10-03.md`](quality-audit-2026-10-03.md).

## Navigation model

Required root documents remain at their existing paths because the runtime
project-doc contract discovers those exact filenames:

```text
.issueops/
├── ADR.md
├── ARCHITECTURE.md
├── CAUTIONS.md
├── CONVENTIONS.md
├── OPERATIONS.md
├── TESTING.md
├── adr/
├── architecture/
├── archive/
├── cautions/
├── conventions/
├── operations/guides/
├── testing/
└── documentation/
```

Each family root listed in the manifest is a canonical index. It owns:

1. the short normative summary needed by every agent;
2. links to responsibility-specific modules;
3. update instructions for that document family.

Detailed modules own procedures, rationale, examples, and historical records.
They link back to their family index and do not duplicate another family's
normative rules.
Other required entrypoints, such as the constitution, keep their standalone
contract; do not replace them with thin redirects merely to match this layout.

## Ownership map

| Topic | Canonical owner |
|---|---|
| accepted architecture decisions | `ADR.md` and `adr/decisions/` |
| dependency direction and runtime topology | `ARCHITECTURE.md` and `architecture/` |
| known risks and incident lessons | `CAUTIONS.md` and `cautions/` |
| implementation and interface conventions | `CONVENTIONS.md` and `conventions/` |
| installation and runtime operation | `OPERATIONS.md` and `operations/` |
| test strategy and verification gates | `TESTING.md` and `testing/` |
| commit formatting | `COMMIT_POLICY.md` |
| OpenAPI requirements | `OPEN_API_SPEC.md` |
| technology selection | `TECH_STACK.md` |
| agent execution sequence | `AGENT_WORKFLOW.md` |
| constitutional priority and safety | `CONSTITUTION.md` |
| issueops historical audit | `archive/issueops-audit.md` |
| whole-project audit snapshot | `PROJECT_AUDIT.md` (root-retained exception) |

References outside the canonical owner carry only a link plus
workflow-specific context.

Two dated audit snapshots are records, not operating documents.
`PROJECT_AUDIT.md` stays at its root path because `quality inspect` parses it
in place (`cmd/issueops/qualitycli/quality_inspect.go`, `collectAuditItems`)
for the `audit-p0-p1-p2-items` signal and quality-catalog candidates cite it
as evidence. `archive/issueops-audit.md` is the retired IssueOps audit kept
verbatim under `archive/`.

## Size and structure budgets

- Manifest-listed root index: at most 250 lines.
- Detailed module in a manifest-listed family: at most 250 lines.
- One module owns one responsibility.
- One ADR file owns one accepted decision.
- One dated caution lesson file owns one incident lesson or tightly coupled
  incident set.
- A module that crosses the line budget must be split by responsibility, not by
  arbitrary part numbers.

The line budget is a retrieval boundary, not a reason to delete detail.
Standalone machine-input contracts outside those families require an explicit
consumer/preservation assessment before splitting; a passing family checker
does not certify every Markdown file in the repository.

## Skill-document quality

- Inventory all owned `skills/<name>/SKILL.md` files and their references;
  distinguish external links, fixtures, generated pages, and historical evidence.
- Preserve frontmatter routing, body-only actor instructions, output fields,
  authorization, refusal/no-input/no-change branches, and stop rules.
- Prefer concise roots and optional examples within the same skill. A 250-line
  entrypoint is a design target, not permission to move mandatory contracts
  beyond an isolated actor's input.
- Resolve repository-level evaluator links from the real source location.
  Missing optional evaluator material must not block ordinary skill execution;
  missing required evaluation evidence prevents a formal quality verdict.
- Record before/after lines, words, reference/link results, scope changes and
  semantic duplication decisions. Do not call smaller prose a measured task
  success-rate or latency improvement.
- Validate all owned skills and local reference paths separately from the
  project-doc checker. Use an evidence-bound preservation review for changed
  contracts; metadata signatures alone do not prove semantic quality.

## Folder contracts

### `adr/`

- `README.md`: decision statuses, naming, and index
- `roadmap.md`: implementation roadmap that remains current
- `decisions/YYYY-MM-DD-<slug>.md`: immutable accepted decision record

### `architecture/`

- `hexagonal-core.md`: domain, application, port, and adapter boundaries
- `runtime.md`: MCP, state, process, and lock topology
- `host-integration.md`: Codex and Claude thin-adapter design
- `issueops.md`: IssueOps capability verticals and ownership

### `cautions/`

- `runtime.md`: process, worker, lock, and state risks
- `security.md`: secrets, command policy, and trust boundaries
- `integrations.md`: host, hook, MCP, remote, and external-tool risks
- `audit-and-process.md`: audit interpretation and verification-process risks
- `issueops-lifecycle.md`: IssueOps state and lifecycle risks
- `issueops-orchestration.md`: IssueOps coordination and provider risks
- `issueops-execution.md`: IssueOps execution and cleanup risks
- `lessons/YYYY-MM-DD-<slug>.md`: dated incident lesson

### `conventions/`

- `go-and-packages.md`: Go, package, contract, port, and adapter conventions
- `cli-mcp-and-output.md`: CLI/MCP schemas and response contracts
- `state-policy-and-hooks.md`: state, guards, policy, hook, and lifecycle rules

### `operations/guides/`

- `operations/guides/install-and-update.md`: installation, bootstrap, update, and rollback
- `operations/guides/cli-and-state.md`: CLI discovery and state lifecycle
- `operations/guides/skills-and-hosts.md`: native skills and host activation
- `operations/guides/troubleshooting.md`: diagnosis and recovery
- `operations/guides/issueops-providers.md`: IssueOps preparation and provider contracts
- `operations/guides/issueops-execution.md`: IssueOps execution and recovery

Existing canonical siblings under `operations/` continue to own MCP,
host, release, and installation procedures linked by `OPERATIONS.md`.

### `testing/`

- `unit-and-contract.md`: unit, integration, fixtures, goldens, and contracts
- `concurrency-and-race.md`: race, process, lock, and nondeterminism rules
- `cli-mcp-and-hosts.md`: CLI, MCP, Codex, and Claude parity
- `self-verification.md`: single-pass self-verification contract
- `api-documentation.md`: OpenAPI static and agent review gates
- `issueops-execution.md`: IssueOps and Orca execution verification

### `archive/`

Retired dated snapshots moved verbatim from living documents:

- `adr-history.md`: superseded ADR history
- `cautions-incidents.md`: superseded incident ledger
- `issueops-audit.md`: retired IssueOps audit snapshot (moved from
  `.issueops/ISSUEOPS_AUDIT.md` on 2026-08-20)

## Link rules

- Use repository-relative Markdown links.
- A root index links to every module in its family.
- Every module links back to its root index.
- Cross-family links target the canonical owner, not a duplicate summary.
- Links to source code use repository-relative paths and line-independent symbol
  names when possible.
- OpenWiki output is not edited as part of this documentation system.

## Update workflow

1. Identify the canonical owner in `manifest.json`.
2. Update one module; create a new decision or lesson record when history must
   remain immutable.
3. Update the family root index only when navigation or the universal summary
   changes.
4. Run the documentation-optimization skill validator.
5. Run `issueops docs --json` and the documented command smoke checks.
6. Review the diff for accidental duplication or information loss.

## DDD 책임 문서 분리 (2026-09-30)

[domain-responsibilities.md](../architecture/domain-responsibilities.md)는 capability별 책임을,
[issueops-cleanup.md](../architecture/issueops-cleanup.md)는 실행과 정리의 효과 순서를 소유한다.
공통 의존 방향과 사이클 권한은 기존 architecture 모듈에 유지한다.
