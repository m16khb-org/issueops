package daemoncli

type Paths = daemonPaths
type Status = daemonStatus

func RunDaemon(args []string) error {
	return runDaemon(args)
}

func RunMCPProxy() error {
	return runMCPProxy()
}

func CheckDaemonStatus() Status {
	return checkDaemonStatus()
}

func DaemonStatusForMCP() Status {
	return daemonStatusForMCP()
}
