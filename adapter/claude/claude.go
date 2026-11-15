// Package claude reads the stream of Claude Code's --output-format stream-json into runscreen's events.
package claude

import (
	"encoding/json"

	"github.com/thellmwhisperer/runscreen"
	"github.com/thellmwhisperer/runscreen/adapter/internal/tool"
)

type event struct {
	Type, Subtype, Model string
	Message              struct {
		Role    string
		Content []tool.Block
	}
}

// Read is the events of line, one JSON line of the stream: what the agent says and thinks, its tool calls and
// their results. A line it does not know has none.
func Read(line []byte) []runscreen.Event {
	var e event
	_ = json.Unmarshal(line, &e) // a field of another type is left empty, the rest is read
	return tool.Blocks(e.Message.Content, e.Type == "assistant" || e.Message.Role == "assistant")
}

// Model is the model the stream's init line names; "" on any other line.
func Model(line []byte) string {
	var e event
	if json.Unmarshal(line, &e) != nil || e.Type != "system" || e.Subtype != "init" {
		return ""
	}
	return e.Model
}
