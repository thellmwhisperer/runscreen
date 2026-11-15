//go:build linux || darwin

package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/thellmwhisperer/runscreen"
)

func ioctl(f *os.File, op, arg uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), op, arg); errno != 0 {
		return errno
	}
	return nil
}

// resize gives the terminal tty cols columns and rows rows; the program on it hears SIGWINCH.
func resize(t *testing.T, tty *os.File, cols, rows uint16) {
	t.Helper()
	ws := struct{ rows, cols, x, y uint16 }{rows, cols, 0, 0}
	if err := ioctl(tty, syscall.TIOCSWINSZ, uintptr(unsafe.Pointer(&ws))); err != nil {
		t.Fatal(err)
	}
}

// shown is what a terminal showed so far; more says it showed more.
type shown struct {
	sync.Mutex
	b    []byte
	more chan struct{}
}

func (s *shown) Write(p []byte) (int, error) {
	s.Lock()
	s.b = append(s.b, p...)
	s.Unlock()
	select {
	case s.more <- struct{}{}:
	default:
	}
	return len(p), nil
}

// since is what the terminal showed from byte i on.
func (s *shown) since(i int) string {
	s.Lock()
	defer s.Unlock()
	return string(s.b[min(i, len(s.b)):])
}

func (s *shown) len() int {
	s.Lock()
	defer s.Unlock()
	return len(s.b)
}

// demo is the demo running on a terminal of its own.
type demo struct {
	t      *testing.T
	master *os.File
	tty    *os.File
	cmd    *exec.Cmd
	out    *shown
	read   chan struct{}
	m      *runscreen.Model // the same screen in the test, every key and click given to both
}

// build is the demo's binary, built once for every test; with E2E_COVERDIR set, built to write its coverage there.
var build = sync.OnceValues(func() (string, error) {
	dir, err := os.MkdirTemp("", "demo-e2e")
	if err != nil {
		return "", err
	}
	bin := filepath.Join(dir, "demo")
	args := []string{"build", "-o", bin}
	if os.Getenv("E2E_COVERDIR") != "" {
		args = append(args, "-cover", "-coverpkg=github.com/thellmwhisperer/runscreen/...")
	}
	out, err := exec.Command("go", append(args, ".")...).CombinedOutput()
	if err != nil {
		return "", errors.New(string(out))
	}
	return bin, nil
})

// command is the demo's command with args, its coverage going to E2E_COVERDIR when set.
func command(t *testing.T, args ...string) *exec.Cmd {
	t.Helper()
	bin, err := build()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "TERM=xterm-256color")
	if dir := os.Getenv("E2E_COVERDIR"); dir != "" {
		cmd.Env = append(cmd.Env, "GOCOVERDIR="+dir)
	}
	return cmd
}

// start runs the demo with args on a terminal of cols by rows.
func start(t *testing.T, cols, rows uint16, args ...string) *demo {
	t.Helper()
	master, tty, err := openPTY()
	if err != nil {
		t.Fatal(err)
	}
	resize(t, tty, cols, rows)
	d := &demo{t: t, master: master, tty: tty, cmd: command(t, args...), out: &shown{more: make(chan struct{}, 1)},
		read: make(chan struct{})}
	d.cmd.Stdin, d.cmd.Stdout, d.cmd.Stderr = tty, tty, tty
	d.cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := d.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _, _ = io.Copy(d.out, master); close(d.read) }()
	t.Cleanup(func() {
		_ = d.cmd.Process.Kill()
		_ = master.Close()
		_ = tty.Close()
	})
	d.answer()
	return d
}

// answer answers, as a terminal does, the question about its background and its cursor that the demo asks as
// it starts; unanswered, the demo waits 5 s for it.
func (d *demo) answer() {
	d.t.Helper()
	deadline := time.After(10 * time.Second)
	for !strings.Contains(d.out.since(0), "\x1b]11;?") {
		select {
		case <-d.out.more:
		case <-deadline:
			d.t.Fatalf("the demo asked nothing of its terminal: %q", d.out.since(0))
		}
	}
	if _, err := d.master.WriteString("\x1b]11;rgb:0000/0000/0000\x1b\\\x1b[1;1R"); err != nil {
		d.t.Fatal(err)
	}
}

