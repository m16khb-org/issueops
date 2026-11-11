//go:build windows

package osprocess

import (
	"errors"
	"os"
	"slices"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// KillTree terminates pid and every live descendant, deepest first. Windows
// has no process groups, so the tree is read from one process snapshot at
// kill time. A child is counted only when it started after its parent, which
// rejects entries whose parent PID was reused. It returns os.ErrProcessDone
// when pid no longer runs.
func KillTree(pid int) error {
	if !Alive(pid) {
		return os.ErrProcessDone
	}
	tree, err := descendants(uint32(pid))
	if err != nil {
		return err
	}
	var errs []error
	for _, child := range slices.Backward(tree) {
		if err := terminateProcess(child); err != nil && !errors.Is(err, os.ErrProcessDone) {
			errs = append(errs, err)
		}
	}
	if err := terminateProcess(uint32(pid)); err != nil {
		if errors.Is(err, os.ErrProcessDone) && len(errs) == 0 {
			return os.ErrProcessDone
		}
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

// Alive reports whether a process with pid exists and has not exited.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Access denied still proves the process exists.
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	return running(handle)
}

// running waits zero time on the process handle, which is signaled at exit.
// Unlike an exit code of STILL_ACTIVE, this cannot mistake an exit status of 259.
func running(handle windows.Handle) bool {
	event, err := windows.WaitForSingleObject(handle, 0)
	return err == nil && event == uint32(windows.WAIT_TIMEOUT)
}

// Terminate ends pid alone, the Windows counterpart of SIGKILL to one
// process. It returns os.ErrProcessDone when pid no longer runs.
func Terminate(pid int) error {
	if pid <= 0 {
		return os.ErrProcessDone
	}
	return terminateProcess(uint32(pid))
}

func terminateProcess(pid uint32) error {
	handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if errors.Is(err, windows.ERROR_INVALID_PARAMETER) {
		return os.ErrProcessDone
	}
	if err != nil {
		return os.NewSyscallError("OpenProcess", err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	if !running(handle) {
		return os.ErrProcessDone
	}
	if err := windows.TerminateProcess(handle, 1); err != nil {
		return os.NewSyscallError("TerminateProcess", err)
	}
	return nil
}

// descendants returns the descendants of root in breadth-first order.
func descendants(root uint32) ([]uint32, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, os.NewSyscallError("CreateToolhelp32Snapshot", err)
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()
	children := map[uint32][]uint32{}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ProcessID != entry.ParentProcessID {
			children[entry.ParentProcessID] = append(children[entry.ParentProcessID], entry.ProcessID)
		}
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, os.NewSyscallError("Process32Next", err)
	}
	var tree []uint32
	queue := []uint32{root}
	for len(queue) > 0 {
		parent := queue[0]
		queue = queue[1:]
		parentStart, ok := startTime(parent)
		if !ok {
			continue
		}
		for _, child := range children[parent] {
			if childStart, ok := startTime(child); ok && childStart >= parentStart {
				tree = append(tree, child)
				queue = append(queue, child)
			}
		}
	}
	return tree, nil
}

func startTime(pid uint32) (int64, bool) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, false
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return 0, false
	}
	return creation.Nanoseconds(), true
}

// Entry is one process from a snapshot. Started is zero when the process
// could not be opened to read its creation time.
type Entry struct {
	PID        int
	ParentPID  int
	Started    time.Time
	Executable string
}

// Snapshot lists every process with its creation time and executable, the
// Windows counterpart of `ps -axo pid=,ppid=,lstart=,comm=`. Executable is
// the full image path when readable and the image file name otherwise.
func Snapshot() ([]Entry, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, os.NewSyscallError("CreateToolhelp32Snapshot", err)
	}
	defer func() { _ = windows.CloseHandle(snapshot) }()
	var entries []Entry
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		item := Entry{PID: int(entry.ProcessID), ParentPID: int(entry.ParentProcessID), Executable: windows.UTF16ToString(entry.ExeFile[:])}
		if started, executable, err := Identity(item.PID); err == nil {
			item.Started, item.Executable = started, executable
		}
		entries = append(entries, item)
	}
	if !errors.Is(err, windows.ERROR_NO_MORE_FILES) {
		return nil, os.NewSyscallError("Process32Next", err)
	}
	return entries, nil
}

// Identity returns the creation time, truncated to seconds like ps lstart,
// and the full image path of pid.
func Identity(pid int) (time.Time, string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return time.Time{}, "", os.NewSyscallError("OpenProcess", err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &creation, &exit, &kernel, &user); err != nil {
		return time.Time{}, "", os.NewSyscallError("GetProcessTimes", err)
	}
	buffer := make([]uint16, windows.MAX_LONG_PATH)
	size := uint32(len(buffer))
	if err := windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return time.Time{}, "", os.NewSyscallError("QueryFullProcessImageName", err)
	}
	started := time.Unix(0, creation.Nanoseconds()).UTC().Truncate(time.Second)
	return started, windows.UTF16ToString(buffer[:size]), nil
}

// CommandLine returns the command line pid was started with.
func CommandLine(pid int) (string, error) {
	return readParameter(pid, func(p *windows.RTL_USER_PROCESS_PARAMETERS) windows.NTUnicodeString { return p.CommandLine })
}

// CurrentDirectory returns the working directory of pid.
func CurrentDirectory(pid int) (string, error) {
	return readParameter(pid, func(p *windows.RTL_USER_PROCESS_PARAMETERS) windows.NTUnicodeString {
		return p.CurrentDirectory.DosPath
	})
}

// readParameter reads one string from the process parameters in pid's PEB.
func readParameter(pid int, field func(*windows.RTL_USER_PROCESS_PARAMETERS) windows.NTUnicodeString) (string, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION|windows.PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return "", os.NewSyscallError("OpenProcess", err)
	}
	defer func() { _ = windows.CloseHandle(handle) }()
	var basic windows.PROCESS_BASIC_INFORMATION
	if err := windows.NtQueryInformationProcess(handle, windows.ProcessBasicInformation,
		unsafe.Pointer(&basic), uint32(unsafe.Sizeof(basic)), nil); err != nil {
		return "", os.NewSyscallError("NtQueryInformationProcess", err)
	}
	var peb windows.PEB
	if err := readMemory(handle, uintptr(unsafe.Pointer(basic.PebBaseAddress)), unsafe.Pointer(&peb), unsafe.Sizeof(peb)); err != nil {
		return "", err
	}
	var params windows.RTL_USER_PROCESS_PARAMETERS
	// Read only through CommandLine: the tail of the structure varies by release.
	if err := readMemory(handle, uintptr(unsafe.Pointer(peb.ProcessParameters)), unsafe.Pointer(&params),
		unsafe.Offsetof(params.Environment)); err != nil {
		return "", err
	}
	value := field(&params)
	if value.Length == 0 {
		return "", nil
	}
	text := make([]uint16, value.Length/2)
	if err := readMemory(handle, uintptr(unsafe.Pointer(value.Buffer)), unsafe.Pointer(&text[0]), uintptr(value.Length)); err != nil {
		return "", err
	}
	return windows.UTF16ToString(text), nil
}

func readMemory(handle windows.Handle, address uintptr, target unsafe.Pointer, size uintptr) error {
	var read uintptr
	if err := windows.ReadProcessMemory(handle, address, (*byte)(target), size, &read); err != nil {
		return os.NewSyscallError("ReadProcessMemory", err)
	}
	if read != size {
		return errors.New("ReadProcessMemory returned a short read")
	}
	return nil
}
