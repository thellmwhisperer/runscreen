// Package markdown renders a line of Markdown for runscreen with goldmark: bold, italic, inline code and code
// blocks. A caller that does not import it does not pull goldmark.
package markdown

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/yuin/goldmark/v2/ast"
	"github.com/yuin/goldmark/v2/parser"

	"github.com/thellmwhisperer/runscreen/internal/style"
)

// Inline is line's Markdown rendered for the terminal; code says it holds code, whose spaces wrapping keeps, and
// block that it is a code block, kept as written.
func Inline(line string) (text string, code, block bool) {
	source := []byte(line)
	root := parser.New().Parse(source)
	for b := root.FirstChild(); b != nil; b = b.NextSibling() {
		if _, ok := b.(*ast.CodeBlock); ok {
			return style.DisplayText(line), true, true
		}
	}
	text, code = render(source, root)
	return text, code, false
}

// render is the text of root, parsed from source, with its emphasis, its code in yellow and its HTML as written.
func render(source []byte, root ast.Node) (string, bool) {
	var out strings.Builder
	code := false
	var walk func(ast.Node, lipgloss.Style)
	walk = func(node ast.Node, s lipgloss.Style) {
		switch n := node.(type) {
		case *ast.Emphasis:
			s = s.Italic(true)
		case *ast.Strong:
			s = s.Bold(true)
		case *ast.AutoLink:
			out.WriteString(s.Render(style.DisplayText(string(n.Label.Value(source)))))
			return
		case *ast.CodeBlock:
			code = true
			out.WriteString(style.Paint(style.Yellow, style.DisplayText(strings.TrimSuffix(n.Value.Str(source), "\n"))))
			return
		case *ast.HTMLBlock:
			out.WriteString(style.DisplayText(strings.TrimSuffix(n.Value.Str(source), "\n")))
			return
		case *ast.Text:
			out.WriteString(s.Render(style.DisplayText(string(n.Value.Value(source)))))
			return
		case *ast.CodeSpan:
			code = true
			out.WriteString(style.Paint(style.Yellow, style.DisplayText(codeSpan(source, n))))
			return
		case *ast.RawHTML:
			out.WriteString(style.DisplayText(string(n.Value.Value(source))))
			return
		}
		for child := node.FirstChild(); child != nil; child = child.NextSibling() {
			walk(child, s)
		}
	}
	walk(root, lipgloss.NewStyle())
	return out.String(), code
}

// codeSpan is n's code as written, its spaces kept: goldmark's value of a code span trims them.
func codeSpan(source []byte, n *ast.CodeSpan) string {
	var value strings.Builder
	for _, index := range n.Value.Indices() {
		value.Write(source[index.Start:index.Stop])
	}
	return value.String()
}
