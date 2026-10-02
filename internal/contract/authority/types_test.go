package authority_test

import (
	"encoding/json"
	"strings"
	"testing"

	"issueops/internal/contract/authority"
)

func TestUseDoesNotSerializeCredential(t *testing.T) {
	use := authority.Use{Key: "request", Token: "private-credential-fixture"}
	data, err := json.Marshal(use)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), use.Token) {
		t.Fatal("serialized authority use contains its credential")
	}
}

func TestUseDoesNotAcceptCredentialFromJSON(t *testing.T) {
	var use authority.Use
	if err := json.Unmarshal([]byte(`{"token":"forged"}`), &use); err != nil {
		t.Fatal(err)
	}
	if use.Token != "" {
		t.Fatal("JSON populated the internal credential")
	}
}
