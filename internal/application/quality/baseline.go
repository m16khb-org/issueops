package quality

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	contract "issueops/internal/contract/quality"
	state "issueops/internal/contract/state"
	domain "issueops/internal/domain/quality"
)

// SNRBaselineStore persists the code signal-to-noise baseline through the state store.
// CanonicalRepository resolves the repository identity that keys the record; ReadState and
// WriteState access the underlying state entries.
type SNRBaselineStore struct {
	CanonicalRepository func(string) (string, error)
	ReadState           func(string) (state.Result, error)
	WriteState          func(string, string) (state.Result, error)
}

func (store SNRBaselineStore) identity(root string) (string, string, error) {
	repository, err := store.CanonicalRepository(root)
	if err != nil {
		return "", "", err
	}
	return repository, domain.SNRBaselineKey(repository), nil
}

// Read returns the stored SNR ratio baseline for the repository at root. The bool is false
// when no baseline exists yet; an error means the store is unconfigured or the record is
// unreadable, malformed, or belongs to another repository.
func (store SNRBaselineStore) Read(root string) (float64, bool, error) {
	if store.ReadState == nil {
		return 0, false, fmt.Errorf("quality SNR baseline store is not configured")
	}
	repository, key, err := store.identity(root)
	if err != nil {
		return 0, false, err
	}
	res, err := store.ReadState(key)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	if !res.OK {
		return 0, false, fmt.Errorf("quality SNR baseline read returned no record")
	}
	var baseline contract.SNRBaselineRecord
	if err := json.Unmarshal([]byte(res.Record.Content), &baseline); err != nil {
		return 0, false, fmt.Errorf("parse quality SNR baseline: %w", err)
	}
	if !domain.ValidSNRBaseline(baseline, repository) {
		return 0, false, fmt.Errorf("quality SNR baseline identity or ratio is invalid")
	}
	return baseline.Ratio, true, nil
}

// Save persists the current ratio as the new baseline for trend
// comparison on a later run.
func (store SNRBaselineStore) Save(root string, ratio float64) error {
	if store.WriteState == nil {
		return fmt.Errorf("quality SNR baseline store is not configured")
	}
	if !domain.ValidSNRRatio(ratio) {
		return fmt.Errorf("quality SNR baseline ratio must be finite and between zero and one")
	}
	repository, key, err := store.identity(root)
	if err != nil {
		return err
	}
	content, err := json.Marshal(contract.SNRBaselineRecord{
		SchemaVersion: contract.SNRBaselineSchemaVersion,
		Repository:    repository,
		Ratio:         ratio,
	})
	if err != nil {
		return err
	}
	_, err = store.WriteState(key, string(content))
	return err
}
