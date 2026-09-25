package installcli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	installapp "issueops/internal/application/install"
	activationapp "issueops/internal/application/nativeactivation"
	activationcontract "issueops/internal/contract/nativeactivation"
	installdomain "issueops/internal/domain/install"
	"issueops/internal/port"
)

func runInstall(args []string) error {
	fs := flag.NewFlagSet("install", flag.ContinueOnError)
	projectLocal := fs.Bool("project-local", false, "also write project-local .mcp.json/.claude settings and project skill links; default is user/global only")
	dryRun := fs.Bool("dry-run", false, "plan files and links without writing them")
	pathMode := fs.String("path-mode", "auto", "manage ~/.local/bin PATH setup: auto, manual, or skip")
	interactive := fs.Bool("interactive", false, "ask for install choices before applying the plan")
	adoptCommandFile := fs.Bool("adopt-command-file", false, "replace an existing managed issueops command file with rollback protection")
	jsonOut := fs.Bool("json", false, "print JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	home, err := installUserHomeDir()
	if err != nil {
		return err
	}
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		codexHome = filepath.Join(home, ".codex")
	}
	if *interactive {
		if err := validateInteractiveInstallInput(os.Stdin); err != nil {
			return err
		}
		choices, err := promptInstallChoices(*projectLocal, *dryRun, *pathMode, os.Stdin, os.Stderr)
		if err != nil {
			return err
		}
		*projectLocal = choices.ProjectLocal
		*dryRun = choices.DryRun
		*pathMode = choices.PathMode
	}
	if !validInstallPathMode(*pathMode) {
		return fmt.Errorf("invalid --path-mode %q: expected auto, manual, or skip", *pathMode)
	}
	root := deps.IssueOpsRoot()
	if deps.NativeInstallRequest == nil || deps.InstallNative == nil {
		return fmt.Errorf("native installer is not configured")
	}
	req := deps.NativeInstallRequest(root, home, codexHome, filepath.Join(root, "bin", "issueops"))
	req.ProjectLocal = *projectLocal
	req.DryRun = *dryRun
	req.AdoptCommandFile = *adoptCommandFile
	candidatePath, err := nativeInstallCandidatePath(req.BinPath, req.DryRun, deps.ExecutablePath)
	if err != nil {
		return err
	}
	stateDir := filepath.Dir(IssueOpsStateRoot())
	activationRequest := activationcontract.Request{StateRoot: stateDir, IssueOpsRoot: req.Root, TargetBinary: req.BinPath}
	if !req.DryRun && deps.ActivationBackend == nil {
		return fmt.Errorf("native activation backend is unavailable")
	}
	activationStep, err := nativeActivationStep(req.DryRun, os.Getenv("ISSUEOPS_NATIVE_ACTIVATION_STEP"))
	if err != nil {
		return err
	}
	if deps.ActivationReadback == nil {
		return fmt.Errorf("native activation readback is not configured")
	}
	readback := deps.ActivationReadback(req)
	activationService := activationapp.NewService(deps.ActivationBackend, readback)
	return executeInstall(req, candidatePath, *pathMode, activationStep, *jsonOut, activationService, activationRequest)
}

func executeInstall(req port.NativeInstallRequest, candidatePath, pathMode, activationStep string, jsonOut bool, activationService *activationapp.Service, activationRequest activationcontract.Request) error {
	outcome, err := installapp.RunTransaction(context.Background(), installapp.TransactionRequest{
		Install: req, CandidatePath: candidatePath, PathMode: pathMode, Step: activationStep,
		TransitionID: os.Getenv("ISSUEOPS_NATIVE_ACTIVATION_TRANSITION_ID"), Activation: activationRequest,
	}, installTransactionEffects{}, activationService)
	if outcome.Install != nil {
		return outputInstallResult(*outcome.Install, err, jsonOut)
	}
	if err != nil {
		return err
	}
	if outcome.Activation == nil {
		return fmt.Errorf("native install produced no result")
	}
	if jsonOut {
		return printJSON(outcome.Activation)
	}
	if activationStep == "abort" {
		fmt.Printf("native activation transition %s aborted\n", outcome.Activation.TransitionID)
	} else {
		fmt.Printf("native activation candidate %s is pending as transition %s\n", outcome.Activation.BinarySHA256, outcome.Activation.TransitionID)
	}
	return nil
}

func nativeInstallCandidatePath(target string, dryRun bool, executable func() (string, error)) (string, error) {
	if executable == nil {
		return "", fmt.Errorf("native install executable inspector is unavailable")
	}
	candidate, err := executable()
	if err != nil {
		return "", fmt.Errorf("inspect native install candidate: %w", err)
	}
	candidate, err = filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("inspect native install candidate: %w", err)
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return "", fmt.Errorf("inspect native install target: %w", err)
	}
	candidate = filepath.Clean(candidate)
	target = filepath.Clean(target)
	if !installdomain.CandidatePathAllowed(candidate, target, dryRun) {
		return "", fmt.Errorf("native install candidate must be the canonical target or a same-directory staged binary")
	}
	return candidate, nil
}

func outputInstallResult(result port.NativeInstallResult, err error, jsonOut bool) error {
	if jsonOut {
		_ = printJSON(result)
		return err
	}
	printInstallNativeResult(result)
	return err
}

func nativeActivationStep(dryRun bool, raw string) (string, error) {
	return installdomain.ActivationStep(dryRun, raw)
}

func installUserHomeDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determine user home directory: %w", err)
	}
	if home == "" {
		return "", errors.New("determine user home directory: empty path")
	}
	return home, nil
}

func validInstallPathMode(mode string) bool {
	return installdomain.ValidPathMode(mode)
}
