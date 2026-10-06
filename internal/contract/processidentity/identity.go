package processidentity

// Identity is the OS observation that binds a recorded PID to one process
// lifetime: its canonical start time and resolved executable path.
type Identity struct {
	StartTime  string
	Executable string
}
