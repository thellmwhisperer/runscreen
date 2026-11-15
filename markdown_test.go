package runscreen

import (
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/thellmwhisperer/runscreen/markdown"
)

// agentBoxSays is the Agent box, width wide, once the agent says text.
func agentBoxSays(t *testing.T, width int, text string) (string, []string) {
	t.Helper()
	m := agentScreen(width, says(text))
	return m.agentBox(width, 40), agentLines(m, width)
}

func requireAgentBoxContains(t *testing.T, width int, text, want string) string {
	t.Helper()
	raw, _ := agentBoxSays(t, width, text)
	if stripped := ansi.Strip(raw); !strings.Contains(stripped, want) {
		t.Fatalf("rendered line is missing %q: %q", want, stripped)
	}
	return raw
}

func activeStylesAtText(raw, text string) (bool, bool, bool) {
	bold, italic := false, false
	offset := 0
	for _, sgr := range regexp.MustCompile(`\x1b\[([0-9;]*)m`).FindAllStringSubmatchIndex(raw, -1) {
		if strings.Contains(raw[offset:sgr[0]], text) {
			return bold, italic, true
		}
		for _, code := range strings.Split(raw[sgr[2]:sgr[3]], ";") {
			switch code {
			case "0":
				bold, italic = false, false
			case "1":
				bold = true
			case "3":
				italic = true
			case "22":
				bold = false
			case "23":
				italic = false
			}
		}
		offset = sgr[1]
	}
	if strings.Contains(raw[offset:], text) {
		return bold, italic, true
	}
	return false, false, false
}

func TestAgentBoxRendersMarkdownBold(t *testing.T) {
	raw, lines := agentBoxSays(t, 60, "**Evaluating helper fallback**")
	if want := []string{"✎ Evaluating helper fallback"}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("Agent box lines:\n%q\nwant:\n%q", lines, want)
	}
	if !strings.Contains(raw, "\x1b[1mEvaluating") {
		t.Fatalf("bold text is not bold:\n%q", raw)
	}
}

func TestAgentBoxRendersUnderscoreAndVariableBacktickMarkdown(t *testing.T) {
	raw, lines := agentBoxSays(t, 100, "__Bold__ and _emphasis_ with ``a `tick` and  double spaces``.")
	if want := []string{"✎ Bold and emphasis with a `tick` and  double spaces."}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("Agent box lines:\n%q\nwant:\n%q", lines, want)
	}
	if !strings.Contains(raw, "\x1b[1mBold") || !strings.Contains(raw, "\x1b[3memphasis") {
		t.Fatalf("Markdown styles are missing:\n%q", raw)
	}
}

func TestAgentBoxRendersNestedAndTripleEmphasis(t *testing.T) {
	for _, text := range []string{"**bold _italic_**", "- ***bold italic***"} {
		raw, lines := agentBoxSays(t, 60, text)
		want := "✎ bold italic"
		styleWants := map[string][2]bool{"bold": {true, false}, "italic": {true, true}}
		if strings.HasPrefix(text, "-") {
			want = "✎ • bold italic"
			styleWants = map[string][2]bool{"bold": {true, true}, "italic": {true, true}}
		}
		if !reflect.DeepEqual(lines, []string{want}) {
			t.Fatalf("Agent box lines for %q:\n%q\nwant:\n%q", text, lines, []string{want})
		}
		for word, styles := range styleWants {
			bold, italic, found := activeStylesAtText(raw, word)
			if !found || bold != styles[0] || italic != styles[1] {
				t.Fatalf("wrong active styles for %q in %q: bold=%t italic=%t found=%t\n%q", word, text, bold, italic,
					found, raw)
			}
		}
	}
}

func TestAgentBoxKeepsFencedMarkdownLiteralAndWrapsAtPanelWidth(t *testing.T) {
	const width = 32
	raw, lines := agentBoxSays(t, width, "```\n**literal**\n- x\n\t123456789012345678901234567890123456\n```")
	want := []string{"✎ **literal**", "- x", "123456789012345678", "901234567890123456"}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("fenced code changed:\n%q\nwant:\n%q", lines, want)
	}
	for _, line := range strings.Split(raw, "\n") {
		if got := ansi.StringWidth(line); got != width {
			t.Fatalf("rendered line is %d columns, want %d: %q", got, width, ansi.Strip(line))
		}
	}
	if strings.Contains(raw, "\t") {
		t.Fatalf("tab reached terminal output: %q", raw)
	}
}

func TestAgentBoxRendersAutoLinksAndHTMLSourceInListsAndText(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{name: "autolink", text: "<https://example.org>", want: "https://example.org"},
		{name: "autolink in list", text: "- <https://example.org>", want: "• https://example.org"},
		{name: "HTML block", text: "<div>text</div>", want: "<div>text</div>"},
		{name: "HTML in list", text: "- <div>text</div>", want: "• <div>text</div>"},
	} {
		t.Run(tc.name, func(t *testing.T) { requireAgentBoxContains(t, 60, tc.text, tc.want) })
	}
}

