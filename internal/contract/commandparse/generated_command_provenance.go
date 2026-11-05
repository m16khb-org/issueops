// Package commandparse owns the wire format of generated command provenance: the trailing
// --generated-by-executable, --generated-by-sha256 and --generated-for-generation flags that bind a
// command the harness printed to the binary and lease generation that produced it, plus the errors for malformed or mismatched flags.
package commandparse

import (
	"encoding/hex"
	"path/filepath"
	"strconv"
	"strings"
)

// Generated command provenance flag names, appended by BindGeneratedCommand and removed by ConsumeGeneratedCommandProvenance.
const (
	GeneratedByExecutableFlag  = "--generated-by-executable"
	GeneratedBySHA256Flag      = "--generated-by-sha256"
	GeneratedForGenerationFlag = "--generated-for-generation"
)

// GeneratedCommandProvenance identifies the binary and lease generation that generated a command:
// the absolute executable path, its lowercase hex SHA-256, and the execution lease generation.
type GeneratedCommandProvenance struct {
	ExecutablePath   string `json:"executable_path"`
	ExecutableSHA256 string `json:"executable_sha256"`
	LeaseGeneration  uint64 `json:"lease_generation"`
}

// Validate checks that the path is absolute, clean, control-free and at most 4096 bytes,
// the digest is 64 lowercase hex characters, and the lease generation is non-zero.
// It returns a *GeneratedCommandProvenanceError otherwise.
func (p GeneratedCommandProvenance) Validate() error {
	if p.ExecutablePath == "" || !filepath.IsAbs(p.ExecutablePath) || filepath.Clean(p.ExecutablePath) != p.ExecutablePath || len(p.ExecutablePath) > 4096 || containsControl(p.ExecutablePath) {
		return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command executable path is invalid", Expected: p}
	}
	if len(p.ExecutableSHA256) != 64 {
		return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command executable SHA-256 is invalid", Expected: p}
	}
	if _, err := hex.DecodeString(p.ExecutableSHA256); err != nil || p.ExecutableSHA256 != strings.ToLower(p.ExecutableSHA256) {
		return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command executable SHA-256 is invalid", Expected: p}
	}
	if p.LeaseGeneration == 0 {
		return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command lease generation is invalid", Expected: p}
	}
	return nil
}

// BindGeneratedCommand rewrites a generated command that starts with the "issueops" token so it runs the
// quoted provenance executable, ensures it asks for --json, and appends the three provenance flags.
// It fails for an empty command, a different leading token, or invalid provenance.
func BindGeneratedCommand(command string, provenance GeneratedCommandProvenance) (string, error) {
	if strings.TrimSpace(command) == "" {
		return "", &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command is empty", Expected: provenance}
	}
	if err := provenance.Validate(); err != nil {
		return "", err
	}
	tail, matched := strings.CutPrefix(command, "issueops")
	if !matched || (tail != "" && !strings.HasPrefix(tail, " ")) {
		return "", &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: "generated command executable token is invalid", Expected: provenance}
	}
	command = quoteGeneratedCommandArg(provenance.ExecutablePath) + tail
	// Every bound command answers JSON with --json; the reseed and claim
	// commands came without it and printed a text line that agents could not
	// parse (GitLab child QA).
	if !strings.Contains(" "+tail+" ", " --json ") {
		command += " --json"
	}
	return command +
		" " + GeneratedByExecutableFlag + " " + quoteGeneratedCommandArg(provenance.ExecutablePath) +
		" " + GeneratedBySHA256Flag + " " + provenance.ExecutableSHA256 +
		" " + GeneratedForGenerationFlag + " " + strconv.FormatUint(provenance.LeaseGeneration, 10), nil
}

