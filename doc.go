// Package runscreen draws the run screen of an agent pipeline: a pipeline box of steps on the left and the
// agent's events on the right. It draws what the caller gives it and reads nothing from disk.
//
// New is the screen, a bubbletea model; Show runs it on a terminal. The caller sends it a Pipeline, the Agent
// that runs, each Event the agent makes, which adapter/claude, adapter/codex and adapter/pi read from each CLI's
// stream, and Boxes to show under the Agent box. SetMarkdown with markdown.Inline renders what the agent says.
package runscreen
