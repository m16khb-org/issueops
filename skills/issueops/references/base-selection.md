# Execution ownership: base 선택

base 선택은 사용자 명시 branch → issue가 지정한 integration branch → 프로젝트 관례 →
관찰한 최근 merge target → repository default 순서다. 선택한 근거와 정확한 base SHA를
보고한다. release flow를 벗어나는 base는 사용자가 명시적으로 그 base를 요청한 경우에만
선택하고 그 예외를 보고한다. 그렇지 않으면 관찰한 release 관례를 따른다.
형제 prepare 스킬이 없어도 이 순서를 적용한다.
