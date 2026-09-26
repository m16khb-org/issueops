package apidoc

import "testing"

func TestControllerRequiresDocumentedRoute(t *testing.T) {
	violations := CheckNestController("users.controller.ts", "@Get('users')\n  async list() { return []; }")
	if len(violations) == 0 {
		t.Fatal("undocumented route must produce a violation")
	}
}
