package runscreen

import (
	"cmp"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/thellmwhisperer/runscreen/internal/style"
)

const (
	red, green, yellow, blue, cyan, dim = style.Red, style.Green, style.Yellow, style.Blue, style.Cyan, style.Dim
)

var paint, bold = style.Paint, style.Bold

// fold is the arrow that marks a row a click folds or unfolds, underlined like a link: ▸ folded, ▾ unfolded.
func fold(open bool) string {
	return "\x1b[4m" + map[bool]string{true: "▾", false: "▸"}[open] + "\x1b[0m"
}

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// states is each state's icon and colour; its word is its name. Running's icon is the spinner.
var states = map[State][2]string{Pending: {"○", dim}, Running: {"", blue}, Waiting: {"⏸", yellow},
	Passed: {"✓", green}, Failed: {"✗", red}, Skipped: {"–", dim}}

// icon is state's icon in its colour.
func (m *Model) icon(state State) string {
	s := states[state]
	if state == Running {
		s[0] = spinner[m.spin%len(spinner)]
	}
	return paint(s[1], s[0])
}

// Human is d as the screen prints it: tenths of a second under a minute, then minutes and seconds, then hours
// and minutes.
func Human(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%.1f s", d.Truncate(100*time.Millisecond).Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%d min %02d s", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%d h %02d min", int(d.Hours()), int(d.Minutes())%60)
}

// Layout: two columns from 100 columns, the left a third between 38 and 48; connectors between steps from 30
// rows.
const (
	twoColumns, leftMin, leftMax, gap, connectors = 100, 38, 48, 2, 30
)

const (
	boxBorderRows       = 2
	minimumBoxRows      = boxBorderRows + 1
	minimumPipelineRows = minimumBoxRows + 2
)

// View is the screen: the pipeline box, then the Agent box and the caller's boxes under it, beside it from 100
// columns, below it under that, and the key line.
func (m *Model) View() string {
	width := max(m.width, 40)
	available := max(0, m.height-lipgloss.Height(m.footer()))
	m.historyBounds, m.boxBounds = bounds{}, nil
	if width < twoColumns || !m.sidebar() {
		pipelineHeight := available
		if m.sidebar() {
			pipelineHeight = stackedHeight(available, m.rightMinimum())
		}
		pipelineBox := m.pipelineBox(width, pipelineHeight)
		m.pipelineBounds = bounds{width: width, height: lipgloss.Height(pipelineBox)}
		panels := []string{pipelineBox}
		if m.sidebar() {
			panels = append(panels, m.right(width, max(1, available-m.pipelineBounds.height)))
			m.place(0, m.pipelineBounds.height, width)
		}
		return strings.Join(panels, "\n") + m.footer()
	}
	left := min(max(width/3, leftMin), leftMax)
	rightWidth := width - left - gap
	leftBox := m.pipelineBox(left, available)
	m.pipelineBounds = bounds{width: left, height: lipgloss.Height(leftBox)}
	rightBox := m.right(rightWidth, available)
	m.place(left+gap, 0, rightWidth)
	return columns(leftBox, rightBox, left) + m.footer()
}

// place puts the bounds of the Agent box and of the boxes under it at x, y, width wide, as right drew them.
func (m *Model) place(x, y, width int) {
	if m.historyHeight > 0 {
		m.historyBounds = bounds{x: x, y: y, width: width, height: m.historyHeight}
		y += m.historyHeight
	}
	m.boxBounds = make([]bounds, len(m.boxHeights))
	for i, h := range m.boxHeights {
		m.boxBounds[i] = bounds{x: x, y: y, width: width, height: h}
		y += h
	}
}

// stackedHeight is the most rows the pipeline box takes of available above the right column, which keeps a third
// of them, rightMinimum at least, and whatever the pipeline box leaves.
func stackedHeight(available, rightMinimum int) int {
	right := max(rightMinimum, available/3)
	return max(1, max(minimumPipelineRows, available-right))
}

// rightMinimum is the fewest rows the right column takes below the pipeline box when stacked.
func (m *Model) rightMinimum() int {
	n := len(m.boxes)
	if m.agentVisible() {
		n++
	}
	return n * minimumBoxRows
}