// until waits, 10 s at most, for the terminal to show each of want, its colours stripped, since byte i.
func (d *demo) until(i int, want ...string) {
	d.t.Helper()
	deadline := time.After(10 * time.Second)
	for {
		s := ansi.Strip(d.out.since(i))
		missing := ""
		for _, w := range want {
			if !strings.Contains(s, w) {
				missing = w
				break
			}
		}
		if missing == "" {
			return
		}
		select {
		case <-d.out.more:
		case <-d.read:
			d.t.Fatalf("the demo ended before it showed %q:\n%s", missing, s)
		case <-deadline:
			d.t.Fatalf("the terminal did not show %q:\n%s", missing, s)
		}
	}
}

// press types keys on the terminal and waits for it to show each of want after them.
func (d *demo) press(keys string, want ...string) {
	d.t.Helper()
	i := d.out.len()
	if _, err := d.master.WriteString(keys); err != nil {
		d.t.Fatal(err)
	}
	d.until(i, want...)
}

// act types keys on the terminal and gives msg to the screen in the test, then waits for the terminal to show
// each of want.
func (d *demo) act(keys string, msg tea.Msg, want ...string) {
	d.t.Helper()
	d.m.View() // drawn, as the terminal's screen was, before it reads msg
	d.m.Update(msg)
	d.m.View()
	d.press(keys, want...)
}

// key is keys typed on the terminal and msg given to the screen in the test; the terminal shows the Agent box's
// first row as the screen in the test has it, the row a scroll changes.
func (d *demo) key(keys string, msg tea.Msg) {
	d.t.Helper()
	d.m.View()
	d.m.Update(msg)
	d.press(keys, d.agentTop())
}

// then is keys typed on the terminal and msg given to the screen in the test; the terminal shows row y as the
// screen in the test has it once it read msg, up to its duration, which the two screens count apart.
func (d *demo) then(keys string, msg tea.Msg, y int) {
	d.t.Helper()
	d.m.View()
	d.m.Update(msg)
	want, _, _ := strings.Cut(d.row(y), "  ")
	d.press(keys, want)
}

// row is the row y of the screen in the test, without its borders.
func (d *demo) row(y int) string {
	d.t.Helper()
	return strings.TrimSpace(strings.Trim(strings.Split(ansi.Strip(d.m.View()), "\n")[y], "│"))
}

// wheel is the wheel turned n times at x, y, on both screens; the terminal shows the Agent box's first row.
func (d *demo) wheel(button tea.MouseButton, x, y, n int) {
	d.t.Helper()
	code := map[tea.MouseButton]int{tea.MouseButtonWheelUp: 64, tea.MouseButtonWheelDown: 65}[button]
	for range n {
		d.m.View()
		d.m.Update(tea.MouseMsg{X: x, Y: y, Button: button, Action: tea.MouseActionPress})
	}
	d.press(strings.Repeat(mouse(code, x, y), n), d.agentTop())
}

// agentTop is the first row inside the Agent box of the screen in the test.
func (d *demo) agentTop() string {
	d.t.Helper()
	_, y := at(d.t, d.m, "╭─ Agent")
	row := strings.Split(ansi.Strip(d.m.View()), "\n")[y+1]
	return strings.TrimSpace(strings.Trim(row, "│"))
}

// click is a left click on the row that shows text, on both screens, then waits for the terminal to show want.
func (d *demo) click(text string, want ...string) {
	d.t.Helper()
	x, y := at(d.t, d.m, text)
	d.act(mouse(0, x, y), tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress}, want...)
}

// quit types q and waits for the demo to end, then gives the last frame it left on the terminal.
func (d *demo) quit() string {
	d.t.Helper()
	if _, err := d.master.WriteString("q"); err != nil {
		d.t.Fatal(err)
	}
	if err := d.cmd.Wait(); err != nil {
		d.t.Fatalf("the demo ended with %v:\n%s", err, ansi.Strip(d.out.since(0)))
	}
	_ = d.tty.Close()
	<-d.read
	s := d.out.since(0)
	return ansi.Strip(s[strings.LastIndex(s, "\x1b[?1049l"):])
}
