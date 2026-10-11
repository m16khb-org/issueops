package install

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"issueops/internal/port"
)

type failingEnv struct {
	environmentStub
	rootErr   error
	skillsErr error
}

func (env *failingEnv) ResolveStableRoot(root string) (string, error) {
	env.events = append(env.events, "root")
	return "/stable" + root, env.rootErr
}

func (env *failingEnv) ListSkills(string) ([]string, error) {
	env.events = append(env.events, "skills")
	return env.skills, env.skillsErr
}

type notOKInstaller struct{ called *bool }

func (notOKInstaller) Name() string { return "omp" }
func (installer notOKInstaller) Install(port.NativeInstallRequest) (port.HostInstallResult, error) {
	*installer.called = true
	return port.HostInstallResult{OK: false}, nil
}

func TestServiceRequiresRootBeforeTouchingEnvironment(t *testing.T) {
	env := &failingEnv{}
	result, err := Service{Environment: env}.Install(port.NativeInstallRequest{})
	if err == nil || !strings.Contains(err.Error(), "root is required") || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(env.events) != 0 {
		t.Fatalf("environment touched: %v", env.events)
	}
}

func TestServiceStableRootFailureSkipsValidationAndHosts(t *testing.T) {
	cause := errors.New("worktree gone")
	env := &failingEnv{rootErr: cause}
	service := Service{Environment: env, Installers: []port.HostInstaller{installerStub{env: &env.environmentStub, name: "codex"}}}
	result, err := service.Install(port.NativeInstallRequest{Root: "/r", BinPath: "/r/bin"})
	if !errors.Is(err, cause) || result.OK || result.BinPath != "/r/bin" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !reflect.DeepEqual(env.events, []string{"root"}) {
		t.Fatalf("events = %v", env.events)
	}
}

func TestServiceSkillListFailureSkipsHosts(t *testing.T) {
	cause := errors.New("skills dir unreadable")
	env := &failingEnv{skillsErr: cause}
	service := Service{Environment: env, Installers: []port.HostInstaller{installerStub{env: &env.environmentStub, name: "codex"}}}
	result, err := service.Install(port.NativeInstallRequest{Root: "/r", BinPath: "/r/bin"})
	if !errors.Is(err, cause) || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if result.Root != "/stable/r" {
		t.Fatalf("result must report the resolved root: %q", result.Root)
	}
	if !reflect.DeepEqual(env.events, []string{"root", "validate", "skills"}) {
		t.Fatalf("events = %v", env.events)
	}
}

func TestServiceExplicitSkillNamesAreNormalizedAndNotListed(t *testing.T) {
	env := &failingEnv{}
	service := Service{Environment: env, Installers: []port.HostInstaller{installerStub{env: &env.environmentStub, name: "codex"}}}
	result, err := service.Install(port.NativeInstallRequest{Root: "/r", BinPath: "/r/bin", SkillNames: []string{" b ", "a", "b", ""}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(result.SkillNames, []string{"a", "b"}) {
		t.Fatalf("skills = %v", result.SkillNames)
	}
	if contains(env.events, "skills") {
		t.Fatalf("explicit skills must not trigger listing: %v", env.events)
	}
	if result.Root != "/stable/r" {
		t.Fatalf("root = %q", result.Root)
	}
}

func TestServiceWithoutInstallersFailsAfterValidation(t *testing.T) {
	env := &failingEnv{environmentStub: environmentStub{skills: []string{"a"}}}
	result, err := Service{Environment: env}.Install(port.NativeInstallRequest{Root: "/r", BinPath: "/r/bin"})
	if err == nil || !strings.Contains(err.Error(), "at least one host installer") || result.OK {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !reflect.DeepEqual(env.events, []string{"root", "validate", "skills"}) {
		t.Fatalf("events = %v", env.events)
	}
}

func TestServiceTreatsNotOKHostResultAsFailureAndSkipsNilInstaller(t *testing.T) {
	env := &failingEnv{environmentStub: environmentStub{skills: []string{"a"}}}
	called := false
	service := Service{Environment: env, Installers: []port.HostInstaller{nil, notOKInstaller{called: &called}}}
	result, err := service.Install(port.NativeInstallRequest{Root: "/r", BinPath: "/r/bin"})
	if !called || err == nil || !strings.Contains(err.Error(), "omp: installer reported ok=false") || result.OK {
		t.Fatalf("called=%v result=%+v err=%v", called, result, err)
	}
	if len(result.Hosts) != 1 || result.Hosts[0].Host != "omp" || result.Hosts[0].OK {
		t.Fatalf("hosts = %+v", result.Hosts)
	}
}

func TestServiceRecordsHostErrorTextAndDefaultsHostName(t *testing.T) {
	env := &failingEnv{environmentStub: environmentStub{skills: []string{"a"}}}
	service := Service{Environment: env, Installers: []port.HostInstaller{installerStub{env: &env.environmentStub, name: "claude", err: errors.New("denied")}}}
	result, err := service.Install(port.NativeInstallRequest{Root: "/r", BinPath: "/r/bin"})
	if err == nil || !strings.Contains(err.Error(), "claude: denied") {
		t.Fatalf("err = %v", err)
	}
	if got := result.Hosts[0]; got.Host != "claude" || got.OK || got.Error != "denied" {
		t.Fatalf("host = %+v", got)
	}
}
