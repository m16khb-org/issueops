---
name: 2026-10-07-ci-home-runner-rg-worktree-lint-cache-binary
description: Caution record for a solved false case or recurring risk.
---

# CI 임시 HOME 설치, runner의 rg 부재, worktree 간 lint cache, 상한 변경 중의 옛 binary

- Date: 2026-10-07
- Kind: `caution`
- Source: issueops-docs #550
- Summary: CI의 임시 HOME 설치는 HTTP supervisor unit을 띄우지 못하고, GitHub runner에는 ripgrep이 없으며, golangci-lint 공유 cache는 지워진 worktree의 결과를 돌려줄 수 있고, 상한을 바꾸는 사이클에서는 설치된 main binary가 옛 상한으로 기록을 거부한다.
- Context: #550(2026-10-07)에서 main CI의 install 실패를 고치는 동안 네 가지를 차례로 만났다. (1) CI는 HOME을 mktemp 디렉터리로 바꿔 install-native.sh를 실행하는데, systemctl --user daemon-reload는 성공하고 enable이 'Unit file issueops-mcp.service does not exist'로 실패했다. installer는 unit을 바꾼 HOME 아래에 쓰지만 이미 실행 중인 user manager는 자기 HOME 기준 경로에서 찾는다. 그 전까지 supervisor는 출력을 버리고 exit status만 남겨 원인이 보이지 않았다. (2) install이 풀리자 2026-10-03 이후 처음 돈 self-verify Python 단계에서 pr-review의 rg 기반 정의 검색 테스트가 실패했다. runner에 rg가 없어 _rg_symbol이 빈 목록을 돌려줬다. (3) 로컬 golangci-lint가 지워진 #548 worktree 경로의 SA1012 결과를 돌려줬다. (4) revise 상한을 3에서 5로 올리는 이 사이클에서, 설치된 main binary가 네 번째 비-waived revise를 옛 상한 3으로 거부했다.
- Resolution: (1) supervisor load 에러가 실패한 명령과 출력 끝 2048바이트를 담게 했고, CI install은 ADR 2026-10-02에 따라 --mcp-transport=stdio를 명시한다. HOME을 바꾼 Linux 설치의 결함 자체는 후속으로 남겼다. 진단 커밋은 단독으로 push하고 그 run이 끝날 때까지 다음 push를 하지 않았다(ci.yml concurrency가 진행 중 run을 취소한다). (2) 스킬 스크립트가 rg 같은 선택 도구를 쓰면 없을 때의 fallback을 둔다. mr_context.py는 rg가 FileNotFoundError면 grep -rnwFI에 .git·바이너리·명시 glob 제외를 붙여 찾는다. rg 없는 PATH(python3, git, grep만 링크한 임시 디렉터리)에서 테스트를 돌려 확인한다. (3) 결과에 다른 worktree 경로가 보이면 golangci-lint cache clean 뒤 다시 실행한다. (4) 상한이나 게이트를 바꾸는 사이클에서 설치된 binary가 옛 규칙으로 거부하면, 근거를 적은 --waive --waiver-rationale로 기록하고 지적은 고친 뒤 다음 라운드로 해소를 확인한다. worktree build로 우회해 기록하지 않는다.
- Evidence:
  - https://github.com/m16khb-org/issueops/actions/runs/37595246857
  - https://github.com/m16khb-org/issueops/actions/runs/37596145318
  - .issueops/issues/550/ci-diagnosis.md
  - internal/adapter/mcpservice/supervisor.go
  - skills/pr-review/scripts/mr_context.py
  - skills/pr-review/tests/test_context_pack.py
