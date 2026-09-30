package installcli

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"issueops/internal/adapter/installutil"
	upstream "issueops/internal/contract/upstream"
	"issueops/internal/port"
)

func TestPreparedInstallTransactionsKeepTheirPathEffects(t *testing.T) {
	var applied []string
	makeTransaction := func(owner string) (*installPathTransaction, *port.NativeInstallResult, string) {
		home, root := t.TempDir(), t.TempDir()
		req := port.NativeInstallRequest{Home: home, BinPath: filepath.Join(root, "bin", "issueops")}
		cli := testInstallCommand(t, Deps{})
		cli.EnsureSymlinkPlan = func(target, path string, dry bool) (port.InstallLink, error) {
			if !dry {
				applied = append(applied, owner)
			}
			return installutil.EnsureSymlinkPlan(target, path, dry)
		}
		result := &port.NativeInstallResult{}
		tx, err := cli.prepareInstallPathPlanForCandidate(result, req, req.BinPath, "skip")
		if err != nil {
			t.Fatal(err)
		}
		cli.EnsureSymlinkPlan = func(string, string, bool) (port.InstallLink, error) {
			t.Fatal("prepared transaction read changed command")
			return port.InstallLink{}, nil
		}
		return tx, result, home
	}
	a, ar, ah := makeTransaction("first")
	b, br, bh := makeTransaction("second")
	for _, entry := range []struct {
		tx     *installPathTransaction
		result *port.NativeInstallResult
		home   string
	}{{a, ar, ah}, {b, br, bh}} {
		if err := entry.tx.Apply(entry.result); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"issueops", "io"} {
			if info, err := os.Lstat(filepath.Join(entry.home, ".local", "bin", name)); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("missing %s link: %v", name, err)
			}
		}
	}
	if !reflect.DeepEqual(applied, []string{"first", "first", "second", "second"}) {
		t.Fatal(applied)
	}
	for _, entry := range []struct {
		tx     *installPathTransaction
		result *port.NativeInstallResult
		home   string
	}{{b, br, bh}, {a, ar, ah}} {
		if err := entry.tx.Rollback(entry.result); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(entry.home, ".local")); !os.IsNotExist(err) {
			t.Fatalf("rollback left directory: %v", err)
		}
	}
}

func TestInstallEffectsKeepInstallerAndUpstreamOwnership(t *testing.T) {
	var calls []string
	makeEffects := func(owner string) installTransactionEffects {
		cli := Command{Deps: Deps{
			InstallNative: func(req port.NativeInstallRequest) (port.NativeInstallResult, error) {
				calls = append(calls, owner+":"+req.Root)
				return port.NativeInstallResult{OK: true, Root: req.Root}, nil
			},
			SyncUpstream: func(ctx context.Context, root string, dry bool) (upstream.Report, error) {
				if _, ok := ctx.Deadline(); !ok {
					t.Fatal("upstream lost timeout")
				}
				calls = append(calls, owner+":upstream:"+root)
				return upstream.Report{}, nil
			},
		}}
		effects := installTransactionEffects{command: cli}
		cli.InstallNative = nil
		cli.SyncUpstream = nil
		return effects
	}
	a, b := makeEffects("first"), makeEffects("second")
	for _, entry := range []struct {
		owner   string
		effects installTransactionEffects
	}{{"first", a}, {"second", b}, {"first", a}} {
		result, err := entry.effects.Install(port.NativeInstallRequest{Root: entry.owner})
		if err != nil || !result.OK {
			t.Fatal(err)
		}
		entry.effects.AppendUpstream(&result, entry.owner, true)
	}
	want := []string{"first:first", "first:upstream:first", "second:second", "second:upstream:second", "first:first", "first:upstream:first"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("calls=%v", calls)
	}
}
