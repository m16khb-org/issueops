package inspect

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strconv"
	"strings"
)

const redactedValue = "<redacted>"

var secretKeyPattern = regexp.MustCompile(`(?i)token|secret|password|authorization|api[_-]?key`)

type configReadError struct{ code string }

func (err configReadError) Error() string { return err.code }

type hostEntry struct {
	entry     map[string]any
	transport string
	sha256    string
}

func readJSONEntry(path string) (hostEntry, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return hostEntry{}, configReadError{"config_missing"}
		}
		return hostEntry{}, configReadError{"config_unreadable"}
	}
	servers, err := jsonServerEntries(body)
	if err != nil {
		return hostEntry{}, err
	}
	switch len(servers) {
	case 0:
		return hostEntry{}, configReadError{"entry_missing"}
	case 1:
	default:
		return hostEntry{}, configReadError{"entry_duplicate"}
	}
	var entry map[string]any
	decoder := json.NewDecoder(bytes.NewReader(servers[0]))
	decoder.UseNumber()
	if err := decoder.Decode(&entry); err != nil || entry == nil {
		return hostEntry{}, configReadError{"entry_malformed"}
	}
	return finishEntry(entry)
}

func jsonServerEntries(body []byte) ([]json.RawMessage, error) {
	if !json.Valid(body) {
		return nil, configReadError{"config_malformed"}
	}
	top, err := jsonObjectFields(body)
	if err != nil {
		return nil, configReadError{"config_malformed"}
	}
	var serverObjects []json.RawMessage
	for _, field := range top {
		if field.key == "mcpServers" {
			serverObjects = append(serverObjects, field.value)
		}
	}
	if len(serverObjects) == 0 {
		return nil, nil
	}
	if len(serverObjects) > 1 {
		return nil, configReadError{"entry_duplicate"}
	}
	fields, err := jsonObjectFields(serverObjects[0])
	if err != nil {
		return nil, configReadError{"config_malformed"}
	}
	var entries []json.RawMessage
	for _, field := range fields {
		if field.key == "issueops" {
			entries = append(entries, field.value)
		}
	}
	return entries, nil
}

type jsonField struct {
	key   string
	value json.RawMessage
}

func jsonObjectFields(body []byte) ([]jsonField, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	token, err := decoder.Token()
	if err != nil {
		return nil, err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, errors.New("not an object")
	}
	var fields []jsonField
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := keyToken.(string)
		if !ok {
			return nil, errors.New("non-string key")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		fields = append(fields, jsonField{key: key, value: value})
	}
	return fields, nil
}

func readCodexEntry(path string) (hostEntry, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return hostEntry{}, configReadError{"config_missing"}
		}
		return hostEntry{}, configReadError{"config_unreadable"}
	}
	entry, count, err := parseCodexIssueOpsTable(string(body))
	if err != nil {
		return hostEntry{}, configReadError{"config_malformed"}
	}
	switch count {
	case 0:
		return hostEntry{}, configReadError{"entry_missing"}
	case 1:
	default:
		return hostEntry{}, configReadError{"entry_duplicate"}
	}
	return finishEntry(entry)
}

func finishEntry(entry map[string]any) (hostEntry, error) {
	transport, err := entryTransport(entry)
	if err != nil {
		return hostEntry{}, err
	}
	canonical, err := json.Marshal(stripSecrets(entry))
	if err != nil {
		return hostEntry{}, configReadError{"entry_malformed"}
	}
	digest := sha256.Sum256(canonical)
	return hostEntry{entry: entry, transport: transport, sha256: hex.EncodeToString(digest[:])}, nil
}

func entryTransport(entry map[string]any) (string, error) {
	url, _ := entry["url"].(string)
	command, _ := entry["command"].(string)
	kind, _ := entry["type"].(string)
	switch {
	case url != "" && command != "":
		return "", configReadError{"entry_transport_ambiguous"}
	case url != "":
		if kind != "" && kind != "http" {
			return "", configReadError{"entry_type_unsupported"}
		}
		return "http", nil
	case command != "":
		if kind != "" && kind != "stdio" {
			return "", configReadError{"entry_type_unsupported"}
		}
		return "stdio", nil
	}
	return "", configReadError{"entry_transport_unknown"}
}

