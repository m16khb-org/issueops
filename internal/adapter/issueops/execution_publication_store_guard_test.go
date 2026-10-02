package issueops

import (
	"context"
	"testing"

	"issueops/internal/adapter/outbound/sqlstore"
	authorityport "issueops/internal/port/authority"
)

func TestRemotePublicationTransactionKeepsRequestGuardButIgnoresCancellation(t *testing.T) {
	type marker struct{}
	guarded := 0
	stateRoot := t.TempDir()
	ctx, cancel := context.WithCancel(context.WithValue(context.Background(), marker{}, "request"))
	ctx = sqlstore.WithRecordGuard(ctx, stateRoot, func(spanCtx context.Context, _ authorityport.RecordReader) (context.Context, error) {
		guarded++
		return spanCtx, nil
	})
	cancel()
	var transitionValue any
	var transitionErr error
	err := (RemotePublicationStore{StateRoot: stateRoot}).WithinTransaction(ctx, "io-guard0001", func(spanCtx context.Context) error {
		transitionValue, transitionErr = spanCtx.Value(marker{}), spanCtx.Err()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if guarded != 1 || transitionValue != "request" || transitionErr != nil {
		t.Fatalf("guarded=%d value=%v ctxErr=%v", guarded, transitionValue, transitionErr)
	}
}
