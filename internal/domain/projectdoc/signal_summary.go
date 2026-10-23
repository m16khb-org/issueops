package projectdoc

// MaxEchoedSignalFiles bounds how many scanned files a bootstrap plan echoes.
// The full list made the plan 80K characters for this repository, past what
// Claude Code accepts as one MCP result; the evidence an agent checks is the
// detected commands, profile, and planned files.
const MaxEchoedSignalFiles = 100

// EchoedSignals returns signals for a result: the first MaxEchoedSignalFiles
// files plus files_total when the scan found more. The input is not changed.
func EchoedSignals(signals ProjectSignals) ProjectSignals {
	if len(signals.Files) <= MaxEchoedSignalFiles {
		return signals
	}
	signals.FilesTotal = len(signals.Files)
	signals.Files = append([]string(nil), signals.Files[:MaxEchoedSignalFiles]...)
	return signals
}
