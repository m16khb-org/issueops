package hookcli

import "testing"

func TestRunHookExport(t *testing.T) {
	if err := runHookWithTestConfig([]string{"--help"}); err == nil {
		t.Fatalf("expected ErrHelp, got nil")
	}
	if err := runHookWithTestConfig([]string{}); err == nil {
		t.Fatalf("expected error for empty args, got nil")
	}
	if err := runHookWithTestConfig([]string{"unknown-hook"}); err == nil {
		t.Fatalf("expected error for unknown hook, got nil")
	}
}
