// Package gotouchgrass runs a cli app in the background
package gotouchgrass

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

type FlagCfg struct {
	StateFile    string // file to store state to
	TickInterval int // time to write to disk (as JSON)
	SimulateNext bool // test tomorrow's rollover safely
}

const (
	defaultStateFile    = ".local/share/go-touch-grass/state.json"
	defaultTickInterval = 60
)

func main()  {
		cfg := &FlagCfg{}

	flag.StringVar(&cfg.StateFile, "state", defaultStateFile, "state file path (JSON)")
	flag.IntVar(&cfg.TickInterval, "tick", defaultTickInterval, "ticker interval in seconds")
	flag.BoolVar(&cfg.SimulateNext, "sim-tomorrow", false, "simulate tomorrow's date for testing rollover")

	flag.Parse()

	if err := validateFlag(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n\n", err)
		flag.Usage()
		os.Exit(1)
	}

	// bussiness logic.
}

// validate flag values
func validateFlag(cfg *FlagCfg) error {
	if cfg.StateFile == "" {
		return errors.New("empty file path")
	}
	if cfg.TickInterval <= 0 {
		return errors.New("tick interval must be greater than 0")
	}
	return nil
}