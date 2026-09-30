package issueopsmodeswitch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	contract "issueops/internal/contract/issueopsmodeswitch"
	"strings"
)

// Remote history is authoritative when observable; otherwise use the prepared base.
func ComparisonRefs(branch, base string) []string {
	return []string{"refs/remotes/origin/" + branch, strings.TrimSpace(base)}
}
func ValidateApply(confirm bool, expected, actual string) error {
	if !confirm {
		return fmt.Errorf("execution switch-mode --apply requires --confirm")
	}
	if expected != actual {
		return fmt.Errorf("stale switch-mode fingerprint; run the preview again and retry with the new value")
	}
	return nil
}
func InventoryFingerprint(inventory contract.Inventory) (string, error) { return hashJSON(inventory) }
func hashJSON(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
