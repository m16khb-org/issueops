# Meeting Canvas 템플릿

## Meeting Canvas Template

Use this exact structure for every individual meeting Canvas. Keep section names and field order stable so the team can scan every meeting the same way:

````markdown
# YYYY-MM-DD [Topic] Title

> 회의일 YYYY-MM-DD · 대상 #sample-platform-team or {channel override} · Source {source} · Status {Draft/Follow-up 필요/완료/확인 필요}

## 메타데이터
|Field|Value|
|  ---  |  ---  |
| Date | YYYY-MM-DD |
| Topic | {배포/테스트품질/장애/온보딩/AI/인프라/정책/리뷰/확인필요} |
| Status | {Draft / Follow-up 필요 / 완료 / 확인 필요} |
| Owner | {짧은 owner 또는 미정} |
| Participants | {짧은 요약. 예: 플랫폼담당 외 4명} |
| Source | {짧은 source. 예: 붙여넣은 화자분리 전사본} |
| Slack thread | {링크 또는 미정} |
| Tracking | {짧은 tracker 요약. 예: Slack List / GitLab / 런북} |
| Access | {#sample-platform-team / channel override / restricted / 미정} |
| Last updated | YYYY-MM-DD |

### 메타데이터 메모
- 회의일이 source에 없으면 `Date`는 `Last updated`와 같은 값을 사용한다.
- 긴 참석자 전체 명단, source URL, Slack thread, GitLab/문서 링크, 접근권한 상세는 이 섹션에 bullet로 둔다.

## TL;DR
- {회의 결론 또는 방향성.}
- {우선순위 또는 실행 흐름.}
- {주요 follow-up 또는 리스크.}

## 결정사항
- **{짧은 결정 제목}**
  - 내용: {확정된 결정}
  - 근거: {timestamp/source}
  - 영향: {영향 범위}
  - 결정자/동의자: {이름 또는 미정}
  - 상태: 확정

## 액션 보드
- [ ] {담당}: {산출물 중심 작업}. 기한: {날짜/미정}. Tracking: {Slack/GitLab/문서 링크 또는 미정}.

## 주제별 논의
### {주제}
- 배경: {왜 논의했는가}
- 논점: {주요 내용}
- 정리: {현재 상태}

## 후속 확인
- {확인할 것}. 담당: {이름/미정}. 확인 위치: {Slack/GitLab/문서/미정}. 기한: {날짜/미정}.

## 리스크/열린 질문
- **{리스크 또는 질문 제목}**
  - 내용: {불명확한 점}
  - 확인 담당: {이름/미정}
  - 확인 방법: {확인 경로}
  - 상태: 확인 필요

---

## 보정 및 원문 부록

### 용어 보정
- `{오인식 표현}` -> `{정확한 표현}`. 근거: {문맥/링크/사용자 보정}. 신뢰도: {높음/중간/낮음}. 확인 방법: {확인 경로}.

### 불확실 단어/문장 보정
- `{불확실한 표현}` -> `{추정 또는 확인 필요}`. 근거: {문맥}. 신뢰도: 낮음. 확인 방법: {누구에게/어디서 확인}.

### 참석자/화자 보정
- `{참석자 1}` -> `{이름 또는 확인 필요}`. 근거: {사용자 제공/문맥}. 신뢰도: {높음/중간/낮음}. 확인 방법: {확인 경로}.

### 원문 전사본 전문
```text
[00:00] Speaker 1: {원문 발화}
[00:15] Speaker 2: {원문 발화}
```
````

The full transcript must remain a verbatim block with visible speaker/timestamp context. 표로 넣지 않는다. Do not summarize, normalize, translate, or replace the pasted transcript under `원문 전사본 전문`.
