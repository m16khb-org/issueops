# 사이클 보드

`issueops list --json` 출력을 자체 완결 HTML 한 파일로 바꿔 멈춘 사이클을 한눈에 본다.
읽기 전용이다. 표의 "다음 명령"은 문자열만 보여 주고 실행하지 않는다.

```bash
issueops list --json | python3 skills/issueops/scripts/render_board.py --out <임시 디렉터리>/board.html
```

- 출력은 저장소 밖 임시 디렉터리에 쓴다. 저장소 안에 두지 않는다.
- 표 위 한 줄 요약의 `표시 N / 전체 M`을 먼저 본다. 전체는 state에 있는 모든 기록 수다(`scanned_records`).
  다른 repo의 기록과 읽기 실패한 기록도 센다. 그래서 `list --repo`로 거른 목록은 표시 수가 전체보다 적은 것이 정상이다.
- 정렬은 `idle_days` 내림차순(값이 없으면 뒤), 같으면 `id` 오름차순이다.
- 입력이 깨졌거나 `ok`가 true가 아니면 stderr 한 줄과 종료 코드 1이고 파일을 만들지 않는다.
- Python 표준 라이브러리만 쓴다. JS, 외부 URL, 상주 서버가 없다. `https` 링크만 `<a>`가 된다.
- 타임라인: `issueops view --id ID --view timeline --json | python skills/issueops/scripts/render_board.py --out "$(mktemp -d)/timeline.html"`
- 그래프: `issueops view --id ID --view graph --json | python skills/issueops/scripts/render_board.py --out "$(mktemp -d)/graph.html"`
- graph는 `next` 판정을 쓰므로 그 사이클의 저장소에서(또는 `--cwd`로 그 저장소를 지정해) 실행한다.
