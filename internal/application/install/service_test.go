package install

import (
	"errors"
	"reflect"
	"testing"

	"issueops/internal/port"
)

type environmentStub struct {
	events        []string
	skills        []string
	validationErr error
}

func (env *environmentStub) AbsClean(path string) string { return path }
func (env *environmentStub) ResolveStableRoot(root string) (string, error) {
	env.events = append(env.events, "root")
	return root, nil
}
func (env *environmentStub) ValidateRuntime(string, string) error {
	env.events = append(env.events, "validate")
	return env.validationErr
}
func (env *environmentStub) ListSkills(string) ([]string, error) {
	env.events = append(env.events, "skills")
	return env.skills, nil
}

type installerStub struct {
	env  *environmentStub
	name string
	err  error
}

func (stub installerStub) Name() string { return stub.name }
func (stub installerStub) Install(request port.NativeInstallRequest) (port.HostInstallResult, error) {
	stub.env.events = append(stub.env.events, stub.name)
	return port.HostInstallResult{OK: true, Messages: []string{request.Root}}, stub.err
}

func TestServiceValidatesAndListsSkillsBeforeHostWrites(t *testing.T) {
	env := &environmentStub{skills: []string{"alpha", "beta"}}
	service := Service{Environment: env, Installers: []port.HostInstaller{installerStub{env: env, name: "codex"}, installerStub{env: env, name: "claude"}}}
	result, err := service.Install(port.NativeInstallRequest{Root: "/root", BinPath: "/root/bin/issueops", DryRun: true})
	if err != nil || !result.OK || !reflect.DeepEqual(result.SkillNames, []string{"alpha", "beta"}) || !reflect.DeepEqual(env.events, []string{"root", "validate", "skills", "codex", "claude"}) {
		t.Fatalf("result=%+v err=%v events=%v", result, err, env.events)
	}
}

func TestServiceValidationFailureHasNoHostEffects(t *testing.T) {
	env := &environmentStub{validationErr: errors.New("bad runtime")}
	service := Service{Environment: env, Installers: []port.HostInstaller{installerStub{env: env, name: "codex"}}}
	result, err := service.Install(port.NativeInstallRequest{Root: "/root", BinPath: "/outside"})
	if err == nil || result.OK || !reflect.DeepEqual(env.events, []string{"root", "validate"}) {
		t.Fatalf("result=%+v err=%v events=%v", result, err, env.events)
	}
}

func TestServiceAggregatesHostFailureWithoutSkippingLaterHosts(t *testing.T) {
	env := &environmentStub{skills: []string{}}
	service := Service{Environment: env, Installers: []port.HostInstaller{
		installerStub{env: env, name: "codex", err: errors.New("failed")}, installerStub{env: env, name: "claude"},
	}}
	result, err := service.Install(port.NativeInstallRequest{Root: "/root", BinPath: "/root/bin/issueops"})
	if err == nil || result.OK || len(result.Hosts) != 2 || !reflect.DeepEqual(env.events, []string{"root", "validate", "skills", "codex", "claude"}) {
		t.Fatalf("result=%+v err=%v events=%v", result, err, env.events)
	}
}
