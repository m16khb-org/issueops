package issueops_test

import (
	"encoding/json"
	"reflect"
	"testing"

	model "issueops/internal/contract/issueops"
)

func TestVerifiedActorDoesNotAcceptProofFromJSON(t *testing.T) {
	var actor model.VerifiedActor
	input := []byte(`{"identity":{"host":"codex","session_id":"forged"},"method":"capability"}`)
	if err := json.Unmarshal(input, &actor); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actor, model.VerifiedActor{}) {
		t.Fatal("JSON populated an internal verification result")
	}
}