func TestAgentBoxClassifiesIndentedCodeBeforeListMarkers(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{name: "four-space code", text: "    - x", want: "✎     - x"},
		{name: "tab-indented code", text: "\t- x", want: "✎         - x"},
		{name: "valid one-space list", text: " - x", want: "✎  • x"},
		{name: "numbered list", text: "2. second", want: "✎ 2. second"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if raw := requireAgentBoxContains(t, 60, tc.text, tc.want); strings.Contains(raw, "\t") {
				t.Fatalf("tab reached terminal output: %q", raw)
			}
		})
	}
}

func TestAgentBoxPreservesInlineCodeWhitespaceWhenWrapping(t *testing.T) {
	for _, text := range []string{
		"`abcdefghijklmnopqrst  abcdefghijklmnop`",
		"- `abcdefghijklmnopqrst  abcdefghijklmnop`",
	} {
		raw, _ := agentBoxSays(t, 32, text)
		var rendered []string
		for _, line := range strings.Split(ansi.Strip(raw), "\n") {
			if !strings.HasPrefix(line, "│ ") {
				continue
			}
			if line = strings.TrimRight(strings.TrimSuffix(strings.TrimPrefix(line, "│ "), " │"), " "); line == "" {
				continue
			}
			if len(rendered) > 0 {
				line = strings.TrimPrefix(line, "  ")
				if strings.HasPrefix(text, "-") {
					line = strings.TrimPrefix(line, "  ")
				}
			} else {
				line = strings.TrimPrefix(strings.TrimPrefix(line, "✎ "), "• ")
			}
			rendered = append(rendered, line)
		}
		if got := strings.Join(rendered, ""); got != "abcdefghijklmnopqrst  abcdefghijklmnop" {
			t.Fatalf("inline code changed while wrapping %q: %q", text, got)
		}
	}
}

func TestAgentBoxPreservesCodeSourceWhitespaceAndExpandsTabs(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{name: "indented block", text: "    x\tz", want: "✎     x   z"},
		{name: "inline code padding", text: "` x `", want: "✎  x "},
		{name: "inline code tab", text: "`x\tz`", want: "x       z"},
	} {
		t.Run(tc.name, func(t *testing.T) { requireAgentBoxContains(t, 60, tc.text, tc.want) })
	}
}

func TestAgentBoxOnlyClosesFenceWithMatchingMarkerAndLength(t *testing.T) {
	raw, lines := agentBoxSays(t, 60, "````go\n~~~\n```\n  kept  \n````\nafter")
	if want := []string{"✎ ~~~", "```", "kept", "after"}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("Agent box lines:\n%q\nwant:\n%q", lines, want)
	}
	if !strings.Contains(ansi.Strip(raw), "  kept  ") {
		t.Fatalf("fenced code whitespace changed:\n%s", ansi.Strip(raw))
	}
}

func TestAgentBoxRendersMarkdownWithinANarrowPanel(t *testing.T) {
	const width = 32
	raw, lines := agentBoxSays(t, width, "Plan for *the* fix:\n- read `view.go` and the tests around it\n"+
		"- fix\n\n```go\nif  x {\n\treturn 1 // a long code line that has to wrap\n}\n```\nDone.")
	want := []string{"✎ Plan for the fix:", "• read view.go and the", "tests around it", "• fix", "if  x {",
		"return 1 // a long", "code line that has to wra", "p", "}", "Done."}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("Agent box lines:\n%q\nwant:\n%q", lines, want)
	}
	for _, l := range strings.Split(raw, "\n") {
		if w := ansi.StringWidth(l); w != width {
			t.Fatalf("line %q is %d wide, want %d", ansi.Strip(l), w, width)
		}
	}
	if stripped := ansi.Strip(raw); !strings.Contains(stripped, "│           return 1") ||
		!strings.Contains(stripped, "│      tests around it") {
		t.Fatalf("indentation not kept:\n%s", stripped)
	}
}

func TestAgentBoxDoesNotPassTerminalControlsThrough(t *testing.T) {
	raw, lines := agentBoxSays(t, 60, "safe\x1b]0;title\a \x1b[2Jtext\r\b")
	if want := []string{"✎ safe text"}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("Agent box lines:\n%q\nwant:\n%q", lines, want)
	}
	for _, c := range []string{"\x1b]", "\x1b[2J", "\a", "\r", "\b"} {
		if strings.Contains(raw, c) {
			t.Fatalf("Agent box passes %q through:\n%q", c, raw)
		}
	}
}

func TestAgentBoxDoesNotDecodeMarkdownEntitiesIntoTerminalControls(t *testing.T) {
	raw, lines := agentBoxSays(t, 60, "safe&#x1b;[2Jtext")
	if want := []string{"✎ safe[2Jtext"}; !reflect.DeepEqual(lines, want) {
		t.Fatalf("Agent box lines:\n%q\nwant:\n%q", lines, want)
	}
	if strings.Contains(raw, "\x1b[2J") {
		t.Fatalf("Agent box emitted an entity-decoded terminal control:\n%q", raw)
	}
}

func TestWrappingKeepsTheSpacesInlineCodeEndsWith(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"x `abc  `", "x\n abc\n  "},
	} {
		text, code, _ := markdown.Inline(c.text)
		if got := ansi.Strip(strings.Join(plainRows(text, 4, code), "\n")); got != c.want {
			t.Fatalf("%q wrapped at 4: %q, want %q", c.text, got, c.want)
		}
	}
}
