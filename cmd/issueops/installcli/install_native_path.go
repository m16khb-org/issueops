package installcli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"issueops/internal/adapter/outbound/osfile"
	installcontract "issueops/internal/contract/install"
	"issueops/internal/port"
)

type installPathTransaction struct {
	ensureSymlinkPlan func(string, string, bool) (port.InstallLink, error)
	req               port.NativeInstallRequest
	command           ManagedCommandPathTransaction
	managed           bool
	applied           bool
	commandCreated    bool
	commandExisted    bool
	commandTarget     string
	shortCreated      bool
	binDirExisted     bool
	localDirExisted   bool
	path              string
	shortPath         string
}

func (c Command) prepareInstallPathPlanFromCandidate(result *port.NativeInstallResult, req port.NativeInstallRequest, candidatePath, mode string) (*installPathTransaction, error) {
	userBin := filepath.Join(req.Home, ".local", "bin")
	commandPath := filepath.Join(userBin, osfile.ExecutableName("issueops"))
	shortCommandPath := filepath.Join(userBin, osfile.ExecutableName("io"))
	transaction := &installPathTransaction{
		req:               req,
		ensureSymlinkPlan: c.EnsureSymlinkPlan,
		path:              commandPath,
		shortPath:         shortCommandPath,
		binDirExisted:     installPathDirectoryExists(userBin),
		localDirExisted:   installPathDirectoryExists(filepath.Dir(userBin)),
	}
	info, statErr := os.Lstat(commandPath)
	if statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		transaction.commandExisted = true
		transaction.commandTarget, statErr = os.Readlink(commandPath)
		if statErr != nil {
			return nil, statErr
		}
	}
	if err := c.planInstallCommandPath(result, req, candidatePath, transaction, info, statErr); err != nil {
		return nil, err
	}
	shortLink, err := ensureShortCommandShimPlan(commandPath, shortCommandPath, true, c.EnsureSymlinkPlan)
	if req.DryRun {
		result.Links = append(result.Links, shortLink)
	}
	if err != nil {
		return nil, err
	}
	if req.DryRun {
		if err := planShellPath(result, req, mode); err != nil {
			return nil, err
		}
	}
	return transaction, nil
}

// planInstallCommandPath plans the issueops command entry: a regular file is
// adopted as a managed command, anything else becomes a symlink plan.
func (c Command) planInstallCommandPath(result *port.NativeInstallResult, req port.NativeInstallRequest, candidatePath string, transaction *installPathTransaction, info os.FileInfo, statErr error) error {
	commandPath := transaction.path
	if statErr == nil && info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
		managed, plan, err := c.PrepareManagedCommandPathCandidate(req.BinPath, candidatePath, commandPath, req.AdoptCommandFile, req.DryRun)
		result.CommandPath = managedCommandPathResult(plan)
		result.Links = append(result.Links, port.InstallLink{Path: commandPath, Target: req.BinPath, WouldCreate: plan.WouldAdopt})
		if err != nil {
			return err
		}
		transaction.command = managed
		transaction.managed = true
		return nil
	}
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}
	link, err := c.EnsureSymlinkPlan(req.BinPath, commandPath, true)
	if req.DryRun {
		result.Links = append(result.Links, link)
	}
	return err
}

func (transaction *installPathTransaction) apply(result *port.NativeInstallResult) error {
	if transaction.managed {
		plan, err := transaction.command.Apply()
		result.CommandPath = managedCommandPathResult(plan)
		transaction.applied = plan.Adopted
		if err != nil {
			return err
		}
		result.Links = append(result.Links, port.InstallLink{Path: transaction.path, Target: transaction.req.BinPath, Created: true})
	} else {
		link, err := transaction.ensureSymlinkPlan(transaction.req.BinPath, transaction.path, false)
		result.Links = append(result.Links, link)
		if err != nil {
			return err
		}
		transaction.commandCreated = link.Created
	}
	shortLink, err := ensureShortCommandShimPlan(transaction.path, transaction.shortPath, false, transaction.ensureSymlinkPlan)
	result.Links = append(result.Links, shortLink)
	transaction.shortCreated = shortLink.Created
	return err
}

func (transaction *installPathTransaction) Apply(result *port.NativeInstallResult) error {
	return transaction.apply(result)
}

func (transaction *installPathTransaction) rollback(result *port.NativeInstallResult) error {
	var errs []error
	errs = transaction.rollbackShortShim(errs)
	errs = transaction.rollbackCommandLink(errs)
	if transaction.managed && transaction.command != nil && transaction.applied {
		plan, err := transaction.command.Rollback()
		if err == nil {
			transaction.applied = false
		}
		result.CommandPath = managedCommandPathResult(plan)
		errs = append(errs, err)
	}
	if !transaction.commandCreated && !transaction.shortCreated {
		errs = transaction.rollbackCreatedDirectories(errs)
	}
	return errors.Join(errs...)
}

