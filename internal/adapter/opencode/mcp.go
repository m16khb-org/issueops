package opencode

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"issueops/internal/port"
)

const (
	mcpCatalogSHA256Env = "ISSUEOPS_MCP_CATALOG_SHA256"
	// mcpCatalogHeader carries the catalog digest on the HTTP entry, so a
	// changed tool catalog changes the entry and opencode reconnects on
	// reload. The server ignores it.
	mcpCatalogHeader = "X-Issueops-Mcp-Catalog-Sha256"
	mcpTransportHTTP = "http"
)

// mcpServersPath is where opencode v2 keeps MCP servers in opencode.json.
var mcpServersPath = []string{"mcp", "servers"}

func (installer Installer) writeUserMCP(path string, req port.NativeInstallRequest) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "opencode_user_mcp_config"}
	if err := requirePlainJSON(path); err != nil {
		return file, err
	}
	config, err := installer.deps.MergeJSONMapFile(path, mcpServersPath, "issueops", req.DryRun, func() (map[string]any, error) {
		return installer.userMCPServer(req)
	})
	if err != nil {
		return file, fmt.Errorf("%s: %w", path, err)
	}
	// The HTTP entry holds a bearer token, so an existing file is narrowed too.
	if req.MCPTransport == mcpTransportHTTP && !req.DryRun {
		if err := os.Chmod(path, 0o600); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return file, err
		}
	}
	return installer.deps.WriteJSONPlan(path, file.Kind, config, 0o600, req.DryRun)
}

func (installer Installer) removeProjectMCP(path string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: "opencode_project_mcp_config"}
	if err := requirePlainJSON(path); err != nil {
		return file, err
	}
	config, removed, err := installer.deps.RemoveJSONMapEntry(path, mcpServersPath, "issueops_project")
	if err != nil {
		return file, fmt.Errorf("%s: %w", path, err)
	}
	if !removed {
		return file, nil
	}
	return installer.deps.WriteJSONPlan(path, file.Kind, config, 0o644, dryRun)
}

func (installer Installer) writeProjectMCP(path, kind string, dryRun bool) (port.InstallFile, error) {
	file := port.InstallFile{Path: path, Kind: kind}
	server, err := installer.localMCPServer("./bin/issueops", ".")
	if err != nil {
		return file, err
	}
	config := map[string]any{"mcp": map[string]any{"servers": map[string]any{"issueops_project": server}}}
	return installer.deps.WriteJSONPlan(path, kind, config, 0o644, dryRun)
}

// userMCPServer is the managed entry. codemode is off so the tools appear as
// native issueops_* tools; oauth is off because the bearer header is the auth.
func (installer Installer) userMCPServer(req port.NativeInstallRequest) (map[string]any, error) {
	if req.MCPTransport != mcpTransportHTTP {
		return installer.localMCPServer(req.BinPath, req.Root)
	}
	catalogSHA256, err := installer.catalogSHA256()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type": "remote",
		"url":  req.MCPURL,
		"headers": map[string]any{
			"Authorization":  "Bearer " + req.MCPBearer,
			mcpCatalogHeader: catalogSHA256,
		},
		"oauth":    false,
		"codemode": false,
	}, nil
}

func (installer Installer) localMCPServer(command, root string) (map[string]any, error) {
	catalogSHA256, err := installer.catalogSHA256()
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"type":    "local",
		"command": []string{command, "mcp"},
		"environment": map[string]any{
			"ISSUEOPS_ROOT":     root,
			mcpCatalogSHA256Env: catalogSHA256,
		},
		"codemode": false,
	}, nil
}

func (installer Installer) catalogSHA256() (string, error) {
	if installer.deps.MCPCatalogSHA256 == nil {
		return "", fmt.Errorf("opencode MCP catalog digest is not configured")
	}
	catalogSHA256, err := installer.deps.MCPCatalogSHA256()
	if err != nil {
		return "", fmt.Errorf("compute opencode MCP catalog digest: %w", err)
	}
	if catalogSHA256 == "" {
		return "", fmt.Errorf("compute opencode MCP catalog digest: empty digest")
	}
	return catalogSHA256, nil
}

// requirePlainJSON stops the install when an existing opencode.json holds
// comments or trailing commas. opencode accepts them, but the managed merge
// writes plain JSON back, which would drop the user's comments.
func requirePlainJSON(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		// A missing file is created; any other read error surfaces from the merge.
		return nil
	}
	if !bytes.Equal(normalizeJSONC(raw), raw) {
		return fmt.Errorf("%s: opencode.json has comments or trailing commas, and issueops rewrites the file as plain JSON; move the commented settings into opencode.jsonc or remove the comments, then install again", path)
	}
	return nil
}

// checkJSONCShadow reads opencode.jsonc, which opencode loads after
// opencode.json and which replaces a same-named server object whole. issueops
// never edits it, to keep the user's comments, so an issueops entry there is an
// error. A file that cannot be read only warns: it must not block the other
// hosts' installs.
func checkJSONCShadow(path string) (warning string, err error) {
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	unreadable := path + ": opencode.jsonc could not be read; make sure it does not define mcp.servers.issueops"
	if err != nil {
		return unreadable, nil
	}
	var config map[string]any
	if err := json.Unmarshal(normalizeJSONC(raw), &config); err != nil {
		return unreadable, nil
	}
	mcp, _ := config["mcp"].(map[string]any)
	servers, _ := mcp["servers"].(map[string]any)
	if _, ok := servers["issueops"]; ok {
		return "", fmt.Errorf("%s: opencode.jsonc defines mcp.servers.issueops and would shadow the managed entry; remove it from opencode.jsonc", path)
	}
	return "", nil
}

// normalizeJSONC turns JSONC into JSON: it blanks // and /* */ comments and
// drops trailing commas, leaving string contents (and their escapes) intact.
func normalizeJSONC(src []byte) []byte {
	out := make([]byte, 0, len(src))
	inString, escaped := false, false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inString:
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/':
			end, replacement, ok := commentEnd(src, i)
			if !ok {
				out = append(out, c)
				break
			}
			i = end
			if replacement != 0 {
				out = append(out, replacement)
			}
		case c == '}' || c == ']':
			out = dropTrailingComma(out)
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return out
}

// commentEnd reports whether a comment starts at the '/' at index start. If so,
// it returns the last index the comment consumes and the byte that replaces
// it: a // comment consumes its ending newline and emits it again (nothing at
// end of input), and a /* */ comment, terminated or not, becomes one space.
func commentEnd(src []byte, start int) (end int, replacement byte, ok bool) {
	if start+1 >= len(src) {
		return start, 0, false
	}
	switch src[start+1] {
	case '/':
		end = start
		for end < len(src) && src[end] != '\n' {
			end++
		}
		if end == len(src) {
			return end, 0, true
		}
		return end, '\n', true
	case '*':
		end = start + 2
		for end+1 < len(src) && (src[end] != '*' || src[end+1] != '/') {
			end++
		}
		return end + 1, ' ', true
	}
	return start, 0, false
}

// dropTrailingComma removes a comma that is the last non-space byte, which can
// only sit outside a string because strings end in a quote.
func dropTrailingComma(out []byte) []byte {
	end := len(out)
	for end > 0 && (out[end-1] == ' ' || out[end-1] == '\t' || out[end-1] == '\n' || out[end-1] == '\r') {
		end--
	}
	if end > 0 && out[end-1] == ',' {
		return append(out[:end-1], out[end:]...)
	}
	return out
}
