package issueops

import "strings"

// branchRefPresent는 exact local branch ref가 여전히 존재하는지 재관측한다.
//
// `update-ref -d` 실패를 부재로 정규화할지 판정하는 유일한 근거다. Git의 오류
// 문구는 버전과 로케일에 따라 달라지므로 문자열 매칭 대신 ref 자체를 다시
// 읽는다. 관측이 불가능하면(예: Git 호출 자체가 실패) 존재하는 것으로 보아
// fail-closed한다 — 부재를 증명하지 못한 상태에서 성공으로 정규화하면 실제
// 실패를 삼키게 된다.
func branchRefPresent(git func(dir string, args ...string) (int, string), repo, branch string) bool {
	if git == nil || strings.TrimSpace(branch) == "" {
		return true
	}
	code, _ := git(repo, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return code != 1
}
