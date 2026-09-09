# Gates: 507

- [x] G1: intent 렌더러가 결정적이고 9개 필드를 담으며 recorded_at을 내지 않는다
  CHECK: go test ./internal/domain/issueopsintent/ -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/domain/issueopsintent	0.226s
- [x] G2: prepare materialize가 intent.md를 0600으로 봉인하고 manifest에 digest를 넣으며 intent 없는 record는 manifest가 같고 내용이 바뀐 재봉인은 거부된다
  CHECK: go test ./internal/adapter/issueops/ -run Intent -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/issueops	9.211s
- [x] G3: RecordIntent가 콜론 형태 자격 증명을 거부하고 등호 형태는 종전대로 redacted 저장한다
  CHECK: go test ./internal/adapter/issueops/intentdesign/ -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/issueops/intentdesign	0.236s
- [x] G4: artifact stage --name intent가 거부된다
  CHECK: go test ./internal/domain/issueopsartifact/ -run NormalizeName -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/domain/issueopsartifact	0.390s
- [x] G5: owner prompt 템플릿과 prompt-engineering 문서 PROMPT 블록이 byte 동일하고 placeholder가 모두 해석된다
  CHECK: go test ./internal/adapter/issueops/ -run Prompt -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/issueops	0.299s
- [x] G6: 봉인된 generation은 템플릿 변경 뒤에도 durable digest로 resume된다
  CHECK: go test ./internal/adapter/issueops/ -run TemplateUpgrade -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/adapter/issueops	0.325s
- [x] G7: issueops-review 스킬이 검증을 통과한다
  CHECK: python3 scripts/validate-skill.py skills/issueops-review
  EXPECT: /Skill is valid/
  EVIDENCE: Skill is valid!
- [x] G8: issueops-verify 스킬이 검증을 통과한다
  CHECK: python3 scripts/validate-skill.py skills/issueops-verify
  EXPECT: /Skill is valid/
  EVIDENCE: Skill is valid!
- [x] G9: issueops-create-pr 스킬이 검증을 통과한다
  CHECK: python3 scripts/validate-skill.py skills/issueops-create-pr
  EXPECT: /Skill is valid/
  EVIDENCE: Skill is valid!
- [x] G10: CONVENTIONS의 artifact 목록이 intent를 포함한다
  CHECK: grep -n intent .issueops/CONVENTIONS.md
  EXPECT: /record\.intent에서 파생/
  EVIDENCE: 113:                     `execution.workspace.artifact_dir`가 이 경로를 영속한다(#482). intent는 | 114:                     staging이 아니라 prepare가 record.intent에서 파생한다(#
- [x] G11: execution.md의 Artifact Staging And Sealing 절이 intent 파생 규칙을 담는다
  CHECK: grep -n intent skills/issueops/references/execution.md
  EXPECT: /`record\.intent`/
  EVIDENCE: 412:  prepare를 다시 실행한다(child record는 `--intent-class delegated-child`를 유지한다). | 437:  모두 비교한다. 누락/drift는 operation ID, intent, terminal/Run/task/dispatch,
- [x] G12: ADR 파일이 존재하고 색인에 올라 있다
  CHECK: grep -n requester-intent .issueops/ADR.md
  EXPECT: /requester-intent-as-a-derived-artifact/
  EVIDENCE: 40:| 2026-09-09 | IssueOps seals the requester intent as a derived artifact next to the plan | [record](adr/2026-09-09-issueops-seals-the-requester-intent-as-a-derived-artifact.md) |
- [x] G13: 전체 테스트가 통과한다
  CHECK: python3 -c "import subprocess,sys; r=subprocess.run(['go','test','./...','-count=1'],capture_output=True); print('ALL_PASS' if r.returncode==0 else 'SOME_FAIL'); sys.exit(r.returncode)"
  EXPECT: ALL_PASS
  EVIDENCE: ALL_PASS
