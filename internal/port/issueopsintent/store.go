package issueopsintent

import model "issueops/internal/contract/issueops"

type Store struct {
	Read       func(string, string) (model.Record, error)
	TouchWrite func(string, model.Record) (model.Record, error)
	Now        func() string
}
