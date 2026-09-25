package install

import (
	"errors"
	"fmt"

	installdomain "issueops/internal/domain/install"
	"issueops/internal/port"
)

type Environment interface {
	AbsClean(string) string
	ResolveStableRoot(string) (string, error)
	ValidateRuntime(root, binary string) error
	ListSkills(string) ([]string, error)
}

type Service struct {
	Environment Environment
	Installers  []port.HostInstaller
}

func (service Service) Install(request port.NativeInstallRequest) (port.NativeInstallResult, error) {
	if request.Root == "" {
		return port.NativeInstallResult{OK: false}, fmt.Errorf("root is required")
	}
	request.Root = service.Environment.AbsClean(request.Root)
	request.Home = service.Environment.AbsClean(request.Home)
	request.CodexHome = service.Environment.AbsClean(request.CodexHome)
	request.BinPath = service.Environment.AbsClean(request.BinPath)
	stableRoot, err := service.Environment.ResolveStableRoot(request.Root)
	if err != nil {
		return port.NativeInstallResult{OK: false, Root: request.Root, BinPath: request.BinPath}, err
	}
	request.Root = stableRoot
	if err := service.Environment.ValidateRuntime(request.Root, request.BinPath); err != nil {
		return port.NativeInstallResult{OK: false, Root: request.Root, BinPath: request.BinPath}, err
	}
	if len(request.SkillNames) == 0 {
		skills, err := service.Environment.ListSkills(request.Root)
		if err != nil {
			return port.NativeInstallResult{OK: false, Root: request.Root}, err
		}
		request.SkillNames = skills
	} else {
		request.SkillNames = installdomain.NormalizeSkillNames(request.SkillNames)
	}
	result := port.NativeInstallResult{
		OK: true, Root: request.Root, Home: request.Home, CodexHome: request.CodexHome,
		BinPath: request.BinPath, SkillNames: append([]string{}, request.SkillNames...),
		Hosts: []port.HostInstallResult{}, Files: []port.InstallFile{}, Links: []port.InstallLink{},
		ProjectLocal: request.ProjectLocal, DryRun: request.DryRun,
	}
	if len(service.Installers) == 0 {
		result.OK = false
		return result, fmt.Errorf("at least one host installer is required")
	}
	var errs []error
	for _, installer := range service.Installers {
		if installer == nil {
			continue
		}
		hostResult, err := installer.Install(request)
		if hostResult.Host == "" {
			hostResult.Host = installer.Name()
		}
		if err != nil {
			hostResult.OK = false
			hostResult.Error = err.Error()
			result.OK = false
			errs = append(errs, fmt.Errorf("%s: %w", installer.Name(), err))
		} else if !hostResult.OK {
			result.OK = false
			errs = append(errs, fmt.Errorf("%s: installer reported ok=false", installer.Name()))
		}
		result.Hosts = append(result.Hosts, hostResult)
		result.Files = append(result.Files, hostResult.Files...)
		result.Links = append(result.Links, hostResult.Links...)
		result.Messages = append(result.Messages, hostResult.Messages...)
	}
	return result, errors.Join(errs...)
}