func stripSecrets(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if key == "headers" || key == "http_headers" || key == "env" {
				out[key] = redactValues(item)
				continue
			}
			if secretKeyPattern.MatchString(key) {
				out[key] = redactedValue
				continue
			}
			out[key] = stripSecrets(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = stripSecrets(item)
		}
		return out
	}
	return value
}

func redactValues(value any) any {
	table, ok := value.(map[string]any)
	if !ok {
		return redactedValue
	}
	out := make(map[string]any, len(table))
	for key := range table {
		out[key] = redactedValue
	}
	return out
}

// parseCodexIssueOpsTable은 Codex config.toml에서 [mcp_servers.issueops] 표와 그
// 하위 표만 해석한다. 다른 표는 헤더 형식만 확인한다.
func parseCodexIssueOpsTable(text string) (map[string]any, int, error) {
	const table = "mcp_servers.issueops"
	entry := map[string]any{}
	count := 0
	current := ""
	lines := strings.Split(text, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(stripTOMLComment(lines[i]))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			if !strings.HasSuffix(line, "]") {
				return nil, 0, errors.New("unterminated table header")
			}
			inner := strings.TrimSpace(strings.Trim(line, "[]"))
			current = strings.ReplaceAll(strings.ReplaceAll(inner, " ", ""), `"`, "")
			if strings.HasPrefix(line, "[[") {
				current = ""
			}
			if current == table {
				count++
			}
			continue
		}
		inTable := current == table
		inSub := strings.HasPrefix(current, table+".")
		inParent := current == "mcp_servers"
		if !inTable && !inSub && !inParent {
			continue
		}
		key, rawValue, ok := strings.Cut(line, "=")
		if !ok {
			return nil, 0, errors.New("expected key = value")
		}
		key = strings.Trim(strings.TrimSpace(key), `"'`)
		rawValue = strings.TrimSpace(rawValue)
		for bracketDepth(rawValue) > 0 && i+1 < len(lines) {
			i++
			rawValue += " " + strings.TrimSpace(stripTOMLComment(lines[i]))
		}
		if inParent && key != "issueops" {
			continue
		}
		value, err := parseTOMLValue(rawValue)
		if err != nil {
			return nil, 0, err
		}
		switch {
		case inParent:
			inline, ok := value.(map[string]any)
			if !ok {
				return nil, 0, errors.New("issueops must be a table")
			}
			count++
			for k, v := range inline {
				entry[k] = v
			}
		case inTable:
			entry[key] = value
		default:
			sub := strings.TrimPrefix(current, table+".")
			nested, _ := entry[sub].(map[string]any)
			if nested == nil {
				nested = map[string]any{}
				entry[sub] = nested
			}
			nested[key] = value
		}
	}
	return entry, count, nil
}

func stripTOMLComment(line string) string {
	var quote byte
	escaped := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case quote == '"' && escaped:
			escaped = false
		case quote == '"' && c == '\\':
			escaped = true
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && (c == '"' || c == '\''):
			quote = c
		case quote == 0 && c == '#':
			return line[:i]
		}
	}
	return line
}

func bracketDepth(text string) int {
	depth := 0
	var quote byte
	escaped := false
	for i := 0; i < len(text); i++ {
		c := text[i]
		switch {
		case quote == '"' && escaped:
			escaped = false
		case quote == '"' && c == '\\':
			escaped = true
		case quote != 0 && c == quote:
			quote = 0
		case quote == 0 && (c == '"' || c == '\''):
			quote = c
		case quote == 0 && (c == '[' || c == '{'):
			depth++
		case quote == 0 && (c == ']' || c == '}'):
			depth--
		}
	}
	return depth
}

