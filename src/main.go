// Command todo is a small, dependency-free command-line todo list manager.
// Data is persisted as JSON in ~/.todo.json (override with --file or $TODO_FILE).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const dateLayout = "2006-01-02"

func main() {
	// the main entry point
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {

	storePath, args, err := extractFileFlag(args)
	if err != nil {
		return err
	}
	if storePath == "" {
		storePath, err = defaultStorePath()
		if err != nil {
			return err
		}
	}

	store, err := NewStore(storePath)
	if err != nil {
		return err
	}

	if len(args) == 0 {
		printUsage()
		return nil
	}

	cmd, rest := args[0], args[1:]

	switch cmd {
	case "add":
		return cmdAdd(store, rest)
	case "list", "ls":
		return cmdList(store, rest)
	case "done", "complete":
		return cmdDone(store, rest, true)
	case "undone", "reopen":
		return cmdDone(store, rest, false)
	case "rm", "remove", "delete":
		return cmdRemove(store, rest)
	case "clear":
		return cmdClear(store, rest)
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		printUsage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

// extractFileFlag pulls a leading --file/-f value out of args so subcommands
// don't each need to know about it. Returns the remaining args unchanged in order.
func extractFileFlag(args []string) (string, []string, error) {
	fs := flag.NewFlagSet("todo", flag.ContinueOnError)
	fs.SetOutput(new(strings.Builder)) // suppress default error printing
	file := fs.String("file", "", "path to the todo storage file")
	fs.StringVar(file, "f", "", "path to the todo storage file (shorthand)")

	// flag package stops at the first non-flag arg, so scan manually
	// to allow the command name to come first, e.g. `todo add --file x "title"`.
	var flagArgs, cmdArgs []string
	seenCmd := false
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !seenCmd && !strings.HasPrefix(a, "-") {
			seenCmd = true
			cmdArgs = append(cmdArgs, a)
			continue
		}
		if a == "--file" || a == "-f" {
			flagArgs = append(flagArgs, a)
			if i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
			continue
		}
		if strings.HasPrefix(a, "--file=") || strings.HasPrefix(a, "-f=") {
			flagArgs = append(flagArgs, a)
			continue
		}
		cmdArgs = append(cmdArgs, a)
	}

	if len(flagArgs) == 0 {
		return "", cmdArgs, nil
	}
	if err := fs.Parse(flagArgs); err != nil {
		return "", nil, err
	}
	return *file, cmdArgs, nil
}

func defaultStorePath() (string, error) {
	if p := os.Getenv("TODO_FILE"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".todo.json"), nil
}

func cmdAdd(store *Store, args []string) error {
	fs := flag.NewFlagSet("add", flag.ExitOnError)
	priority := fs.String("priority", "medium", "priority: low, medium, high")
	fs.StringVar(priority, "p", "medium", "priority (shorthand)")
	due := fs.String("due", "", "due date, format YYYY-MM-DD")
	fs.StringVar(due, "d", "", "due date (shorthand)")

	var flagArgs, titleWords []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "-p" || a == "--priority" || a == "-d" || a == "--due":
			flagArgs = append(flagArgs, a)
			if i+1 < len(args) {
				i++
				flagArgs = append(flagArgs, args[i])
			}
		case strings.HasPrefix(a, "-p=") || strings.HasPrefix(a, "--priority=") ||
			strings.HasPrefix(a, "-d=") || strings.HasPrefix(a, "--due="):
			flagArgs = append(flagArgs, a)
		default:
			titleWords = append(titleWords, a)
		}
	}
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}

	title := strings.TrimSpace(strings.Join(titleWords, " "))
	if title == "" {
		return fmt.Errorf("add requires a title, e.g. todo add \"buy milk\"")
	}

	var dueTime *time.Time
	if *due != "" {
		t, err := time.Parse(dateLayout, *due)
		if err != nil {
			return fmt.Errorf("invalid --due date %q, expected format YYYY-MM-DD", *due)
		}
		dueTime = &t
	}

	t := store.Add(title, *priority, dueTime)
	if err := store.Save(); err != nil {
		return err
	}
	fmt.Printf("Added #%d: %s\n", t.ID, t.Title)
	return nil
}

