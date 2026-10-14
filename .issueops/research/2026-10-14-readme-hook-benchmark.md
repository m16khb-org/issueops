# Research: IssueOps README의 첫 화면 문구는 비슷한 오픈소스와 비교해 적절한가

## TL;DR

- 결론: "저장소에 기록해 git으로 공유한다"는 문구만으로는 차별점이 되지 않는다. Kage와 OwnMem이 거의 같은 문장을 첫 화면에 쓰고 있다. IssueOps만의 내용은 기록이 이슈부터 PR까지의 사이클 안에서 관문으로 강제되고, 바뀐 코드와 같은 PR에 담긴다는 점이다. 첫 화면 문구를 이 내용으로 바꿨다.
- 신뢰도: Medium. README 작성 관행은 여러 독립 출처가 일치하지만, 모두 의견이나 관찰이며 효과를 측정한 자료는 없다.
- 출처: 독립 출처 17개. 단일 출처 주장 2개, 이견이 있는 주장 0개.

## Method

- 조사 방향 세 가지: (1) 같은 문제를 다루는 프로젝트(에이전트 메모리, ADR 도구), (2) 널리 쓰이는 개발 도구 README의 첫 화면, (3) README 작성 가이드와 1차 문서.
- 가져온 출처: GitHub REST API(`gh api repos/<repo>/readme`)로 README 18개를 받았고, 웹 문서 4개를 읽었다. 모두 2026-10-14에 가져왔다.
- 교차 검증: 관행에 관한 주장은 저장소 README 2개 이상과 가이드 1개 이상이 일치하는지 확인했다. 에이전트 메모리의 공유 범위는 공식 문서와 제3자 글로 이중 확인했다.
- 한계: 별도의 반박 검토 서브에이전트는 돌리지 않았다. 반박 검토는 직접 했으므로 독립 검토가 아니다.

## Findings

1. **한 줄 소개는 "범주 + 대상" 명사구가 주류다.** OwnMem은 "Git-native memory for AI coding agents", beads는 "A memory upgrade for your coding agent", Backlog.md는 "Markdown-native Task Manager & Kanban visualizer for any Git repository", uv는 "An extremely fast Python package and project manager, written in Rust"를 쓴다. Backlog.md는 그 아래에 결과를 말하는 두 번째 줄("AI agents write the code. You review the tasks")을 둔다. thoughtbot 가이드도 첫머리에 굵은 글씨로 무엇을 하는지 짧게 쓰라고 권한다. 상태: Confirmed. 신뢰도: Medium.
2. **설치 명령이 첫 화면에 있다.** Backlog.md(`npm i -g backlog.md`), Kage(`npx -y @kage-core/kage-graph-mcp install`), beads(`curl ... | bash`), Codex CLI(`curl ... | sh`)가 그렇다. makeareadme는 설치 단계와 요구 환경을 구체적으로 적으라고 권한다. IssueOps는 Draft 릴리스 때문에 clone과 Go 빌드가 필요하다. 상태: Confirmed. 신뢰도: Medium.
3. **첫 화면 근처에 실제 결과물을 보여 준다.** Kage는 "A real receipt from this repo's own run history"라며 실제 실행 결과를, uv와 Backlog.md는 실행 화면 GIF를, OwnMem은 재현 명령을 붙인 벤치마크 그림을 보여 준다. makeareadme도 예상 출력을 보여 주라고 권한다. 상태: Confirmed. 신뢰도: Medium.
4. **지시 파일과 기본 메모리를 표로 비교한다.** OwnMem은 "How it compares" 표에서 instruction files, built-in agent memory, OwnMem을 누가 쓰는지, 어디에 남는지, 무엇이 모델에 전달되는지 기준으로 비교한다. Dosu 글은 같은 내용을 지시 파일, 공유 검색, 메모리 서비스라는 세 층으로 나눈다. makeareadme는 대안이 있으면 차별점을 적으라고 권한다. 상태: Confirmed. 신뢰도: Medium.
5. **"저장소에 남겨 git으로 공유"는 이미 흔한 문구다.** Kage는 "lives in your repo as plain files, shared via git", OwnMem은 "The repository owns memory", memorix는 "Local-first shared memory layer", beads는 git 기반 이슈 추적기를 내세운다. Kage는 `kage merge`가 에이전트가 배운 것을 반영한다고 설명하므로 IssueOps와 가장 가깝다. 상태: Confirmed. 신뢰도: Medium.
6. **같은 PR에 결정 기록을 남기라는 권고가 있다.** Dosu 글은 ADR을 변경과 같은 pull request에서 쓰라고 권한다. IssueOps는 사이클의 6단계(문서 반영)가 커밋과 PR 발행보다 앞서고, `project_docs_review`가 없으면 다음 단계로 넘어가지 않는다(`internal/domain/issueopsreview/gates.go`, `internal/domain/issueopsnext/classify.go`). 상태: Single-sourced(외부 권고는 Dosu 한 곳). 신뢰도: Low~Medium.
7. **Claude Code의 자동 메모리는 컴퓨터마다 따로 저장된다.** 공식 문서는 "Auto memory is machine-local ... Files are not shared across machines or cloud environments"라고 쓰고, Dosu 글도 같은 내용을 인용한다. 같은 문서는 `CLAUDE.md`를 200줄 이하로 유지하라고 권한다. 상태: Confirmed. 신뢰도: High(1차 문서).
8. **자기 도구로 자기 프로젝트를 개발한다는 사실을 밝힌다.** Backlog.md는 "Dogfooded: nearly all of Backlog.md's own code is written by AI agents working through Backlog.md itself"라고 쓰고 작업 기록 폴더를 링크한다. 상태: Single-sourced. 신뢰도: Low.
9. **첫 화면을 밀어내는 요소가 많다.** claude-mem은 언어 링크 30여 개, memorix는 지원 호스트 18개 이상, basic-memory는 이름보다 앞에 가격 배너를 둔다. 첫 화면 문구가 아래로 밀린다는 평가는 조사자의 판단이다. 상태: 관찰은 Confirmed, 효과는 unverified.