func parseTOMLValue(text string) (any, error) {
	reader := &tomlReader{text: text}
	value, err := reader.value()
	if err != nil {
		return nil, err
	}
	reader.skipSpace()
	if reader.pos != len(reader.text) {
		return nil, errors.New("trailing characters after value")
	}
	return value, nil
}

type tomlReader struct {
	text string
	pos  int
}

func (reader *tomlReader) skipSpace() {
	for reader.pos < len(reader.text) && (reader.text[reader.pos] == ' ' || reader.text[reader.pos] == '\t') {
		reader.pos++
	}
}

func (reader *tomlReader) value() (any, error) {
	reader.skipSpace()
	if reader.pos >= len(reader.text) {
		return nil, io.ErrUnexpectedEOF
	}
	switch reader.text[reader.pos] {
	case '"':
		return reader.basicString()
	case '\'':
		end := strings.IndexByte(reader.text[reader.pos+1:], '\'')
		if end < 0 {
			return nil, errors.New("unterminated literal string")
		}
		value := reader.text[reader.pos+1 : reader.pos+1+end]
		reader.pos += end + 2
		return value, nil
	case '[':
		return reader.array()
	case '{':
		return reader.inlineTable()
	}
	return reader.scalar()
}

func (reader *tomlReader) basicString() (any, error) {
	start := reader.pos
	reader.pos++
	for reader.pos < len(reader.text) {
		switch reader.text[reader.pos] {
		case '\\':
			reader.pos += 2
			continue
		case '"':
			reader.pos++
			value, err := strconv.Unquote(reader.text[start:reader.pos])
			if err != nil {
				return nil, fmt.Errorf("invalid string: %w", err)
			}
			return value, nil
		}
		reader.pos++
	}
	return nil, errors.New("unterminated string")
}

func (reader *tomlReader) array() (any, error) {
	reader.pos++
	items := []any{}
	for {
		reader.skipSpace()
		if reader.pos >= len(reader.text) {
			return nil, io.ErrUnexpectedEOF
		}
		if reader.text[reader.pos] == ']' {
			reader.pos++
			return items, nil
		}
		item, err := reader.value()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
		reader.skipSpace()
		if reader.pos < len(reader.text) && reader.text[reader.pos] == ',' {
			reader.pos++
		}
	}
}

func (reader *tomlReader) inlineTable() (any, error) {
	reader.pos++
	table := map[string]any{}
	for {
		reader.skipSpace()
		if reader.pos >= len(reader.text) {
			return nil, io.ErrUnexpectedEOF
		}
		if reader.text[reader.pos] == '}' {
			reader.pos++
			return table, nil
		}
		keyEnd := strings.IndexAny(reader.text[reader.pos:], "=}")
		if keyEnd < 0 || reader.text[reader.pos+keyEnd] != '=' {
			return nil, errors.New("inline table key without value")
		}
		key := strings.Trim(strings.TrimSpace(reader.text[reader.pos:reader.pos+keyEnd]), `"'`)
		reader.pos += keyEnd + 1
		item, err := reader.value()
		if err != nil {
			return nil, err
		}
		table[key] = item
		reader.skipSpace()
		if reader.pos < len(reader.text) && reader.text[reader.pos] == ',' {
			reader.pos++
		}
	}
}

func (reader *tomlReader) scalar() (any, error) {
	end := reader.pos
	for end < len(reader.text) && !strings.ContainsRune(",]} \t", rune(reader.text[end])) {
		end++
	}
	token := reader.text[reader.pos:end]
	reader.pos = end
	switch token {
	case "true":
		return true, nil
	case "false":
		return false, nil
	}
	cleaned := strings.ReplaceAll(token, "_", "")
	if number, err := strconv.ParseInt(cleaned, 0, 64); err == nil {
		return number, nil
	}
	if number, err := strconv.ParseFloat(cleaned, 64); err == nil {
		return number, nil
	}
	return nil, fmt.Errorf("invalid value %q", token)
}
