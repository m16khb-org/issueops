//go:build unix

package doctor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"issueops/internal/testsupport"
)

func TestProbeGatewayHTTPStopsOnCanceledContext(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer server.Close()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ProbeGatewayHTTP(ctx, server.URL); err == nil {
		t.Fatal("a canceled context must fail the probe")
	}
	if hits.Load() != 0 {
		t.Fatal("the probe reached the gateway although its context was canceled")
	}
}

func TestRunLsofStopsOnCanceledContext(t *testing.T) {
	bin := t.TempDir()
	testsupport.WriteFakeCommand(t, bin, "lsof", "#!/bin/sh\n: > \"$(dirname \"$0\")/ran\"\n")
	t.Setenv("PATH", bin)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := runLsof(ctx, "-nP"); err == nil {
		t.Fatal("a canceled context must fail the lsof run")
	}
	if _, err := os.Stat(filepath.Join(bin, "ran")); err == nil {
		t.Fatal("lsof ran although its context was canceled")
	}
}
