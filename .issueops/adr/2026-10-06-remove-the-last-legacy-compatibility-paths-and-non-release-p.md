---
name: 2026-10-06-remove-the-last-legacy-compatibility-paths-and-non-release-p
description: Accepted decision record with rationale, alternatives, and consequences.
---

# Remove the last legacy compatibility paths and non-release platform code

- Date: 2026-10-06
- Kind: `adr`
- Source: cli
- Summary: 의도적으로 남겨 두었던 호환 경로까지 모두 지운다: Orca completed_at v1 포맷 파서, OrcaWorkerDoneClient/SendWorkerDone, project docs bootstrap의 legacyFlat 보존, darwin/linux 이외 플랫폼 stub, remote score 입력의 related_issues/labels alias, 이미 사라진 파일을 가리키는 DDD inventory policy.
- Context: 사용자가 legacy와 dangling을 하나도 남기지 말라고 지시했다. 같은 날 앞선 ADR(2026-10-06-remove-openwiki-and-the-remaining-legacy-test-only-and-unsup)은 completed_at fallback과 !darwin&&!linux stub을 남겼고, OrcaWorkerDoneClient는 #127 보존 결정으로, legacyFlat은 2026-08-21 bootstrap-respects-inprogress-repos 결정으로 남아 있었다. 재감사(deadcode ./cmd/..., deadcode -test ./..., production 참조 0 exported 심볼 스캔, 현행 문서 경로 대조)에서 이 항목들과 참조 0인 alias, test-only 노출, 사라진 파일을 가리키는 inventory policy 226건이 추가로 나왔다. Orca 1.4.221은 모든 task의 completed_at을 RFC3339(...Z)로 내보낸다.
- Decision: 1) completed_at은 RFC3339/RFC3339Nano만 받고, 그 밖의 값은 orca_task_timestamp_invalid로 fail-closed한다. 2) OrcaWorkerDoneClient, OrcaWorkerDoneRequest/Result, Client.SendWorkerDone을 지운다. 3) bootstrap의 legacyFlat 분기와 legacy_flat_layout_preserved 경고를 지운다. manifest가 없는 repo도 일반 모듈형 bootstrap을 타고, 기존 family 파일은 --sync에서도 덮어쓰지 않는다(family_docs_preserved). 4) darwin/linux 이외 stub과 항상 nil이던 requireSupportedPlatform을 지우고, handoff delivery 경로는 runtime.GOOS 분기 하나로 합친다. release-build-matrix.sh의 windows 분기도 지운다. 5) DecodeIssueOpsRemoteScoringRequest를 지우고 json.Unmarshal로 읽는다. 옛 키는 다른 모르는 키처럼 무시된다. 6) DDD inventory에서 사라진 source_path policy를 지우고, 아키텍처 테스트가 source_path와 evidence 테스트의 실재를 검사하게 한다. 7) 참조 0 alias(PassPowKPoint, ReliabilityReport), test-only Clone 메서드, ConformanceProbeConfig.ProductionDispatch, 중복 계약 테스트 파일을 지우고 StructuredPromptSectionHeadings는 export_test.go로 옮긴다. 8) 루트 plans/, .issueops/drafts/, 루트 PROMPT.md를 지운다.
- Consequences: 앞선 ADR 2026-10-06-remove-openwiki...의 completed_at fallback 유지와 !darwin&&!linux stub 유지 결정, 2026-08-21 bootstrap-respects-inprogress-repos의 legacy flat 보존 결정, #127의 SendWorkerDone 보존 결정을 대체한다. 옛 Orca(공백 구분 UTC completed_at)를 쓰면 operational health가 timestamp invalid를 보고한다. flat family root만 있고 manifest가 없는 대상 repo는 bootstrap이 manifest와 overview를 새로 만든다. freebsd 등 비 release 플랫폼 빌드는 실패한다. 검증: go build/vet ./..., linux·darwin × amd64·arm64 빌드, deadcode(./cmd/..., -test ./...) 빈 결과, go test -p 4 ./... -count=1 (캐시 누락으로 빌드 실패한 9개 패키지는 재실행해 통과).
