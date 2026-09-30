package statepath

import (
	"testing"
)

func TestNormalizeKey(t *testing.T) {
	tests := []struct {
		key     string
		wantErr bool
	}{
		{"valid-key", false},
		{"valid.key_123", false},
		{"a", false},
		{"", true},
		{"../escape", true},
		{"path/separator", true},
		{"path\\separator", true},
		{"key with spaces", true},
	}
	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			_, err := NormalizeKey(test.key)
			if (err != nil) != test.wantErr {
				t.Errorf("NormalizeKey(%q) error=%v, wantErr=%v", test.key, err, test.wantErr)
			}
		})
	}
}