func (m *Model) footer() string {
	gap := "\n\n  "
	if m.height < 10 {
		gap = "\n  "
	}
	key := func(k, what string) string { return lipgloss.NewStyle().Bold(true).Render(k) + " " + paint(dim, what) }
	keys := key("q", "quit")
	if m.agentVisible() {
		show := map[bool]string{true: "hide", false: "show"}
		keys += "  " + key("t", show[m.unfold]+" tool use") + "  " + key("h", show[m.unfoldThink]+" thinking")
	}
	return gap + keys
}

// columns is left, every line padded to width, beside right.
func columns(left, right string, width int) string {
	l, r := strings.Split(left, "\n"), strings.Split(right, "\n")
	lines := make([]string, max(len(l), len(r)))
	for i := range lines {
		if i < len(l) {
			lines[i] = l[i] + strings.Repeat(" ", max(0, width-lipgloss.Width(l[i])))
		} else {
			lines[i] = strings.Repeat(" ", width)
		}
		if i < len(r) {
			lines[i] += strings.Repeat(" ", gap) + r[i]
		}
	}
	return strings.Join(lines, "\n")
}

// box is content in a rounded box of width with title in its top border, its border in colour border.
func box(title, content string, width int, border string) string {
	inner := width - 4
	fill := strings.Repeat("─", max(1, width-5-lipgloss.Width(title)))
	lines := []string{paint(border, "╭─ ") + title + " " + paint(border, fill+"╮")}
	for _, l := range strings.Split(content, "\n") {
		l = ansi.Truncate(l, inner, "…")
		side := paint(border, "│")
		lines = append(lines, side+" "+l+strings.Repeat(" ", max(0, inner-lipgloss.Width(l)))+" "+side)
	}
	return strings.Join(append(lines, paint(border, "╰"+strings.Repeat("─", width-2)+"╯")), "\n")
}

// border is the colour of the border of the box of pane p: lit when keys scroll it.
func (m *Model) border(p *pane) string {
	return map[bool]string{true: cyan, false: dim}[m.focused() == p]
}

// boxTitle is title in bold, starred while keys scroll p.
func (m *Model) boxTitle(title string, p *pane, width int) string {
	if m.focused() == p {
		title += "*"
	}
	return bold(cyan, ansi.Truncate(title, max(1, width-6), "…"))
}

// spread is left and, right-aligned in width, right, which gives way first when both do not fit.
func spread(left, right string, width int) string {
	room := width - lipgloss.Width(left) - 1
	if right = ansi.Truncate(right, max(0, room), "…"); right == "" {
		return left
	}
	return left + strings.Repeat(" ", max(1, room+1-lipgloss.Width(right))) + right
}

// pipelineBox is the pipeline box, height rows at most: the title and the pipeline's state, then a row for each
// step and sub-step, a connector between steps when the terminal is tall enough, the rows from its reading
// position.
func (m *Model) pipelineBox(width, height int) string {
	inner := width - 4
	head := m.head(inner)
	// The title stays above the rows when it leaves them a line; else only its first line stays and the rest
	// scroll above the rows, so a long title never makes the box taller. With neither title nor state, the rows
	// start at the top.
	lines, rows := append(head, ""), []paneLine(nil)
	if m.pipelineData.Title == "" && m.pipelineData.State == "" {
		lines = nil
	} else if len(lines)+1 > height-boxBorderRows {
		lines = lines[:1]
		for i, l := range append(head[1:], "") {
			rows = append(rows, paneLine{key: fmt.Sprintf("title:%d", i), text: l})
		}
	}
	rows = m.stepRows(rows, m.pipelineData.Steps, "", inner, 0)
	rows = m.compactPipeline(rows, height-2-len(lines))
	lines = append(lines, m.pipeline.show(rows, max(1, height-2-len(lines)))...)
	return box(m.boxTitle("Pipeline", &m.pipeline, width), strings.Join(lines, "\n"), width, m.border(&m.pipeline))
}

