// Package cli is a command registry and orchestration.
package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
)

// Subcommand defines the interface for separate CLI categories.
// Init parses the subcommand-specific flags and stores results on the
// receiver; Run executes the logic. Parse happens exactly once, in Init.
type Subcommand interface {
	Name() string                         // e.g., "tracker" or "install"
	Description() string                  // for the help menu
	Init(args []string) error             // parse flags for this command
	Run() error                           // execute the logic
}

type App struct {
	commands map[string]Subcommand
	order    []string // preserves registration order for stable help output
}

func NewApp() *App {
	return &App{commands: make(map[string]Subcommand)}
}

func (a *App) Register(cmd Subcommand) {
	if _, exists := a.commands[cmd.Name()]; exists {
		return
	}
	a.commands[cmd.Name()] = cmd
	a.order = append(a.order, cmd.Name())
}

func (a *App) Run() error {
	if len(os.Args) < 2 {
		a.PrintUsage(os.Stderr)
		return fmt.Errorf("no subcommand provided")
	}

	switch os.Args[1] {
	case "--version", "-v":
		fmt.Printf("go-touch-grass version v%s (Schema v%d)\n", AppVersion, CurrentSchemaVersion)
		return nil
	case "--help", "-h":
		a.PrintUsage(os.Stdout)
		return nil
	}

	name := os.Args[1]
	cmd, exists := a.commands[name]
	if !exists {
		a.PrintUsage(os.Stderr)
		return fmt.Errorf("unknown subcommand: %s", name)
	}

	if err := cmd.Init(os.Args[2:]); err != nil {
		return err
	}
	return cmd.Run()
}

// PrintUsage prints the top-level application help menu to w.
func (a *App) PrintUsage(w io.Writer) {
	fmt.Fprintf(w, "go-touch-grass v%s — track daily machine usage\n\n", AppVersion)
	fmt.Fprintf(w, "Usage: go-touch-grass <subcommand> [flags]\n\n")
	fmt.Fprintf(w, "Global Options:\n")
	fmt.Fprintf(w, "  -v, --version  Print version info\n")
	fmt.Fprintf(w, "  -h, --help     Print application help\n\n")
	fmt.Fprintf(w, "Available subcommands:\n")

	// Stable, registration-ordered output (falls back to sorted for safety).
	names := append([]string{}, a.order...)
	if len(names) == 0 {
		for k := range a.commands {
			names = append(names, k)
		}
		sort.Strings(names)
	}
	for _, name := range names {
		fmt.Fprintf(w, "  %-12s %s\n", name, a.commands[name].Description())
	}
	fmt.Fprintf(w, "\nUse \"go-touch-grass <subcommand> --help\" for more information about a subcommand.\n")
}