func (transaction *installPathTransaction) rollbackShortShim(errs []error) []error {
	if !transaction.shortCreated {
		return errs
	}
	if err := removeInstallPathIfPresent(transaction.shortPath); err != nil {
		return append(errs, err)
	}
	transaction.shortCreated = false
	return errs
}

func (transaction *installPathTransaction) rollbackCommandLink(errs []error) []error {
	if transaction.commandExisted && !transaction.managed {
		if err := removeInstallPathIfPresent(transaction.path); err != nil {
			return append(errs, err)
		}
		transaction.commandCreated = false
		if err := os.Symlink(transaction.commandTarget, transaction.path); err != nil {
			errs = append(errs, err)
		}
		return errs
	}
	if transaction.commandCreated {
		if err := removeInstallPathIfPresent(transaction.path); err != nil {
			return append(errs, err)
		}
		transaction.commandCreated = false
	}
	return errs
}

func (transaction *installPathTransaction) rollbackCreatedDirectories(errs []error) []error {
	type cleanupPath struct {
		path    string
		existed bool
	}
	for _, item := range []cleanupPath{
		{path: filepath.Dir(transaction.path), existed: transaction.binDirExisted},
		{path: filepath.Dir(filepath.Dir(transaction.path)), existed: transaction.localDirExisted},
	} {
		if item.existed {
			continue
		}
		if err := removeInstallDirectoryIfEmpty(item.path); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func (transaction *installPathTransaction) Rollback(result *port.NativeInstallResult) error {
	return transaction.rollback(result)
}

func installPathDirectoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (transaction *installPathTransaction) finalize(result *port.NativeInstallResult) error {
	if !transaction.managed || transaction.command == nil {
		return nil
	}
	plan, err := transaction.command.Finalize()
	result.CommandPath = managedCommandPathResult(plan)
	return err
}

func (transaction *installPathTransaction) Finalize(result *port.NativeInstallResult) error {
	return transaction.finalize(result)
}

func managedCommandPathResult(plan installcontract.ManagedCommandPathPlan) *port.ManagedCommandPathResult {
	return &port.ManagedCommandPathResult{
		Path: plan.Path, Target: plan.Target, BackupPath: plan.BackupPath, AdoptionApproved: plan.AdoptionApproved,
		WouldAdopt: plan.WouldAdopt, Adopted: plan.Adopted, Committed: plan.Committed, RolledBack: plan.RolledBack,
		RollbackAvailable: plan.RollbackAvailable, BackupRetained: plan.BackupRetained,
	}
}

func planShellPath(result *port.NativeInstallResult, req port.NativeInstallRequest, mode string) error {
	commandPath := filepath.Join(req.Home, ".local", "bin", osfile.ExecutableName("issueops"))
	shortCommandPath := filepath.Join(req.Home, ".local", "bin", osfile.ExecutableName("io"))
	if mode == "manual" {
		result.Messages = append(result.Messages, `path-mode=manual: command shims are planned at `+commandPath+` and `+shortCommandPath+`; `+manualLocalBinPathHint)
		return nil
	}
	if mode == "skip" {
		result.Messages = append(result.Messages, "path-mode=skip: "+localBinPathKind+" PATH update skipped; command shims still use "+commandPath+" and "+shortCommandPath)
		return nil
	}
	if localBinInPath(req.Home) {
		return nil
	}
	return addLocalBinToPath(result, req)
}

func ensureShortCommandShimPlan(target, path string, dryRun bool, ensureSymlinkPlan func(string, string, bool) (port.InstallLink, error)) (port.InstallLink, error) {
	link := port.InstallLink{Path: path, Target: target}
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return ensureSymlinkPlan(target, path, dryRun)
	}
	if err != nil {
		return link, err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return link, fmt.Errorf("refusing to replace existing io command: %s", path)
	}
	current, err := os.Readlink(path)
	if err != nil {
		return link, err
	}
	if current != target {
		return link, fmt.Errorf("refusing to replace existing io command symlink %s -> %s", path, current)
	}
	return link, nil
}

func localBinInPath(home string) bool {
	localBin := filepath.Clean(filepath.Join(home, ".local", "bin"))
	for _, entry := range filepath.SplitList(os.Getenv("PATH")) {
		if filepath.Clean(entry) == localBin {
			return true
		}
	}
	return false
}
