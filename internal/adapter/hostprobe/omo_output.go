package hostprobe

import (
	"bytes"
	"encoding/json"
)

// omoOutput retains bounded evidence while draining display-only JSONL events.
// Both a single event and the retained stream have the normal output limit.
type omoOutput struct {
	output *boundedBuffer
	line   []byte
}

func (o *omoOutput) Write(data []byte) (int, error) {
	n := len(data)
	for len(data) > 0 && !o.output.truncated {
		end := bytes.IndexByte(data, '\n')
		if end < 0 {
			end = len(data)
		}
		if len(o.line)+end > MaxOutputBytes {
			o.output.truncated = true
			o.line = nil
			break
		}
		o.line = append(o.line, data[:end]...)
		if end == len(data) {
			break
		}
		o.flush()
		data = data[end+1:]
	}
	return n, nil
}

func (o *omoOutput) flush() {
	if len(o.line) == 0 || o.output.truncated {
		return
	}
	var event struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(o.line, &event) != nil ||
		(event.Type != "message_update" && event.Type != "tool_hook_status") {
		_, _ = o.output.Write(o.line)
		_, _ = o.output.Write([]byte{'\n'})
	}
	o.line = o.line[:0]
}
