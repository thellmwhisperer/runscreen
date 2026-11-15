package codex

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/thellmwhisperer/runscreen"
)

// recorded is what Read makes of each line of the recording: an event a line, as line has it.
func recorded(t *testing.T) []string {
	t.Helper()
	b, err := os.ReadFile("../../testdata/recordings/codex.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, l := range bytes.Split(b, []byte("\n")) {
		for _, e := range Read(l) {
			out = append(out, line(e))
		}
	}
	return out
}

// line is e on one line: what it says or thinks, its first 40 bytes, or its tool call.
func line(e runscreen.Event) string {
	if e.Kind != runscreen.ToolCall {
		text := strings.ReplaceAll(e.Text, "\n", `\n`)
		return fmt.Sprintf("%d %s", e.Kind, text[:min(40, len(text))])
	}
	return fmt.Sprintf("tool %s %q %q %s %q", e.ID, e.Tool, e.Detail, e.Result, e.Summary)
}

func TestReadTurnsTheRecordedStreamIntoEvents(t *testing.T) {
	cat, rg := "/bin/bash -lc 'cat doc.go .golangci.yml'", "/bin/bash -lc 'pwd; rg --files -g AGENTS.md -g go.mod'"
	want := []string{
		"0 I’ll read both files and run `go vet .",
		fmt.Sprintf(`tool item_1 "shell" %q running ""`, cat),
		fmt.Sprintf(`tool item_1 "shell" %q passed "// Package runscreen draws the run screen of an agent pipeline: a`+
			` pipeline box of steps on the left and the"`, cat),
		fmt.Sprintf(`tool item_2 "shell" %q running ""`, rg),
		fmt.Sprintf(`tool item_2 "shell" %q passed "/work/runscreen"`, rg),
		"0 - `go vet ./...` could not run: the read",
	}
	if got := recorded(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("events:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// The recording carries no reasoning and no failed command: these lines do.
func TestReadReasoningAndFailedCommands(t *testing.T) {
	var got []runscreen.Event
	for _, l := range []string{
		`{"type":"item.completed","item":{"id":"r","type":"reasoning","text":"**Checking** the tests"}}`,
		`{"type":"item.started","item":{"id":"r2","type":"reasoning","text":"partial"}}`,
		`{"type":"item.completed","item":{"id":"c","type":"command_execution","command":"false",` +
			`"aggregated_output":"","exit_code":1,"status":"completed"}}`,
		`{"type":"item.completed","item":{"id":"d","type":"command_execution","command":"go test",` +
			`"aggregated_output":"killed\n","exit_code":null,"status":"failed"}}`,
		`{"type":"item.completed","item":{"id":"f","type":"file_change"}}`,
	} {
		got = append(got, Read([]byte(l))...)
	}
	want := []runscreen.Event{{Kind: runscreen.Thinks, Text: "**Checking** the tests"},
		{Kind: runscreen.ToolCall, ID: "c", Tool: "shell", Detail: "false", Result: runscreen.Failed},
		{Kind: runscreen.ToolCall, ID: "d", Tool: "shell", Detail: "go test", Result: runscreen.Failed, Summary: "killed"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events:\n%#v\nwant:\n%#v", got, want)
	}
}
