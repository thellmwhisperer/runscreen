// Package codex reads the stream of codex exec --json into runscreen's events.
package codex

import (
	"encoding/json"

	"github.com/thellmwhisperer/runscreen"
	"github.com/thellmwhisperer/runscreen/adapter/internal/tool"
)

type event struct {
	Type string
	Item struct {
		ID, Type, Command, Text, Status string
		ExitCode                        *int   `json:"exit_code"`
		AggregatedOutput                string `json:"aggregated_output"`
	}
}

// Read is the events of line, one JSON line of the stream: the agent's messages and reasoning, and its commands,
// running once started, then passed or failed with the first line of their output. A line it does not know has
// none.
func Read(line []byte) []runscreen.Event {
	var e event
	_ = json.Unmarshal(line, &e)
	switch {
	case e.Item.Type == "reasoning" && e.Type == "item.completed":
		return tool.Thinks(e.Item.Text)
	case e.Item.Type == "agent_message" && e.Type == "item.completed":
		return tool.Says(e.Item.Text)
	case e.Item.Type != "command_execution":
		return nil
	case e.Type == "item.started":
		return []runscreen.Event{tool.Call(e.Item.ID, "shell", e.Item.Command, runscreen.Running)}
	}
	failed := e.Item.Status == "failed" || e.Item.ExitCode != nil && *e.Item.ExitCode != 0
	return []runscreen.Event{tool.Summed(tool.Call(e.Item.ID, "shell", e.Item.Command, tool.Result(failed)),
		e.Item.AggregatedOutput)}
}
