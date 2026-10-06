package testsupport

import (
	"os"
	"slices"
	"testing"
)

// TestClearInheritedOperatorSwitchesRemovesEverySwitch는 격리 계약 자체를
// 고정한다. 이 테스트가 없으면 목록에 새 스위치를 추가하고도 격리를 잊는다.
func TestClearInheritedOperatorSwitchesRemovesEverySwitch(t *testing.T) {
	for _, name := range OperatorSwitches {
		t.Setenv(name, "1")
	}

	cleared := ClearInheritedOperatorSwitches()

	for _, name := range OperatorSwitches {
		if value, present := os.LookupEnv(name); present {
			t.Fatalf("%s가 지워지지 않았다: %q", name, value)
		}
		if !slices.Contains(cleared, name) {
			t.Fatalf("%s가 제거 보고에 없다: %v", name, cleared)
		}
	}
}

// TestClearInheritedOperatorSwitchesIsIdempotent는 변수가 애초에 없을 때
// (CI 환경) 아무것도 보고하지 않음을 고정한다.
func TestClearInheritedOperatorSwitchesIsIdempotent(t *testing.T) {
	for _, name := range OperatorSwitches {
		t.Setenv(name, "1")
	}
	ClearInheritedOperatorSwitches()

	if cleared := ClearInheritedOperatorSwitches(); len(cleared) != 0 {
		t.Fatalf("이미 비어 있는 환경에서 제거를 보고하면 안 된다: %v", cleared)
	}
}
