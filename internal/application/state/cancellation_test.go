package state

import (
	"context"
	"errors"
	"testing"
	"time"

	statecontract "issueops/internal/contract/state"
	stateport "issueops/internal/port/state"
)

func TestCanceledRequestsDoNotOpenStorage(t *testing.T) {
	for _, action := range []string{"write", "record", "delete", "lock", "prune", "dry-prune"} {
		t.Run(action, func(t *testing.T) {
			opens := 0
			store := &memoryStore{records: map[string][]byte{}}
			service := NewService(Dependencies{
				StateDir:        func() string { return "/state" },
				StatePath:       func(dir, key string) string { return dir + "/" + key },
				OpenStore:       func(string) (stateport.Store, error) { opens++; return store, nil },
				ExistingRecords: memoryExistingReader{store: store},
			})
			ctx, cancel := context.WithCancel(t.Context())
			cancel()
			var err error
			switch action {
			case "write":
				_, err = service.Write(ctx, "key", "value")
			case "record":
				_, err = service.WriteRecord(ctx, "/state", "key", statecontract.RecordEnvelope{})
			case "delete":
				err = service.Delete(ctx, "key")
			case "lock":
				err = service.WithKeyLock(ctx, "/state", "key", func(context.Context) error {
					t.Error("called callback")
					return nil
				})
			case "prune", "dry-prune":
				_, err = service.Prune(ctx, time.Hour, action == "prune")
			}
			if !errors.Is(err, context.Canceled) || opens != 0 {
				t.Fatalf("err=%v opens=%d, want cancellation without storage", err, opens)
			}
		})
	}
}
