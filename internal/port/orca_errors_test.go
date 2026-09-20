package port

import (
	"encoding/json"
	"testing"
)

func TestOrcaErrorMessage(t *testing.T) {
	err := &OrcaError{Code: "E_ORCA_CLI", Detail: "orca probe failed"}
	if err.Error() == "" {
		t.Fatal("orca error message must be non-empty")
	}
}

func TestValidateOrcaPromptReceiptRequiresBaselineWorkingSequencePresence(t *testing.T) {
	const requestID = "22222222-2222-4222-8222-222222222222"
	for _, test := range []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "missing", raw: `{"request_id":"22222222-2222-4222-8222-222222222222","stages":["input_accepted"],"provider":"omo","process_incarnation":"process-1","generation":1}`, wantErr: true},
		{name: "explicit zero", raw: `{"request_id":"22222222-2222-4222-8222-222222222222","stages":["input_accepted"],"provider":"omo","process_incarnation":"process-1","generation":1,"baseline_working_sequence":0}`},
		{name: "positive", raw: `{"request_id":"22222222-2222-4222-8222-222222222222","stages":["input_accepted"],"provider":"omo","process_incarnation":"process-1","generation":1,"baseline_working_sequence":9}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var receipt OrcaPromptReceipt
			if err := json.Unmarshal([]byte(test.raw), &receipt); err != nil {
				t.Fatal(err)
			}
			err := ValidateOrcaPromptReceipt(receipt, requestID, "process-1")
			if (err != nil) != test.wantErr {
				t.Fatalf("validation err=%v wantErr=%v receipt=%+v", err, test.wantErr, receipt)
			}
		})
	}
}
