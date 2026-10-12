package statepath

import (
	"strings"
	"testing"
)

func TestNormalizeKeyLengthAndCharacterBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr bool
	}{
		{"128 chars", strings.Repeat("a", 128), false},
		{"129 chars", strings.Repeat("a", 129), true},
		{"first a", "a", false},
		{"first z", "z", false},
		{"first A", "A", false},
		{"first Z", "Z", false},
		{"first 0", "0", false},
		{"first 9", "9", false},
		{"inner boundaries", "x" + "azAZ09._-", false},
		{"before a", "a`", true},
		{"after z", "a{", true},
		{"before A", "a@", true},
		{"after Z", "a[", true},
		{"after 9", "a:", true},
		{"first before a", "`a", true},
		{"first after z", "{a", true},
		{"first before A", "@a", true},
		{"first after Z", "[a", true},
		{"first hyphen rejected", "-a", true},
		{"first after 9", ":a", true},
		{"first dot rejected", ".a", true},
		{"first underscore rejected", "_a", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NormalizeKey(test.key)
			if (err != nil) != test.wantErr {
				t.Fatalf("NormalizeKey(%q) error=%v, wantErr=%v", test.key, err, test.wantErr)
			}
		})
	}
}