func cmdList(store *Store, args []string) error {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	all := fs.Bool("all", false, "show completed todos too")
	fs.BoolVar(all, "a", false, "show completed todos too (shorthand)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	items := store.Sorted()
	if len(items) == 0 {
		fmt.Println("No todos yet. Add one with: todo add \"my task\"")
		return nil
	}

	shown := 0
	for _, t := range items {
		if t.Done && !*all {
			continue
		}
		printTodo(t)
		shown++
	}
	if shown == 0 {
		fmt.Println("Nothing pending. Use --all to see completed items too.")
	}
	return nil
}

func printTodo(t *Todo) {
	box := "[ ]"
	if t.Done {
		box = "[x]"
	}

	priorityMark := map[string]string{"high": "!!!", "medium": "!!", "low": "!"}[t.Priority]

	line := fmt.Sprintf("%s #%-3d %s (%s)", box, t.ID, t.Title, priorityMark)

	if t.DueDate != nil {
		line += fmt.Sprintf("  due:%s", t.DueDate.Format(dateLayout))
		if !t.Done && t.DueDate.Before(time.Now().Truncate(24*time.Hour)) {
			line += " OVERDUE"
		}
	}
	fmt.Println(line)
}

func cmdDone(store *Store, args []string, done bool) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: todo done <id> [id ...]")
	}
	for _, arg := range args {
		id, err := strconv.Atoi(arg)
		if err != nil {
			return fmt.Errorf("invalid id %q", arg)
		}
		t := store.Find(id)
		if t == nil {
			return fmt.Errorf("no todo with id %d", id)
		}
		if t.Done != done {
			store.Toggle(id)
		}
	}
	if err := store.Save(); err != nil {
		return err
	}
	verb := "Completed"
	if !done {
		verb = "Reopened"
	}
	fmt.Printf("%s %d item(s)\n", verb, len(args))
	return nil
}

func cmdRemove(store *Store, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: todo rm <id> [id ...]")
	}
	removed := 0
	for _, arg := range args {
		id, err := strconv.Atoi(arg)
		if err != nil {
			return fmt.Errorf("invalid id %q", arg)
		}
		if store.Remove(id) {
			removed++
		} else {
			fmt.Fprintf(os.Stderr, "warning: no todo with id %d\n", id)
		}
	}
	if err := store.Save(); err != nil {
		return err
	}
	fmt.Printf("Removed %d item(s)\n", removed)
	return nil
}

func cmdClear(store *Store, args []string) error {
	fs := flag.NewFlagSet("clear", flag.ExitOnError)
	yes := fs.Bool("yes", false, "skip confirmation prompt")
	fs.BoolVar(yes, "y", false, "skip confirmation prompt (shorthand)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if !*yes {
		fmt.Print("Remove all completed todos? [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))
		if resp != "y" && resp != "yes" {
			fmt.Println("Cancelled.")
			return nil
		}
	}

	n := store.ClearDone()
	if err := store.Save(); err != nil {
		return err
	}
	fmt.Printf("Cleared %d completed item(s)\n", n)
	return nil
}

func printUsage() {
	fmt.Print(`todo — a simple command-line todo list

Usage:
  todo add "task title" [--priority|-p low|medium|high] [--due|-d YYYY-MM-DD]
  todo list|ls [--all|-a]
  todo done <id> [id ...]
  todo undone <id> [id ...]
  todo rm <id> [id ...]
  todo clear [--yes|-y]
  todo help

Global flag:
  --file, -f <path>   use a specific storage file (default: ~/.todo.json,
                       or $TODO_FILE if set)

Examples:
  todo add "Write report" -p high -d 2026-07-25
  todo list
  todo done 1 3
  todo rm 2
`)
}
