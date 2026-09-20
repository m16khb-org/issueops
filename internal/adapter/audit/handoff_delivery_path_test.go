package audit

import (
	"os"
	"testing"
)

func TestHandoffDeliveryWindowsModeAcceptsNativePermissionsAndRejectsReparseTypes(t *testing.T) {
	tests := []struct {
		name      string
		mode      os.FileMode
		directory bool
		want      bool
	}{
		{name: "native directory", mode: os.ModeDir | 0o777, directory: true, want: true},
		{name: "native regular file", mode: 0o666, want: true},
		{name: "symlink directory", mode: os.ModeSymlink | os.ModeDir | 0o777, directory: true},
		{name: "irregular file", mode: os.ModeIrregular | 0o666},
		{name: "wrong object type", mode: os.ModeDir | 0o777},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validHandoffDeliveryWindowsMode(test.mode, test.directory); got != test.want {
				t.Fatalf("valid mode=%v directory=%v got=%v want=%v", test.mode, test.directory, got, test.want)
			}
		})
	}
}
