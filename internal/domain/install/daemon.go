package install

func MayTerminateDaemon(pid, currentPID int) bool { return pid != currentPID }
