// Package pi reads the stream of pi --mode json into runscreen's events.
package pi

import (
	"encoding/json"

	"github.com/thellmwhisperer/runscreen"
	"github.com/thellmwhisperer/runscreen/adapter/internal/tool"
)

type event struct {
	Type    string
	Message struct {
		Role, Model, Provider string
		Content               []tool.Block
	}
	ToolName   string `json:"toolName"`
	ToolCallID string `json:"toolCallId"`
	IsError    bool   `json:"isError"`
	Args       json.RawMessage
	Result     json.RawMessage
}

// Read is the events of line, one JSON line of the stream: what the agent says and thinks once its message ends,
// its tool calls as they start and end. A line it does not know has none.
func Read(line []byte) []runscreen.Event {
	var e event
	_ = json.Unmarshal(line, &e) // a field of another type is left empty, the rest is read
	switch e.Type {
	case "message_end":
		return tool.Blocks(e.Message.Content, e.Message.Role == "assistant")
	case "tool_execution_start":
		return []runscreen.Event{tool.Call(e.ToolCallID, e.ToolName, tool.Detail(e.Args), runscreen.Running)}
	case "tool_execution_end":
		var r struct {
			IsError bool
			Content json.RawMessage
		}
		_ = json.Unmarshal(e.Result, &r)
		return []runscreen.Event{tool.Summed(tool.Call(e.ToolCallID, e.ToolName, "", tool.Result(e.IsError || r.IsError)),
			tool.ContentText(r.Content))}
	}
	return nil
}

// Model is the provider and model an assistant message's end names, as provider/model; "" on any other line.
func Model(line []byte) string {
	var e event
	_ = json.Unmarshal(line, &e)
	if e.Type != "message_end" || e.Message.Role != "assistant" || e.Message.Model == "" {
		return ""
	}
	if e.Message.Provider != "" {
		return e.Message.Provider + "/" + e.Message.Model
	}
	return e.Message.Model
}
