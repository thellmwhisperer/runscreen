package pi

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/thellmwhisperer/runscreen"
)

// recording is the recording's lines.
func recording(t *testing.T) [][]byte {
	t.Helper()
	b, err := os.ReadFile("../../testdata/recordings/pi.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	return bytes.Split(b, []byte("\n"))
}

// line is e on one line: what it says or thinks, its first 40 bytes, or its tool call.
func line(e runscreen.Event) string {
	if e.Kind != runscreen.ToolCall {
		text := strings.ReplaceAll(e.Text, "\n", `\n`)
		return fmt.Sprintf("%d %s", e.Kind, text[:min(40, len(text))])
	}
	id, _, _ := strings.Cut(e.ID, "|")
	return fmt.Sprintf("tool %s %q %q %s %q", id[:min(12, len(id))], e.Tool, e.Detail, e.Result, e.Summary)
}

func TestReadTurnsTheRecordedStreamIntoEvents(t *testing.T) {
	var got []string
	for _, l := range recording(t) {
		for _, e := range Read(l) {
			got = append(got, line(e))
		}
	}
	// pi names each call in its message, then as its execution starts, then as it ends.
	want := []string{
		`tool call_647b028 "read" "doc.go" running ""`,
		`tool call_f2d523b "read" ".golangci.yml" running ""`,
		`tool call_be61394 "bash" "go vet ./..." running ""`,
		`tool call_647b028 "read" "doc.go" running ""`,
		`tool call_f2d523b "read" ".golangci.yml" running ""`,
		`tool call_be61394 "bash" "go vet ./..." running ""`,
		`tool call_647b028 "read" "" passed "// Package runscreen draws the run screen of an agent pipeline: a pipeline` +
			` box of steps on the left and the"`,
		`tool call_f2d523b "read" "" passed "version: \"2\""`,
		`tool call_be61394 "bash" "" passed "(no output)"`,
		"0 - `go vet ./...` passed with no diagnost",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestModelIsTheAssistantMessagesProviderAndModel(t *testing.T) {
	var models []string
	for _, l := range recording(t) {
		if m := Model(l); m != "" {
			models = append(models, m)
		}
	}
	if !reflect.DeepEqual(models, []string{"openai-codex/gpt-6.1-sol", "openai-codex/gpt-6.1-sol"}) {
		t.Fatalf("models: %q", models)
	}
	only := `{"type":"message_end","message":{"role":"assistant","model":"m","content":[]}}`
	if got := Model([]byte(only)); got != "m" {
		t.Fatalf("a model without its provider: %q", got)
	}
}

// The recording carries no thinking and no failed call: these lines do.
func TestReadThinkingAndAFailedCall(t *testing.T) {
	var got []runscreen.Event
	for _, l := range []string{
		`{"type":"message_end","message":{"role":"assistant","content":[{"type":"thinking","thinking":"Plan it."},` +
			`{"type":"text","text":"  "}]}}`,
		`{"type":"tool_execution_end","toolCallId":"c","toolName":"bash","isError":true,` +
			`"result":{"content":[{"type":"text","text":"exit 1"}]}}`,
		`{"type":"tool_execution_end","toolCallId":"d","toolName":"bash","result":{"isError":true,"content":"no"}}`,
	} {
		got = append(got, Read([]byte(l))...)
	}
	want := []runscreen.Event{{Kind: runscreen.Thinks, Text: "Plan it."},
		{Kind: runscreen.ToolCall, ID: "c", Tool: "bash", Result: runscreen.Failed, Summary: "exit 1"},
		{Kind: runscreen.ToolCall, ID: "d", Tool: "bash", Result: runscreen.Failed, Summary: "no"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events:\n%#v\nwant:\n%#v", got, want)
	}
}
