# Agent instructions

- Before completing any work, run `make fmt` and then `make lint` locally.
- Fix any formatting or lint failures and rerun the commands. If a command cannot run, report the command and the reason in the final response.
- When implementing, debugging, reviewing, or refactoring code, apply the `karpathy-guidelines` skill (global, also at `~/.agents/skills/karpathy-guidelines/SKILL.md`): surface assumptions, prefer the simplest sufficient solution, make surgical changes, define verifiable goals.
- When authoring or reviewing code, use the `ponytail` skill (global, `~/.claude/skills/ponytail`): pick the laziest solution that actually works — stdlib before dependencies, fewest files, shortest working diff, no speculative abstractions.
