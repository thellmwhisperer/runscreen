//go:build linux || darwin

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thellmwhisperer/runscreen"
)

// stream is a stream the test writes, in name, each line a JSON value of lines: what no recording carries.
func stream(t *testing.T, name string, lines ...any) string {
	t.Helper()
	var b strings.Builder
	for _, l := range lines {
		j, err := json.Marshal(l)
		if err != nil {
			t.Fatal(err)
		}
		b.Write(append(j, '\n'))
	}
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type object = map[string]any

// message is a claude line of type kind whose message, by role, holds blocks.
func message(kind, role string, blocks ...object) object {
	return object{"type": kind, "message": object{"role": role, "content": blocks}}
}

// says is what the agent says in the rich stream: every kind of Markdown line the Agent box draws.
var said = strings.Join([]string{
	"*Italic* and <https://example.org> and <b>raw</b>",
	"<div>block</div>",
	"    indented code",
	">     quoted code",
	"  ```go",
	"fmt.Println(1)",
	"  ```",
	"bell\a here",
	"x `" + strings.Repeat("a", 71) + "  `",
}, "\n")

// rich is a claude stream with a call about nothing it names, a call that never ends
// and Markdown of every kind.
func rich(t *testing.T) string {
	t.Helper()
	return stream(t, "claude-rich.jsonl",
		message("assistant", "assistant", object{"type": "text", "text": "  "}),
		object{"type": "system", "subtype": "init", "model": "claude-rich"},
		message("assistant", "assistant", object{"type": "tool_use", "id": "grep", "name": "Grep",
			"input": object{"pattern": "TODO"}}),
		message("user", "user", object{"type": "tool_result", "tool_use_id": "grep", "content": "a.go: TODO"}),
		message("assistant", "assistant", object{"type": "tool_use", "id": "odd", "name": "Task",
			"input": object{"other": "x"}}),
		message("user", "user", object{"type": "tool_result", "tool_use_id": "odd", "content": "done"}),
		message("assistant", "assistant", object{"type": "text", "text": said}),
		message("assistant", "assistant", object{"type": "tool_use", "id": "slow", "name": "Bash",
			"input": object{"command": "sleep 100"}}),
	)
}

func TestTheDemoDrawsEveryKindOfLine(t *testing.T) {
	path := rich(t)
	d := start(t, 120, 40, "-delay", "1ms", path)
	d.m = replica(t, path, runscreen.Agent{}, 120, 40)
	d.until(0, "Agent · claude · claude-rich", "▸ · 2 tools: Grep, Task",
		"✎ Italic and https://example.org and <b>raw</b>",
		"<div>block</div>", "    indented code", "quoted code", "fmt.Println(1)", "bell here", "1 tool: Bash · sleep 100",
		"   "+strings.Repeat("a", 70))
	d.act("t", runes('t'), "✓ ok Grep TODO · a.go: TODO", "✓ ok Task  · done", "running Bash sleep 100")
	if last := d.quit(); !strings.Contains(last, "✓ Grep TODO") || !strings.Contains(last, "Bash sleep 100") ||
		strings.Contains(last, "\a") {
		t.Fatalf("the last frame:\n%s", last)
	}
	d = start(t, 120, 40, "-delay", "1ms", "-plain", path)
	d.until(0, "✎ *Italic* and <https://example.org> and <b>raw</b>", "`aaaa")
	d.quit()
}

func TestTheDemoReadsCodexReasoningAndPiThinking(t *testing.T) {
	codex := stream(t, "codex-reasoning.jsonl",
		object{"type": "item.completed", "item": object{"id": "r", "type": "reasoning", "text": "Checking the **tests**"}},
		object{"type": "item.started", "item": object{"id": "c", "type": "command_execution", "command": "false"}},
		object{"type": "item.completed", "item": object{"id": "c", "type": "command_execution", "command": "false",
			"aggregated_output": "boom\n", "exit_code": 1, "status": "failed"}})
	d := start(t, 120, 40, "-delay", "1ms", codex)
	d.until(0, "Agent · codex", "▸ ∴ Checking the tests", "✗ failed shell false · boom", "✗ shell false")
	d.quit()
	pi := stream(t, "pi-thinking.jsonl",
		object{"type": "message_end", "message": object{"role": "assistant", "model": "m",
			"content": []object{{"type": "thinking", "thinking": "Pondering."}, {"type": "text", "text": "Said."}}}})
	d = start(t, 120, 40, "-delay", "1ms", pi)
	d.until(0, "Agent · pi · m", "▸ ∴ Pondering.", "✎ Said.")
	d.quit()
}

func TestTheDemoKeepsItsPlaceWhenRowsLeave(t *testing.T) {
	b, err := os.ReadFile("../../testdata/recordings/claude.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	// The recording, under a name long enough to fold the title at 80 columns.
	path := filepath.Join(t.TempDir(), "claude-recorded-on-this-repository-and-replayed-under-a-long-name.jsonl")
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	d := start(t, 80, 24, "-delay", "1ms", path)
	d.m = replica(t, path, runscreen.Agent{}, 80, 24)
	d.until(0, "Line types (4)", "✓ Show the last frame")
	d.act("t", runes('t'), "t hide tool use")
	d.key(home, tea.KeyMsg{Type: tea.KeyHome})
	d.key(down, tea.KeyMsg{Type: tea.KeyDown})
	if top := d.agentTop(); !strings.Contains(top, "ok Read /work/runscreen/doc.go") {
		t.Fatalf("the fixture must read from a call's row, not %q", top)
	}
	d.key("t", runes('t')) // the call's row leaves: its group's row stays on top
	if top := d.agentTop(); !strings.HasPrefix(top, "▸ · 3 tools") {
		t.Fatalf("folded, the box must show the call's group on top, not %q", top)
	}
	resize(t, d.tty, 80, 9)
	d.act("", tea.WindowSizeMsg{Width: 80, Height: 9}, "▸ claude-recorded-on-this-repository", "q quit  t show")
	d.act("i", runes('i'), "▾ claude-recorded-on-this-repository")
	d.act(tab, tea.KeyMsg{Type: tea.KeyTab}, "Pipeline*")
	d.then(home, tea.KeyMsg{Type: tea.KeyHome}, 2) // held on the title's second line
	d.then("i", runes('i'), 2)                     // the title's lines leave: the box keeps its place
	if top := d.row(1); !strings.HasPrefix(top, "▸ claude-recorded-on-this-repository") {
		t.Fatalf("the title folded again: %q", top)
	}
	footer := mouse(64, 2, 8) // the wheel over the key line scrolls nothing
	d.act(footer+"h", runes('h'), "h hide thinking")
	if last := d.quit(); !strings.Contains(last, "Pipeline*") || strings.Contains(last, "Recording") {
		t.Fatalf("the last frame, 9 rows tall, without room for the Recording box:\n%s", last)
	}
}
