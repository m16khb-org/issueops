package update

import (
	"errors"
	"reflect"
	"testing"
)

type fakeInstaller struct {
	calls      []string
	installErr error
	refreshErr error
}

func (fake *fakeInstaller) Install(root string, args []string) error {
	fake.calls = append(fake.calls, append([]string{root}, args...)...)
	return fake.installErr
}

func (fake *fakeInstaller) RefreshDaemon() error {
	fake.calls = append(fake.calls, "refresh")
	return fake.refreshErr
}

func TestServiceRunsInstallerBeforeDaemonRefresh(t *testing.T) {
	for _, tc := range []struct {
		name       string
		options    Options
		installErr error
		want       []string
		wantErr    bool
	}{
		{name: "update", options: Options{Root: "/repo", ProjectLocal: true, PathMode: "skip", SkipBuild: true}, want: []string{"/repo", "--project-local", "--path-mode=skip", "--skip-build", "refresh"}},
		{name: "dry run", options: Options{Root: "/repo", DryRun: true, JSON: true}, want: []string{"/repo", "--dry-run", "--json"}},
		{name: "install failure", options: Options{Root: "/repo"}, installErr: errors.New("write failed"), want: []string{"/repo"}, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeInstaller{installErr: tc.installErr}
			err := (Service{Installer: fake, RefreshDaemon: fake.RefreshDaemon}).Run(tc.options)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(fake.calls, tc.want) {
				t.Fatalf("calls = %v, want %v", fake.calls, tc.want)
			}
		})
	}
}
