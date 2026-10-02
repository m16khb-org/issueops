# 조사 검증 원장

## 판정 기준

| 항목 | 현재 상태 | 완료에 필요한 증거 |
|---|---|---|
| 제품별 최신 릴리스 | 확인 완료 | 공식 원문의 버전·날짜·prerelease 여부, preview/과거 사례 분리 |
| 기능 존재 | 확인 완료 | 문서·설치 구현의 anchor, version 경계 기록 |
| issueops의 현재 공백 | 확인 완료 | 코드·인접 테스트, JSONL 실제 대조 재현, 기존 기능과 구분 |
| 개선 효과 | 가설로 분리 | 제안별 workload·품질·비용·지연 검증 기준, 측정 전 개선율 주장 없음 |
| 모델 배정 | 네 모델 실행 확인 | Luna·Sol·Astra·Fable 5.1 실제 작업 영수증 |
| 최종 보고서 | 검토 지적 반영 완료 | 출처·코드·우선순위·반증·Fable 반영표 |

## 근거의 독립성

공식 문서와 같은 공급자의 GitHub 릴리스는 두 독립 출처로 세지 않는다.
공식 기능 소개는 단일 1차 출처 예외로 다룬다.
보고서에서 성능 개선 수치를 주장하려면 재현 가능한 측정이 필요하다.

## 검증 범위

현재 변경은 조사 Markdown뿐이다. 프로덕션 진단·테스트·빌드를 기능 변경의
검증으로 대신 제시하지 않는다. 최종 문서 검증에서는 파일·링크·근거를 확인하고,
프로젝트 문서 검증 규칙에 따른 self-verify 필요성과 결과를 별도로 기록한다.
Go 코드 변경이 없으므로 별도 race/vet를 자동으로 추가하지 않는다.

## 초기 반증 질문

- 호스트에 이미 있는 기능을 issueops core에 중복 구현하려는 제안인가?
- 기존 trace, review-metrics, catalog hash, progress 출력을 누락으로 오인했는가?
- 설치본, 최신 공식 릴리스, 개발 브랜치를 혼동했는가?
- 캐시가 actor/generation/workspace fence나 policy 재평가를 무효화하는가?
- hook에 상태 관측과 정책 집행을 다시 넣는가?
- 요청된 모델과 실제 fallback 실행 모델이 다른가?
- 새 기능의 존재를 issueops에서 측정된 성능 개선으로 잘못 해석했는가?

## 실제 모델 영수증

- `st_01a0fa37` (`claude-01`):
  `chatgpt-subscription/gpt-6-luna`.
- `st_01a0fa4c` (`omo-02`):
  `chatgpt-subscription/gpt-6.1-sol`.
- `st_01a0fa76` (`streamable-boundary`), completed:
  `chatgpt-subscription/gpt-6-astra`.
- `st_01a0fb21` (final research audit), completed:
  `anthropic-subscription/claude-fable-5-1`.
  판정은 ready with corrections. 중간 2건·낮음 4건을 메인이 확인해 반영했다.

## 설치 버전과 공개 버전

메인이 로컬 `omo-ai/package.json`을 읽어 설치본 `5.1.8`과
`@code-yeongyu/senpi` 의존 버전 `2026.10.1-2`를 확인했다.
공식 GitHub `v5.1.9` API는 `2026-10-02T01:13:10Z`,
`prerelease:false`와 engine `2026.10.1-3`을 반환했다.
5.1.9의 child timeout/retry 상속 수정은 공개 릴리스의 변경이지 현재 설치본에서
재현·확인한 수정이 아니다. 설치나 업데이트는 실행하지 않았다.

후속 집계에서 배포 package.json이 `5.1.9 / 2026.10.1-3`으로 바뀐 것을
메인도 직접 읽어 확인했다. 이 조사에서 설치 명령을 실행한 것은 아니며 변경
원인을 조사하지 않았다. 현재 세션에 고정된 runtime 디렉터리의 package.json은
여전히 `2026.10.1-2`다. 배포 패키지 갱신을 현재 세션의 엔진 교체로 해석하지 않는다.

## 발견된 기존 검증 환경 실패

`shared-01.md`는 담당자가 수행한 self-verify의 실패를 기록한다.
Python script tests에서 `ModuleNotFoundError: No module named 'pydantic'`가
났고, collect-all 재시도는 go test 단계에 진입한 뒤 시간 제한으로 끝났다.
이 부분 결과를 성공한 battery로 사용하지 않는다. 최종 검증 판단은 메인이
실제 명령 출력으로 별도 확인한다.

메인의 직접 실행도 같은 오류로 exit 1이었다. [명령과 traceback](self-verify-result.md)에
기록했으며, Go test/build까지 성공한 것으로 보고하지 않는다.
연구 문서·출처·실행 재현 검증과 전체 repository battery의 실패를 구별한다.