// head is the pipeline box's first lines in inner columns: its title and, right-aligned, its state. A title too
// long for its line beside the state folds behind the arrow, cut with an ellipsis, until i or a click on it
// splits it in whole lines.
func (m *Model) head(inner int) []string {
	title, state := m.pipelineData.Title, ""
	if s := m.pipelineData.State; s != "" {
		state = m.icon(s) + " " + bold(states[s][1], string(s))
	}
	room := max(1, inner-lipgloss.Width(state)-1)
	head := []string{paint(dim, title)}
	m.titleArrow = bounds{}
	if ansi.StringWidth(title) > room {
		m.titleArrow = bounds{x: 2, y: 1, width: 1, height: 1}
		head = []string{ansi.Truncate(title, max(1, room-2), "…")}
		if m.unfoldTitle {
			head = strings.Split(ansi.Wrap(title, max(1, room-2), ""), "\n")
		}
		for i, l := range head {
			head[i] = "  " + paint(dim, l)
		}
		head[0] = fold(m.unfoldTitle) + head[0][1:]
	}
	head[0] = spread(head[0], state, inner)
	return head
}

// stepRows is rows and a row for each of steps and, under it, its sub-steps, depth levels in; path is the key of
// their parent.
func (m *Model) stepRows(rows []paneLine, steps []Step, path string, inner, depth int) []paneLine {
	for i, s := range steps {
		key := "step:" + path + strconv.Itoa(i)
		if depth == 0 && i > 0 && m.height >= connectors {
			rows = append(rows, paneLine{key: "connector:" + key, parent: key, text: paint(dim, "│")})
		}
		rows = append(rows, paneLine{key: key, text: m.row(s, depth, inner)})
		rows = m.stepRows(rows, s.Steps, path+strconv.Itoa(i)+"/", inner, depth+1)
	}
	return rows
}

// compactPipeline gives an ended pipeline's steps room before spending rows on connectors.
func (m *Model) compactPipeline(rows []paneLine, height int) []paneLine {
	if !running(m.pipelineData.Steps) && len(rows) > height {
		return slices.DeleteFunc(rows, func(r paneLine) bool { return strings.HasPrefix(r.key, "connector:") })
	}
	return rows
}

// running says whether a step of steps, or of their sub-steps, runs.
func running(steps []Step) bool {
	return slices.ContainsFunc(steps, func(s Step) bool { return s.State == Running || running(s.Steps) })
}

// took is how long s took, or, while it runs, has taken by now; false when it has no time to show.
func took(s Step, now time.Time) (time.Duration, bool) {
	switch {
	case s.Start.IsZero():
		return 0, false
	case s.State == Running:
		return now.Sub(s.Start), true
	case !s.End.IsZero():
		return s.End.Sub(s.Start), true
	}
	return 0, false
}

// row is s's line, depth levels in: its icon, its name and how long it took, then, right-aligned, its note and
// its state.
func (m *Model) row(s Step, depth, width int) string {
	prefix := strings.Repeat("  ", depth) + m.icon(s.State) + " "
	suffix := ""
	if d, ok := took(s, m.now); ok {
		suffix = "  " + paint(dim, Human(d))
	}
	status := bold(states[s.State][1], string(s.State))
	if s.Note != "" {
		// The note gives way before the name: it gets only what the name, the duration and the state leave.
		room := width - lipgloss.Width(prefix) - lipgloss.Width(s.Name) - lipgloss.Width(suffix) -
			lipgloss.Width(status) - 4
		if room > 0 {
			status = paint(dim, ansi.Truncate(oneLine(s.Note), room, "…")+" · ") + status
		}
	}
	nameWidth := max(0, width-lipgloss.Width(prefix)-lipgloss.Width(suffix)-lipgloss.Width(status)-1)
	left := prefix + ansi.Truncate(oneLine(s.Name), nameWidth, "…") + suffix
	left = ansi.Truncate(left, max(0, width-lipgloss.Width(status)-1), "…")
	return spread(left, status, width)
}

// sidebar says whether there is a box beside the pipeline box to show.
func (m *Model) sidebar() bool { return m.agentVisible() || len(m.boxes) > 0 }

// agentVisible says whether the Agent box shows: once the caller named the agent or gave it an event.
func (m *Model) agentVisible() bool { return m.agentInfo != nil || len(m.agent) > 0 }

