---
name: 2026-11-12-a-failed-create-keeps-its-provider-command-and-http-status-a
description: Accepted decision record with rationale, alternatives, and consequences.
---

# A failed create keeps its provider command and HTTP status as structured fields

- Date: 2026-11-12
- Kind: `adr`
- Source: claude-code main session
- Summary: invoked_unknown으로 남는 issue·child create 실패는 자유 문자열에 더해 고정 enum failure_command와 정수 failure_http_status를 record에 남긴다.
- Context: 2026-11-03 조사에서 사이클 3건이 `glab issue create failed: ERROR 404 Not Found`로 3주째 invoked_unknown이었고, record의 failure 문자열만으로는 원인을 구조적으로 읽을 수 없었다. failure는 redaction을 거친 자유 문자열(최대 2048바이트)이다.
- Decision: 1) `port.IssueProviderCreateError`에 `Command`와 `HTTPStatus`를 더하고 GitHub·GitLab adapter가 invoked 실패에서 채운다. Command는 호출 지점의 상수다(`gh issue create`, `glab issue create`, `glab api graphql`). 2) HTTP 상태는 순수 함수 `remoteparse.ProviderHTTPStatus`가 redaction 뒤의 진단 문자열에서 뽑는다. `HTTP NNN:` 접두 형식은 그대로 받고, 접두 없는 코드는 표준 reason phrase가 정확히 뒤따를 때만 받는다. 400~599만 받고 서로 다른 코드가 둘이면 0이다. 3) `IssueCreateIntent`와 `IssueCreateOutcome`에 `failure_command`(허용 목록)와 `failure_http_status`를 더한다. child operation은 intent를 임베드하므로 같은 필드를 갖는다. `schema_version`은 1 그대로다. 4) outcome을 기록할 때 허용 목록 밖 값은 거부하지 않고 빈 값과 0으로 정규화한다. failure 문자열을 잃지 않기 위해서다. 재시도 시작과 완료는 두 필드를 지운다. 5) list에는 노출하지 않는다. `status --field issue_create_intent`로 읽는다.
- Consequences: record reader는 모르는 필드를 거부한다(`DisallowUnknownFields`). 이 변경 전에 빌드한 binary는 새 필드가 든 record를 invalid state로 거부한다. 머지 뒤 `issueops update`를 실행하고 호스트 세션을 다시 시작한다. 허용 목록은 늘리기만 한다. 값을 빼면 그 값이 든 기존 record를 읽지 못한다. 실제 gh·glab의 stderr 문구는 관측하지 못했고 문서화된 형식으로만 테스트했다.
- Evidence:
  - commit 39b78191c
  - internal/domain/remoteparse/provider_http_status.go, internal/contract/issueops/issue_create.go, internal/domain/issueops/issue_create_lifecycle.go
  - TestIssueCreateCLIRecordsFailureCause, TestChildCreateCLIRecordsFailureCause (fake gh/glab 404 through the production wiring)
  - .issueops/research/2026-11-03-optimization-quality-candidates.md section 7
- Alternatives / rejected options:
  - stderr 원문이나 URL을 구조화: redaction 대상이다
  - bounded runner가 stderr를 따로 돌려주게 변경: 공용 runner 시그니처가 바뀌고 모든 provider 호출부가 흔들린다
  - 쓰기 경로에서 허용 목록 밖 값을 거부: outcome 쓰기 전체가 실패해 원래 남던 failure 문자열까지 잃는다
  - invoked_unknown을 자동 재시도 가능하게 변경: ADR 2026-10-25가 그 거부를 유지한다
  - schema_version 올리기: additive 필드에는 올리지 않는다는 기존 결정(2026-07-07 caution)과 어긋난다
