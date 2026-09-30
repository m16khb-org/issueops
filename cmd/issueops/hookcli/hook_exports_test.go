package hookcli

import "testing"

func TestRunHookExport(t *testing.T) {
	if err := runHook([]string{"--help"}); err == nil {
		t.Fatalf("expected ErrHelp, got nil")
	}
	if err := runHook([]string{}); err == nil {
		t.Fatalf("expected error for empty args, got nil")
	}
	if err := runHook([]string{"unknown-hook"}); err == nil {
		t.Fatalf("expected error for unknown hook, got nil")
	}
}
