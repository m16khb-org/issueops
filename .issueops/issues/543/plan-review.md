# 계획 검토 기록

## 1차: 수정 요청

- gpt-6-astra xhigh 독립 검토: 현재 risk 실행은 plan JSON을 stdout 앞에 두어 8KiB tail 절삭 시 기준·HEAD 근거를 잃는다. NormalizePaths의 TrimSpace도 NUL로 읽은 파일명 양끝 공백·newline을 바꾼다. 두 보존 경계와 회귀 검증을 계획에 명시해야 한다.

## 2차: 통과

- gpt-6-astra xhigh fresh delta 검토: 성공·실패 8KiB 초과 출력에서도 기준·HEAD와 scope failure를 보존하는 회귀 기준이 tail 절삭 결함을 다룬다. collector부터 classifier까지 실제 경로를 유지하고 양끝 whitespace로만 다른 경로를 구별하는 검증이 TrimSpace 결함을 다룬다. 기존 분류기·공용 CLI/MCP scope·기준 미지정 호환·prepared base 호출 연결과 파일 소유권도 유지되어 필수 결함이 없다.
