package tool

import (
	"encoding/json"
	"testing"

	"github.com/thellmwhisperer/runscreen"
)

func TestDetailIsTheFirstLineOfTheInputsSubject(t *testing.T) {
	for input, want := range map[string]string{
		`{"command":"go test\n./..."}`: "go test", `{"file_path":"a.go"}`: "a.go", `{"filePath":"b.go"}`: "b.go",
		`{"path":"c"}`: "c", `{"pattern":"*.go"}`: "*.go", `{"skill":"review"}`: "review",
		`{"url":"https://example.org"}`: "https://example.org", `{"query":"q"}`: "q", `{"other":"x"}`: "", `[]`: "",
	} {
		if got := Detail(json.RawMessage(input)); got != want {
			t.Fatalf("Detail(%s) = %q, want %q", input, got, want)
		}
	}
}

func TestContentTextIsAStringOrItsBlocksText(t *testing.T) {
	for content, want := range map[string]string{`"plain"`: "plain", `[{"type":"text","text":"a"},{"text":"b"}]`: "ab",
		`3`: ""} {
		if got := ContentText(json.RawMessage(content)); got != want {
			t.Fatalf("ContentText(%s) = %q, want %q", content, got, want)
		}
	}
}

func TestSummedIsTheFirstLineOfTheOutput(t *testing.T) {
	e := Summed(Call("1", "bash", "ls", Result(true)), "  first  \nsecond")
	if e.Summary != "first" || e.Result != runscreen.Failed || Result(false) != runscreen.Passed {
		t.Fatalf("summed call: %+v", e)
	}
}

func TestBlankSaysAndThinksAreNothing(t *testing.T) {
	if Says(" \n") != nil || Thinks(" ") != nil || len(Says(" hi ")) != 1 || Says(" hi ")[0].Text != "hi" ||
		Thinks("hm\n")[0].Text != "hm\n" {
		t.Fatal("blank text made an event, or text lost its words")
	}
}