// ConsumeGeneratedCommandProvenance strips the provenance flags (in "--flag value" or "--flag=value" form) from args
// and returns the remaining args and the parsed provenance. The bool reports whether any provenance flag was present;
// duplicated, valueless, incomplete, or invalid flags return an error.
func ConsumeGeneratedCommandProvenance(args []string) ([]string, GeneratedCommandProvenance, bool, error) {
	clean := make([]string, 0, len(args))
	values := map[string]string{}
	present := false
	for index := 0; index < len(args); index++ {
		name, value, matched := generatedCommandProvenanceArg(args[index])
		if !matched {
			clean = append(clean, args[index])
			continue
		}
		present = true
		if _, duplicate := values[name]; duplicate {
			return nil, GeneratedCommandProvenance{}, true, invalidGeneratedCommandProvenance("generated command provenance flag is duplicated")
		}
		if value == "" {
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "--") {
				return nil, GeneratedCommandProvenance{}, true, invalidGeneratedCommandProvenance("generated command provenance flag is missing a value")
			}
			index++
			value = args[index]
		}
		values[name] = value
	}
	if !present {
		return append([]string(nil), args...), GeneratedCommandProvenance{}, false, nil
	}
	for _, name := range []string{GeneratedByExecutableFlag, GeneratedBySHA256Flag, GeneratedForGenerationFlag} {
		if values[name] == "" {
			return nil, GeneratedCommandProvenance{}, true, invalidGeneratedCommandProvenance("generated command provenance envelope is incomplete")
		}
	}
	generation, err := strconv.ParseUint(values[GeneratedForGenerationFlag], 10, 64)
	if err != nil {
		return nil, GeneratedCommandProvenance{}, true, invalidGeneratedCommandProvenance("generated command lease generation is invalid")
	}
	evidence := GeneratedCommandProvenance{
		ExecutablePath: values[GeneratedByExecutableFlag], ExecutableSHA256: values[GeneratedBySHA256Flag], LeaseGeneration: generation,
	}
	if err := evidence.Validate(); err != nil {
		return nil, GeneratedCommandProvenance{}, true, err
	}
	return clean, evidence, true, nil
}

// GeneratedCommandProvenanceError reports a provenance failure with a stable Code.
// Expected and Observed carry the bound and the actually observed binary, and Generation the lease generation.
type GeneratedCommandProvenanceError struct {
	Code       string
	Message    string
	Expected   GeneratedCommandProvenance
	Observed   GeneratedCommandProvenance
	Generation uint64
}

// NewGeneratedCommandProvenanceObservationError returns the error for a failed binary provenance observation;
// the underlying cause is intentionally not exposed in the message.
func NewGeneratedCommandProvenanceObservationError(error) error {
	return &GeneratedCommandProvenanceError{
		Code:    "generated_command_provenance_observation_failed",
		Message: "generated command binary provenance observation failed",
	}
}

func (e *GeneratedCommandProvenanceError) Error() string {
	if e == nil {
		return "generated command provenance error"
	}
	return e.Message
}

// IssueOpsErrorFields returns the structured error fields (code, expected and observed executable and SHA-256, lease generation) for JSON error output.
func (e *GeneratedCommandProvenanceError) IssueOpsErrorFields() map[string]any {
	if e == nil {
		return nil
	}
	return map[string]any{
		"code":                e.Code,
		"expected_executable": e.Expected.ExecutablePath,
		"expected_sha256":     e.Expected.ExecutableSHA256,
		"observed_executable": e.Observed.ExecutablePath,
		"observed_sha256":     e.Observed.ExecutableSHA256,
		"lease_generation":    e.Generation,
	}
}

func containsControl(value string) bool {
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return true
		}
	}
	return false
}

func generatedCommandProvenanceArg(arg string) (string, string, bool) {
	for _, name := range []string{GeneratedByExecutableFlag, GeneratedBySHA256Flag, GeneratedForGenerationFlag} {
		if arg == name {
			return name, "", true
		}
		if after, ok := strings.CutPrefix(arg, name+"="); ok {
			return name, after, true
		}
	}
	return "", "", false
}

func invalidGeneratedCommandProvenance(message string) error {
	return &GeneratedCommandProvenanceError{Code: "generated_command_provenance_invalid", Message: message}
}

func quoteGeneratedCommandArg(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}
