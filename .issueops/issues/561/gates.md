# Gates: 561

- [x] G1: 품질 표 테스트가 실제 이슈 발췌의 선택과 기권을 고정한다
  CHECK: go test ./internal/domain/issueopsremote/ -run TestScoreIssueOpsRemoteLabelQualityTable -count=1 -v
  EXPECT: --- PASS: TestScoreIssueOpsRemoteLabelQualityTable
  EVIDENCE: PASS | ok  	issueops/internal/domain/issueopsremote	0.142s
- [x] G2: 검증 집합 232-560 단일 라벨 이슈에서 기본 threshold로 선택 50개 이상 정답 비율 85% 이상
  CHECK: python3 scripts/autoresearch/label_score_eval.py --binary bin/issueops --min-number 232 --max-number 560 --check
  EXPECT: PASS
  EVIDENCE: {"min_number": 232, "max_number": 560, "n_total": 116, "n_selected": 56, "n_correct": 54, "accuracy": 0.9643} | PASS
- [x] G3: 관련 이슈 점수 명시 score CLI remote score 동작이 유지된다
  CHECK: go test ./internal/domain/issueopsremote/ ./cmd/issueops/issueopscli/remotecmd/ ./cmd/issueops/issueopscli/ -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/cmd/issueops/issueopscli/remotecmd	11.561s | ok  	issueops/cmd/issueops/issueopscli	20.922s
- [x] G4: DDD 책임 원장이 새 심볼과 스크립트를 포함한다
  CHECK: go test ./internal/architecture/ -count=1
  EXPECT: ok
  EVIDENCE: ok  	issueops/internal/architecture	6.889s
