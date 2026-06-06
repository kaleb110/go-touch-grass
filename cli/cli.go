// Package cli is a command registry and orchestration
package cli

import (
	"flag"
	"fmt"
	"os"
)

// Subcommand defines the interface for separate CLI categories
type Subcommand interface {
	Name() string                         // e.g., "server" or "db"
	Description() string                  // For the help menu
	Init([]string) (*flag.FlagSet, error) // Sets up unique flags for this command
	Run() error                           // Executes the logic
}

type App struct {
	commands map[string]Subcommand
}

func NewApp() *App {
	return &App{commands: make(map[string]Subcommand)}
}

func (a *App) Register(cmd Subcommand) {
	a.commands[cmd.Name()] = cmd
}

func (a *App) Run() error {
	if len(os.Args) < 2 {
		a.PrintUsage()
		return fmt.Errorf("no subcommand provided")
	}

	// Handle standard global help flags: `app --help` or `app -h`
	if os.Args[1] == "--help" || os.Args[1] == "-h" {
		a.PrintUsage()
		return nil
	}

	subcommandName := os.Args[1]
	cmd, exists := a.commands[subcommandName]
	if !exists {
		a.PrintUsage()
		return fmt.Errorf("unknown subcommand: %s", subcommandName)
	}

	// Initialize the subcommand's unique flag set with the remaining args
	fs, err := cmd.Init(os.Args[2:])
	if err != nil {
		return err
	}

	// cases we want to early exit
	if fs == nil {
		return nil
	}

	// Parse only the flags belonging to this subcommand
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}

	// Execute the command logic
	return cmd.Run()
}

// PrintUsage prints the top-level application help menu
func (a *App) PrintUsage() {
	fmt.Fprintf(os.Stderr, "Usage: go-touch-grass <subcommand> [flags]\n")
	fmt.Fprintf(os.Stderr, "\nAvailable subcommands:\n")
	for _, cmd := range a.commands {
		fmt.Fprintf(os.Stderr, "  %-12s %s\n", cmd.Name(), cmd.Description())
	}
	fmt.Fprintf(os.Stderr, "\nUse \"go-touch-grass <subcommand> --help\" for more information about a subcommand.\n")
}
