// Package issueopsintent defines the port through which issueops intent flows
// read and write cycle records.
package issueopsintent

import model "issueops/internal/contract/issueops"

// Store bundles the cycle record operations an intent flow needs: Read loads a
// record, TouchWrite persists a modified record and returns the stored result,
// and Now supplies the timestamp string used for intent records.
type Store struct {
	Read       func(string, string) (model.Record, error)
	TouchWrite func(string, model.Record) (model.Record, error)
	Now        func() string
}
