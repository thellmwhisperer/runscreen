package markdown

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestInlineRendersALineOfMarkdown(t *testing.T) {
	for _, c := range []struct {
		line, text, raw string
		code, block     bool
	}{
		{line: "**bold** and _it_", text: "bold and it", raw: "\x1b[1mbold"},
		{line: "`x\ty` here", text: "x       y here", raw: "\x1b[33mx", code: true},
		{line: "    indented\tcode", text: "    indented    code", code: true, block: true},
		{line: "<https://example.org> <b>raw</b>", text: "https://example.org <b>raw</b>"},
		{line: "<div>block</div>", text: "<div>block</div>"},
		{line: "safe&#x1b;[2Jtext\x07", text: "safe[2Jtext"},
	} {
		text, code, block := Inline(c.line)
		if ansi.Strip(text) != c.text || code != c.code || block != c.block || !strings.Contains(text, c.raw) {
			t.Fatalf("Inline(%q) = %q, %v, %v; want %q, %v, %v with %q", c.line, text, code, block, c.text, c.code,
				c.block, c.raw)
		}
		if strings.ContainsAny(ansi.Strip(text), "\x1b\x07\t") {
			t.Fatalf("Inline(%q) passes a control through: %q", c.line, text)
		}
	}
}
