package hookenv

import (
	"testing"

	"issueops/internal/testsupport"
)

// TestEnforcementSwitchReadsFalseAfterIsolation은 #395의 실패 조건을 직접
// 재현한다. 상속된 kill-switch가 켜져 있어도 격리 후에는 enforcement 판독이
// 꺼짐으로 돌아와야 한다 — 이것이 dogfood 셸과 CI가 같은 결론을 내는 근거다.
func TestEnforcementSwitchReadsFalseAfterIsolation(t *testing.T) {
	t.Setenv("ISSUEOPS_DISABLE_HOOKS", "1")
	if !Bool("ISSUEOPS_DISABLE_HOOKS") {
		t.Fatal("전제 확인 실패: 켜 둔 스위치는 true로 읽혀야 한다")
	}

	testsupport.ClearInheritedOperatorSwitches()

	if Bool("ISSUEOPS_DISABLE_HOOKS") {
		t.Fatal("격리 후에는 스위치가 꺼진 것으로 읽혀야 한다")
	}
}
