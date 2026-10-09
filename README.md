# todo

A small command-line todo list manager written entirely in Go, with zero
external dependencies. Tasks are stored as JSON in `~/.todo.json` (or a
custom path via `--file`/`-f` or the `TODO_FILE` environment variable), so
your list persists between runs.

## Build

Requires Go 1.21+.

```bash
go build -o todo .
```

This produces a single `todo` binary you can put anywhere on your `PATH`.

## Usage

```
todo add "task title" [--priority|-p low|medium|high] [--due|-d YYYY-MM-DD]
todo list|ls [--all|-a]
todo done <id> [id ...]
todo undone <id> [id ...]
todo rm <id> [id ...]
todo clear [--yes|-y]
todo help
```

### Examples

```bash
todo add "Write quarterly report" -p high -d 2026-07-25
todo add "Buy milk"
todo list                 # shows pending items, sorted by priority/due date
todo list --all           # also shows completed items
todo done 1               # mark item #1 complete
todo undone 1             # reopen item #1
todo rm 2                 # delete item #2
todo clear -y             # delete all completed items without prompting
```

Use a separate list file for a project, for example:

```bash
todo -f ./project-tasks.json add "Set up CI"
```

## How it's organized

- `main.go` — CLI argument parsing and command dispatch (add, list, done,
  undone, rm, clear, help).
- `todo.go` — the `Todo` and `Store` types: in-memory data model, sorting
  logic, and JSON load/save to disk.

## Sorting

`todo list` orders items as: incomplete before completed, then by priority
(high → medium → low), then by due date (soonest first, items without a due
date last), then by ID. Overdue, incomplete items are flagged `OVERDUE`.
