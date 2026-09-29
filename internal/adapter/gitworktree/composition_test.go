package gitworktree

import "issueops/internal/adapter/preflight"

func testProvisioner() Provisioner {
	return Provisioner{GitCmd: preflight.GitCmd, GitOut: preflight.GitOut}
}