## Cross-Check Results

- Confirmed 6개(1, 2, 3, 4, 5, 7), Single-sourced 2개(6, 8), 효과 미검증 1개(9).
- README 관행에 관한 주장은 모두 관찰이나 의견이다. 별 수와 README 형식 사이의 인과는 확인할 수 없다.

## Adversarial Review

- 반박 시도: "같은 PR에 담긴다"는 사이클을 쓸 때만 성립한다. 사이클 없이 `project append`만 쓰면 사용자가 따로 커밋할 수 있다. README 본문의 「사이클 없이 쓰기」가 이 경우를 따로 설명하므로 한 줄 소개에는 사이클 경로 기준으로 썼다.
- 반박 시도: Kage도 병합 시점에 학습 내용을 반영한다. 따라서 "PR과 함께"만으로 유일하다고 주장하지 않고, 계획 섹션 게이트와 문서 반영 판정을 함께 적었다.
- 반박 시도: Kage와 OwnMem은 기록을 실제 코드와 대조해 낡은 기록을 걸러 낸다. IssueOps에는 이 기능이 없으므로 README에서 그런 주장을 하지 않는다.
- 독립 검토자는 없었다.

## Disputed / Unresolved

- 없음.

## Open Questions

- 사전 빌드 바이너리를 공개해 설치를 한 줄로 줄일 수 있는가(Draft `v0.1.0` 공개 여부).
- `docs/assets/issueops-hero.png`는 "에이전트들이 코어 하나를 공유한다"는 구조를 그리므로 README 첫 화면에서 「아키텍처」 절로 옮겼다. 첫 화면에는 Codex 내장 `image_gen`으로 만든 `docs/assets/issueops-hero-team-memory.png`(기록이 PR과 저장소를 거쳐 팀원의 에이전트에게 가는 흐름)를 넣었다.
- GitHub 저장소 설명이 아직 "issue-driven agent workflows for Codex, Claude Code, and Omo"다.
- 기록이 실제 코드와 어긋났을 때 이를 감지하는 기능이 필요한가(Kage, OwnMem과의 기능 차이).

## Source Index

| # | URL | 설명 | 유형 | 가져온 날짜 | 신뢰도 |
|---|---|---|---|---|---|
| 1 | https://github.com/kage-core/Kage | Kage README | 저장소 README | 2026-10-14 | Medium |
| 2 | https://github.com/grpcer/ownmem | OwnMem README | 저장소 README | 2026-10-14 | Medium |
| 3 | https://github.com/steveyegge/beads | beads README | 저장소 README | 2026-10-14 | Medium |
| 4 | https://github.com/MrLesk/Backlog.md | Backlog.md README | 저장소 README | 2026-10-14 | Medium |
| 5 | https://github.com/thedotmack/claude-mem | claude-mem README | 저장소 README | 2026-10-14 | Medium |
| 6 | https://github.com/AVIDS2/memorix | memorix README | 저장소 README | 2026-10-14 | Medium |
| 7 | https://github.com/basicmachines-co/basic-memory | basic-memory README | 저장소 README | 2026-10-14 | Medium |
| 8 | https://github.com/astral-sh/uv | uv README | 저장소 README | 2026-10-14 | Medium |
| 9 | https://github.com/github/spec-kit | Spec Kit README | 저장소 README | 2026-10-14 | Medium |
| 10 | https://github.com/openai/codex | Codex CLI README | 저장소 README | 2026-10-14 | Medium |
| 11 | https://github.com/Aider-AI/aider | aider README | 저장소 README | 2026-10-14 | Medium |
| 12 | https://github.com/npryce/adr-tools | adr-tools README | 저장소 README | 2026-10-14 | Medium |
| 13 | https://github.com/thomvaill/log4brains | log4brains README | 저장소 README | 2026-10-14 | Medium |
| 14 | https://thoughtbot.com/blog/how-to-write-a-great-readme | How To Write A Great README (2013, 2020 갱신) | 가이드 | 2026-10-14 | Medium |
| 15 | https://www.makeareadme.com/ | Make a README | 가이드 | 2026-10-14 | Medium |
| 16 | https://dosu.dev/articles/team-memory-for-coding-agents | Team Memory for Coding Agents (2026-09-24, 판매사 글) | 기술 글 | 2026-10-14 | Medium |
| 17 | https://code.claude.com/docs/en/memory | How Claude remembers your project | 공식 문서 | 2026-10-14 | High |
