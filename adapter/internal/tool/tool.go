// Package tool is what the adapters share: the events of a tool call and of what the agent says, from the
// fields each CLI's stream names them by.
package tool

import (
	"encoding/json"
	"strings"

	"github.com/thellmwhisperer/runscreen"
)

// Call is the event of tool call id: name's call with detail, its result so far.
func Call(id, name, detail string, result runscreen.State) runscreen.Event {
	return runscreen.Event{Kind: runscreen.ToolCall, ID: id, Tool: name, Detail: detail, Result: result}
}

// Summed is tool call e with its summary: the first line of its output, or of its error, when it has one.
func Summed(e runscreen.Event, output string) runscreen.Event {
	line, _, _ := strings.Cut(output, "\n")
	e.Summary = strings.TrimSpace(line)
	return e
}

// Result is a tool call's end: Failed or Passed.
func Result(failed bool) runscreen.State {
	if failed {
		return runscreen.Failed
	}
	return runscreen.Passed
}

// Says is the agent saying text; nothing when text is blank.
func Says(text string) []runscreen.Event {
	if text = strings.TrimSpace(text); text == "" {
		return nil
	}
	return []runscreen.Event{{Kind: runscreen.Says, Text: text}}
}

// Thinks is the agent thinking text; nothing when text is blank, as a thinking block with only its signature is.
func Thinks(text string) []runscreen.Event {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	return []runscreen.Event{{Kind: runscreen.Thinks, Text: text}}
}

// Detail is what a tool call's input is about, in one line: its command, path, pattern or skill.
func Detail(input json.RawMessage) string {
	var in map[string]any
	_ = json.Unmarshal(input, &in)
	for _, k := range []string{"command", "file_path", "filePath", "path", "pattern", "skill", "url", "query"} {
		if s, ok := in[k].(string); ok {
			line, _, _ := strings.Cut(s, "\n")
			return line
		}
	}
	return ""
}

// ContentText is a tool result's content: a string, or the text of its blocks.
func ContentText(content json.RawMessage) string {
	var text string
	if json.Unmarshal(content, &text) == nil {
		return text
	}
	var blocks []struct{ Text string }
	_ = json.Unmarshal(content, &blocks)
	for _, b := range blocks {
		text += b.Text
	}
	return text
}

// Block is a block of a claude or pi message's content.
type Block struct {
	Type, Name, Text, Thinking, ID string
	ToolUseID                      string          `json:"tool_use_id"`
	IsError                        bool            `json:"is_error"`
	Input, Arguments               json.RawMessage // claude's and pi's
	Content                        json.RawMessage // claude's tool_result: a string or text blocks
}

// Blocks is the events of the content of a message; what it says only when the agent's is the message.
func Blocks(content []Block, agents bool) (evs []runscreen.Event) {
	for _, b := range content {
		switch b.Type {
		case "text":
			if agents { // not a prompt's text or a tool result
				evs = append(evs, Says(b.Text)...)
			}
		case "thinking":
			evs = append(evs, Thinks(b.Thinking)...)
		case "tool_use":
			evs = append(evs, Call(b.ID, b.Name, Detail(b.Input), runscreen.Running))
		case "toolCall": // pi's, which its tool_execution_start may never follow
			evs = append(evs, Call(b.ID, b.Name, Detail(b.Arguments), runscreen.Running))
		case "tool_result":
			evs = append(evs, Summed(Call(b.ToolUseID, "", "", Result(b.IsError)), ContentText(b.Content)))
		}
	}
	return evs
}