// right is, at width in height rows, the Agent box and the caller's boxes under it. Each of the caller's boxes
// takes the rows it needs, a share of the column at most, a third while the Agent box shows, which takes the rest.
func (m *Model) right(width, height int) string {
	m.historyHeight = 0
	heights := make([]int, len(m.boxes))
	defer func() { m.boxHeights = heights }() // the boxes shown, once drawn: tab goes on reading the last frame's
	remaining, used := height, 0
	if m.agentVisible() {
		remaining -= minimumBoxRows
	}
	var boxes []string
	for i := range m.boxes {
		share := remaining / (len(m.boxes) - i)
		if remaining >= minimumBoxRows {
			share = max(share, minimumBoxRows) // the first boxes first, when not every one has room
		}
		if m.agentVisible() {
			share = min(share, max(minimumBoxRows, height/3))
		}
		if share < minimumBoxRows {
			continue // no room: the box stays off the screen until the terminal grows
		}
		b := m.extraBox(i, width, share-boxBorderRows)
		heights[i] = lipgloss.Height(b)
		remaining, used = remaining-heights[i], used+heights[i]
		boxes = append(boxes, b)
	}
	if m.agentVisible() {
		boxes = append([]string{m.agentBox(width, max(1, height-used-boxBorderRows))}, boxes...)
	}
	return strings.Join(boxes, "\n")
}

// extraBox is the caller's box i, its rows from its reading position, lines rows at most.
func (m *Model) extraBox(i, width, lines int) string {
	b, p := m.boxes[i], m.boxPanes[i]
	rows := m.boxRows(nil, b.Title, b.Rows, "", "", 0, width)
	shown := p.show(rows, min(len(rows), lines))
	return box(m.boxTitle(b.Title, p, width), strings.Join(shown, "\n"), width, m.border(p))
}

// boxRows is out and the lines of rows, depth levels in, under the row by key parent: each row's Markdown, behind
// the arrow a click folds or unfolds when it has rows of its own, and those rows under it once unfolded.
func (m *Model) boxRows(out []paneLine, title string, rows []Row, path, parent string, depth, width int) []paneLine {
	if m.rowOpen == nil {
		m.rowOpen = map[string]bool{}
	}
	indent := strings.Repeat("  ", depth)
	for i, r := range rows {
		key := "box:" + title + ":" + path + strconv.Itoa(i)
		marker, open := "  ", false
		if len(r.Rows) > 0 {
			m.rowOpen[key] = r.Open
			open = m.unfolded(key)
			marker = fold(open) + " "
		}
		texts, _, _ := m.textRows(indent+marker, indent+"  ", r.Text, width, codeFence{})
		for j, text := range texts {
			line := paneLine{key: key, parent: parent, text: text}
			if j == 0 && len(r.Rows) > 0 {
				line.toggle = key
			}
			out = append(out, line)
		}
		if open {
			out = m.boxRows(out, title, r.Rows, path+strconv.Itoa(i)+"/", key, depth+1, width)
		}
	}
	return out
}

var listItem = regexp.MustCompile(`^( {0,3})([-*+]|\d+[.)])[ \t]+(.*)$`)

type codeFence struct {
	marker byte
	length int
}

func fence(line string) (byte, int, string, bool) {
	i := 0
	for i < len(line) && line[i] == ' ' && i < 4 {
		i++
	}
	if i > 3 || i == len(line) || line[i] != '`' && line[i] != '~' {
		return 0, 0, "", false
	}
	marker := line[i]
	start := i
	for i < len(line) && line[i] == marker {
		i++
	}
	if i-start < 3 {
		return 0, 0, "", false
	}
	return marker, i - start, strings.TrimSpace(line[i:]), true
}

