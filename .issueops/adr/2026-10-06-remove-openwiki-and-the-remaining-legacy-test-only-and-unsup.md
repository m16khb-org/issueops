---
name: 2026-10-06-remove-openwiki-and-the-remaining-legacy-test-only-and-unsup
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Remove OpenWiki and the remaining legacy, test-only, and unsupported-platform code

- Date: 2026-10-06
- Kind: `adr`
- Source: cli
- Summary: 생성형 openwiki/와 자동 갱신 워크플로, 제거된 draft-wiki staging 디렉터리, 숨은 self_augment_* MCP alias, Orca 집계 인터페이스와 그것만 붙잡던 adapter 메서드, test-only production 코드, 어떤 release 대상에서도 컴파일되지 않는 windows/!unix stub을 지운다.
- Context: 33915907(daemon 제거)·5d90885a(legacy 호환 경로 제거)·57091827(dead/test-only 코드 제거) 뒤 재감사(deadcode ./cmd/..., production 참조 0 exported 심볼 스캔, 문서·fixture 참조 대조)에서 잔재가 남았다. openwiki/는 agent-harness 시절 내용(agent-harness 611회, daemon 22파일)을 현행처럼 설명했고, --field 도움말은 지운 alias를, project usage는 없는 draft-wiki 명령을 안내했다. release matrix(scripts/release-build-matrix.sh)는 darwin/linux × amd64/arm64이고 GOOS=windows 빌드는 untagged 코드(installutil/activation.go syscall.Stat_t)에서 이미 실패한다.
- Decision: 1) openwiki/와 .github/workflows/openwiki-update.yml, AGENTS.md·CLAUDE.md의 OPENWIKI 블록을 지운다. 코드 설명은 .issueops 문서가 소유한다. 2) .issueops/draft-wiki/를 지우고 docs index 제외 목록은 .issueops/evidence만 둔다(승인 후보는 cautions/integrations.md §20에 이미 반영, 나머지 draft는 git 이력에 남는다). 3) self_augment_history/compare/promote alias와 catalog advertised 필드를 지운다. self_verify_*만 남는다. 4) port.OrcaClient 집계와 그것만 붙잡던 Orca 메서드·role interface, capability_baseline 계약, RootClaim/RootConflict, DeleteBucket, 미사용 alias를 지우고 responsecontract·ProcessInspectorFunc는 internal/testsupport로 옮긴다. 5) RunID 없는 cycle을 taskID로 역조회하던 resolveLegacyCycleRun을 지운다(현재 writer는 RunID를 항상 기록하며 없으면 inventory_unknown으로 fail-closed). 6) *_windows.go와 !unix stub을 지운다. !darwin&&!linux stub은 freebsd에서 컴파일되므로 남긴다. 7) project bootstrap --write를 지운다(write = !dry-run).
- Consequences: 외부 호출자가 self_augment_* alias나 project bootstrap --write를 쓰면 실패한다. RunID 없는 과거 cycle 레코드는 operational health에서 inventory_unknown이 된다. windows/비unix 빌드를 다시 지원하려면 stub부터 새로 설계해야 한다. Orca completed_at v1 포맷 fallback(task_timestamp.go)은 domain-responsibilities.md의 종료 조건(지원 Orca 전부 RFC3339Nano + release contract에서 legacy layout 제거)을 아직 충족하지 못해 유지한다. 검증: go build/vet ./..., GOOS=linux go build ./cmd/issueops, go test -p 4 ./... -count=1, deadcode ./cmd/... 출력 없음.
