# Recordings

One real JSON stream from each agent CLI, recorded on 15-Nov-2026 on this repository at commit `7f7f666`. The adapters' tests read them. Nothing here is written by hand.

Every run got the same prompt:

> Read doc.go and .golangci.yml, run `go vet ./...`, then decide which two of the enabled linters matter most for a terminal UI library and why. Answer in three short Markdown bullets. Do not change any file.

| File | CLI | Model | Command |
| --- | --- | --- | --- |
| `claude.jsonl` | Claude Code 2.1.323 | claude-sonnet-5-5 | `claude -p <prompt> --model claude-sonnet-5-5 --effort high --setting-sources project --strict-mcp-config --output-format stream-json --verbose --allowedTools 'Read,Bash(go vet:*),Glob,Grep'` |
| `codex.jsonl` | codex-cli 0.167.0-alpha.4.1 | gpt-6.1-sol | `codex exec --json -m gpt-6.1-sol -c model_reasoning_effort=high -c model_reasoning_summary=detailed --sandbox read-only <prompt>`, with an empty `CODEX_HOME` |
| `pi.jsonl` | pi 1.5.2 | gpt-6.1-sol | `pi -p --mode json --thinking high --no-session --no-skills --no-context-files --no-extensions --no-mcp --no-prompt-templates --no-approve <prompt>` |

Each CLI ran without the user's instructions, skills, hooks or extensions. What was changed after recording:

- the working directory reads `/work/runscreen` and the home directory `/home/user`;
- in `claude.jsonl`, the hook events are dropped and the `init` event's lists of skills, commands, agents, plugins and MCP servers are emptied.

What each recording holds:

- `claude.jsonl`: thinking, three tool calls with their results, and the final text.
- `codex.jsonl`: two command executions and the agent's messages. Codex printed no reasoning items in this run.
- `pi.jsonl`: streamed text, three tool calls with their results. pi printed no thinking in this run.