func wrapMarkdown(text string, width int) string {
	lines := strings.Split(ansi.Wrap(text, width, ""), "\n")
	plain := ansi.Strip(text)
	offset := 0
	var wrapped []string
	for _, line := range lines {
		visible := ansi.Strip(line)
		start := strings.Index(plain[offset:], visible)
		if start < 0 {
			return ansi.Hardwrap(text, width, true)
		}
		line = plain[offset:offset+start] + line
		offset += start + len(visible)
		if ansi.StringWidth(line) > width {
			wrapped = append(wrapped, strings.Split(ansi.Hardwrap(line, width, true), "\n")...)
		} else {
			wrapped = append(wrapped, line)
		}
	}
	if offset < len(plain) {
		wrapped[len(wrapped)-1] += plain[offset:]
		if ansi.StringWidth(wrapped[len(wrapped)-1]) > width {
			last := strings.Split(ansi.Hardwrap(wrapped[len(wrapped)-1], width, true), "\n")
			wrapped = append(wrapped[:len(wrapped)-1], last...)
		}
	}
	return strings.Join(wrapped, "\n")
}

func plainRows(text string, width int, code bool) []string {
	if code {
		text = wrapMarkdown(text, width)
	} else {
		text = ansi.Wrap(text, width, "")
	}
	return strings.Split(text, "\n")
}

func (m *Model) listRows(line string, width int) ([]string, bool) {
	item := listItem.FindStringSubmatch(line)
	if item == nil {
		return nil, false
	}
	lead := item[1] + cmp.Or(strings.Trim(item[2], "-*+"), "•") + " "
	bodyText, bodyHasCode, _ := m.inline(item[3])
	bodyWidth := max(1, width-ansi.StringWidth(lead))
	body := plainRows(bodyText, bodyWidth, bodyHasCode)
	wrapped := append([]string{lead + body[0]}, body[1:]...)
	for i := 1; i < len(wrapped); i++ {
		wrapped[i] = strings.Repeat(" ", ansi.StringWidth(lead)) + wrapped[i]
	}
	return wrapped, true
}

// textRows is the Markdown of text in rows of a box width wide, the first behind prefix, the rest behind indent:
// fenced code kept as is, hard wrapped, lists with their continuation under their text. Terminal controls are
// stripped and tabs are expanded before they reach the terminal.
func (m *Model) textRows(prefix, indent, text string, width int, code codeFence) ([]string, []int, codeFence) {
	text = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, ansi.Strip(text))
	w := max(1, width-4-max(ansi.StringWidth(prefix), ansi.StringWidth(indent)))
	var rows []string
	var sources []int
	for source, line := range strings.Split(text, "\n") {
		var wrapped []string
		wrapped, code = m.lineRows(line, w, code)
		for _, l := range wrapped {
			sources = append(sources, source)
			if rows == nil {
				rows = append(rows, prefix+l)
			} else {
				rows = append(rows, indent+l)
			}
		}
	}
	return rows, sources, code
}

// lineRows is line in rows of width, inside the fenced code code, or after it; and the fence it leaves open.
func (m *Model) lineRows(line string, width int, code codeFence) ([]string, codeFence) {
	marker, length, info, isFence := fence(line)
	if code.length > 0 {
		if isFence && marker == code.marker && length >= code.length && info == "" {
			return nil, codeFence{}
		}
		return strings.Split(paint(yellow, ansi.Hardwrap(style.DisplayText(line), width, true)), "\n"), code
	}
	if isFence {
		return nil, codeFence{marker, length}
	}
	lineText, hasCode, isBlock := m.inline(line)
	if !isBlock {
		if list, ok := m.listRows(line, width); ok {
			return list, code
		}
	}
	return plainRows(lineText, width, hasCode), code
}

// agentBox shows what the agent says and thinks and its tool calls, ending with the latest activity. What it says
// wraps in full; what it thinks and the tool calls fold to one row until opened.
func (m *Model) agentBox(width, lines int) string {
	var out []paneLine
	var calls []*activity
	for _, a := range m.agent {
		if a.says == "" && a.thinking == "" {
			calls = append(calls, a)
			continue
		}
		out, calls = m.toolLines(out, calls), nil
		key := fmt.Sprintf("activity:%p", a)
		if a.says != "" {
			texts, _, _ := m.textRows("✎ ", "  ", a.says, width, codeFence{})
			for _, text := range texts {
				out = append(out, paneLine{key: key, text: text})
			}
			continue
		}
		out = m.thinkingLines(out, key, a.thinking, width)
	}
	out = m.toolLines(out, calls)
	if len(out) == 0 {
		out = []paneLine{{key: "starting", text: paint(dim, "starting")}}
	}
	shown := m.history.show(out, lines)
	pad := max(0, lines-len(shown))
	shown = append(make([]string, pad), shown...)
	m.history.toggles = append(make([]string, pad), m.history.toggles...)
	title := "Agent"
	if a := m.agentInfo; a != nil {
		for _, part := range []string{a.Harness, a.Model, a.Effort} {
			if part != "" {
				title += " · " + part
			}
		}
	}
	result := box(m.boxTitle(title, &m.history, width), strings.Join(shown, "\n"), width, m.border(&m.history))
	m.historyHeight = lipgloss.Height(result)
	return result
}

