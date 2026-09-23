package updatecli

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

var postInstallDaemonRefresh = refreshRunningDaemonAfterInstall
var daemonProcessLister = listDaemonProcesses
var daemonProcessTerminator = terminateDaemonProcess
var installedDaemonCommandRunner = runInstalledDaemonCommand

type daemonProcess struct {
	PID     int
	Command string
}

// refreshRunningDaemonAfterInstall은 설치 뒤 남아 있는 daemon을 내리기만 한다.
// issueops mcp는 host 세션 안에서 in-process로 동작하므로 새 세션은 daemon을 쓰지
// 않는다. 이전 binary로 떠 있는 MCP proxy는 연결이 끊기면 스스로 daemon을 다시
// 띄우고, 그때 설치된 새 binary가 실행된다. 쓰는 곳이 없는 daemon은 띄우지 않는다.
func refreshRunningDaemonAfterInstall() (bool, error) {
	binary := filepath.Join(deps.IssueOpsRoot(), "bin", "issueops")
	if err := installedDaemonCommandRunner(binary, "daemon", "stop", "--json"); err != nil {
		return true, err
	}
	if _, err := terminateStaleDaemonProcesses(); err != nil {
		return true, err
	}
	return true, nil
}

func runInstalledDaemonCommand(binary string, args ...string) error {
	cmd := exec.Command(binary, args...)
	cmd.Dir = deps.IssueOpsRoot()
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		command := strings.Join(args, " ")
		detail := strings.TrimSpace(stderr.String())
		if detail == "" {
			return fmt.Errorf("run installed daemon command %q: %w", command, err)
		}
		return fmt.Errorf("run installed daemon command %q: %w: %s", command, err, detail)
	}
	return nil
}

func terminateStaleDaemonProcesses() (int, error) {
	processes, err := daemonProcessLister()
	if err != nil {
		return 0, err
	}
	currentPID := os.Getpid()
	terminated := 0
	for _, process := range processes {
		if process.PID == currentPID {
			continue
		}
		if err := daemonProcessTerminator(process.PID); err != nil {
			return terminated, err
		}
		terminated++
	}
	return terminated, nil
}

func listDaemonProcesses() ([]daemonProcess, error) {
	out, err := exec.Command("ps", "-axo", "pid=,command=").Output()
	if err != nil {
		// ps may be unavailable in sandboxed environments; treat as no matching processes.
		return nil, nil
	}
	binary := filepath.Join(deps.IssueOpsRoot(), "bin", "issueops")
	var processes []daemonProcess
	for _, line := range strings.Split(string(out), "\n") {
		process, ok := parseDaemonProcess(line, binary)
		if ok {
			processes = append(processes, process)
		}
	}
	return processes, nil
}

func parseDaemonProcess(line, binary string) (daemonProcess, bool) {
	line = strings.TrimSpace(line)
	if line == "" {
		return daemonProcess{}, false
	}
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return daemonProcess{}, false
	}
	pid, err := strconv.Atoi(fields[0])
	if err != nil {
		return daemonProcess{}, false
	}
	command := strings.Join(fields[1:], " ")
	if command != binary+" daemon --internal" {
		return daemonProcess{}, false
	}
	return daemonProcess{PID: pid, Command: command}, true
}

func terminateDaemonProcess(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}
