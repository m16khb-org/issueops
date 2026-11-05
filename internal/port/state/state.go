// Package state defines the state store ports that application services
// depend on. Adapters implement them. It is a port-layer package.
package state

import "context"

type reader interface {
	Get(bucket, id string) ([]byte, bool, error)
}

// ExistingReader reads a record from dir only if the store already exists. The
// bool is false when the store or the record is absent.
type ExistingReader interface {
	GetExisting(dir, bucket, id string) ([]byte, bool, error)
}

// TransactionalStore reads records and applies a batch of mutations
// atomically: all take effect or none do.
type TransactionalStore interface {
	reader
	Mutate(context.Context, []Mutation) error
}

// Store is a TransactionalStore that can also list the IDs in a bucket and run
// a function inside a store-wide span.
type Store interface {
	TransactionalStore
	List(bucket string) ([]string, error)
	WithSpan(context.Context, func(context.Context) error) error
}

// Mutation is one write in a Mutate batch: set Data for Bucket/ID, or remove
// the row when Delete is set. RequireAbsent fails the batch if the row already
// exists and cannot be combined with Delete.
type Mutation struct {
	Bucket        string
	ID            string
	Data          []byte
	Delete        bool
	RequireAbsent bool
}