// thinkingLines is out and thinking, which folds like a run of tool calls: one row, its first line, until clicked
// or h opens it.
func (m *Model) thinkingLines(out []paneLine, key, thinking string, width int) []paneLine {
	if !m.unfolded(key) {
		first, _, _ := m.inline(oneLine(thinking))
		return append(out, paneLine{key: key, toggle: key, text: fold(false) + " ∴ " + first})
	}
	texts, _, _ := m.textRows(fold(true)+" ∴ ", "  ", strings.TrimRight(thinking, "\n"), width, codeFence{})
	for i, text := range texts {
		line := paneLine{key: key, text: text}
		if i == 0 {
			line.toggle = key
		}
		out = append(out, line)
	}
	return out
}

// toolLines is out and calls, a run of tool calls between what the agent says or thinks: a failed call on its
// own row, the others folded, in groups between failures, to one row each.
func (m *Model) toolLines(out []paneLine, calls []*activity) []paneLine {
	start := 0
	for i, a := range calls {
		if a.result == Failed {
			out = m.toolGroup(out, calls[start:i])
			out = append(out, m.callLine(a, ""))
			start = i + 1
		}
	}
	return m.toolGroup(out, calls[start:])
}

// toolGroup is out and calls on one dim row: how many and which tools, the detail of the one that runs, if one
// does; each call on its own row under it once unfolded.
func (m *Model) toolGroup(out []paneLine, calls []*activity) []paneLine {
	if len(calls) == 0 {
		return out
	}
	var names []string
	counts, runs, runningDetail := map[string]int{}, false, ""
	for _, a := range calls {
		if counts[a.tool]++; counts[a.tool] == 1 {
			names = append(names, a.tool)
		}
		if a.result == "" || a.result == Running {
			runs = true
			runningDetail, _, _ = strings.Cut(a.detail, "\n")
		}
	}
	for i, tool := range names {
		if counts[tool] > 1 {
			names[i] += fmt.Sprintf(" ×%d", counts[tool])
		}
	}
	n := len(calls)
	line := fmt.Sprintf("%d tool%s: %s", n, map[bool]string{true: "s"}[n > 1], strings.Join(names, ", "))
	key := fmt.Sprintf("tools:%p", calls[0])
	unfolded := m.unfolded(key)
	head := fold(unfolded) + " "
	if runs {
		if !unfolded && runningDetail != "" {
			line += " · " + runningDetail
		}
		out = append(out, paneLine{key: key, toggle: key, text: head + m.icon(Running) + " " + paint(dim, line)})
	} else {
		out = append(out, paneLine{key: key, toggle: key, text: head + paint(dim, "· "+line)})
	}
	if unfolded {
		for _, a := range calls {
			out = append(out, m.callLine(a, key))
		}
	}
	return out
}

// callLine is call a on one row: its state's icon and word, its tool, its detail and, after " · ", the first line
// of its output, without terminal controls.
func (m *Model) callLine(a *activity, group string) paneLine {
	state := a.result
	if state != Passed && state != Failed {
		state = Running
	}
	word := cmp.Or(map[State]string{Passed: "ok"}[state], string(state))
	line := a.tool + " " + oneLine(a.detail)
	if summary := oneLine(a.summary); summary != "" {
		line += " · " + summary
	}
	return paneLine{key: fmt.Sprintf("activity:%p", a), parent: group,
		text: "    " + m.icon(state) + " " + paint(states[state][1], word) + " " + paint(dim, line)}
}

// oneLine is the first line of s, without terminal controls.
func oneLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return style.DisplayText(ansi.Strip(line))
}
