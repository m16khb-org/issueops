package daemon

// ProcessInspector keeps the host observation command and environment fixed for one caller.
type ProcessInspector struct {
	PSExecutable  string
	PSLookupError error
	Environment   []string
}
