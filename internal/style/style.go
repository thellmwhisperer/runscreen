// Package style is what the screen and its Markdown share: the colours of its roles and text made safe for the
// terminal.
package style

import (
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
)

// The colours of the screen's roles, ANSI only, so they follow the terminal's theme.
const (
	Red, Green, Yellow, Blue, Cyan, Dim = "1", "2", "3", "4", "6", "8"
)

func init() { lipgloss.SetColorProfile(termenv.ANSI) }

// Paint is s in colour.
func Paint(color, s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Render(s)
}

// Bold is s in bold and colour.
func Bold(color, s string) string {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).Render(s)
}

// DisplayText is value without control characters, its tabs expanded to the next multiple of 8 columns.
func DisplayText(value string) string {
	var out strings.Builder
	column := 0
	for _, r := range value {
		if r == '\t' {
			spaces := 8 - column%8
			out.WriteString(strings.Repeat(" ", spaces))
			column += spaces
		} else if !unicode.IsControl(r) {
			out.WriteRune(r)
			column += ansi.StringWidth(string(r))
		}
	}
	return out.String()
}
