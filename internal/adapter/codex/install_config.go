package codex

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"issueops/internal/port"
)

func (installer Installer) writeGlobalConfig(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "codex_user_mcp_config"}
	text := ""
	if b, err := os.ReadFile(path); err == nil {
		text = string(b)
		if !req.DryRun {
			backup := path + ".harness.bak"
			if _, statErr := os.Stat(backup); errors.Is(statErr, fs.ErrNotExist) {
				if writeErr := os.WriteFile(backup, []byte(text), 0o600); writeErr != nil {
					return file, writeErr
				}
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) && !req.DryRun {
		return file, err
	}
	for _, section := range []string{"mcp_servers.issueops", "mcp_servers.issueops.env", "mcp_servers.issueops.http_headers"} {
		text = removeTOMLSection(text, section)
	}
	if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	if strings.TrimSpace(text) != "" && !strings.HasSuffix(text, "\n\n") {
		text += "\n"
	}
	text += installer.codexGlobalBlock(req)
	existing, _ := os.ReadFile(path)
	if string(existing) == text {
		return file, nil
	}
	if req.DryRun {
		file.WouldWrite = true
		return file, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return file, err
	}
	if err := restrictToOwner(path, req); err != nil {
		return file, err
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		return file, err
	}
	file.Written = true
	return file, nil
}

func (installer Installer) codexGlobalBlock(req port.NativeInstallRequest) string {
	if req.MCPTransport == mcpTransportHTTP {
		return fmt.Sprintf("[mcp_servers.issueops]\nurl = %s\nhttp_headers = { Authorization = %s }\n",
			installer.deps.TOMLString(req.MCPURL), installer.deps.TOMLString("Bearer "+req.MCPBearer))
	}
	return fmt.Sprintf(`[mcp_servers.issueops]
command = %s
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.issueops.env]
ISSUEOPS_ROOT = %s
`, installer.deps.TOMLString(req.BinPath), installer.deps.TOMLString(req.Root))
}

const mcpTransportHTTP = "http"

// restrictToOwner narrows an existing config to 0600 before a bearer is
// written into it; os.WriteFile keeps the mode of an existing file.
func restrictToOwner(path string, req port.NativeInstallRequest) error {
	if req.MCPTransport != mcpTransportHTTP || req.DryRun {
		return nil
	}
	if err := os.Chmod(path, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func codexTemplate(req port.NativeInstallRequest) string {
	return `[mcp_servers.issueops]
command = "./bin/issueops"
args = ["mcp"]
startup_timeout_sec = 30

[mcp_servers.issueops.env]
ISSUEOPS_ROOT = "."
`
}

func removeTOMLSection(src, section string) string {
	marker := "[" + section + "]"
	for {
		pos := strings.Index(src, marker)
		if pos < 0 {
			return src
		}
		next := strings.Index(src[pos+len(marker):], "\n[")
		if next < 0 {
			src = strings.TrimRight(src[:pos], " \t\r\n") + "\n"
			continue
		}
		nextPos := pos + len(marker) + next + 1
		src = src[:pos] + src[nextPos:]
	}
}
