package update

import (
	"errors"
	"reflect"
	"testing"
)

type fakeInstaller struct {
	calls      []string
	installErr error
}

func (fake *fakeInstaller) Install(root string, args []string) error {
	fake.calls = append(fake.calls, append([]string{root}, args...)...)
	return fake.installErr
}

func TestServiceForwardsOptionsToInstaller(t *testing.T) {
	for _, tc := range []struct {
		name       string
		options    Options
		installErr error
		want       []string
		wantErr    bool
	}{
		{name: "update", options: Options{Root: "/repo", ProjectLocal: true, PathMode: "skip", SkipBuild: true}, want: []string{"/repo", "--project-local", "--path-mode=skip", "--skip-build"}},
		{name: "dry run", options: Options{Root: "/repo", DryRun: true, JSON: true}, want: []string{"/repo", "--dry-run", "--json"}},
		{name: "stdio transport", options: Options{Root: "/repo", MCPTransport: "stdio"}, want: []string{"/repo", "--mcp-transport=stdio"}},
		{name: "install failure", options: Options{Root: "/repo"}, installErr: errors.New("write failed"), want: []string{"/repo"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeInstaller{installErr: tc.installErr}
			err := (Service{Installer: fake}).Run(tc.options)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(fake.calls, tc.want) {
				t.Fatalf("calls = %v, want %v", fake.calls, tc.want)
			}
		})
	}
}
