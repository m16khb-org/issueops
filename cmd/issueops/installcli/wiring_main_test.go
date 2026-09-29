package installcli

import (
	"fmt"

	"os"
	"testing"
)

// 프로덕션에서는 issueopsapp이 주입한다. 설치 경로 테스트는 실제 채택 트랜잭션을
// 검증하므로 같은 배선을 재현한다.
func TestMain(m *testing.M) {
	exitCode := runWithManagedCommandFixture(
		buildManagedTestCommandSource,
		func(fixture managedCommandFixture) int {
			managedTestCommandSource = fixture
			return m.Run()
		},
		func(err error) { fmt.Fprintln(os.Stderr, err) },
	)
	os.Exit(exitCode)
}
