// Package gotouchgrass runs a cli app in the background
package gotouchgrass

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type FlagCfg struct {
	StateFile    string // file to store state to
	TickInterval int    // time to write to disk (as JSON)
	SimulateNext bool   // test tomorrow's rollover safely
}

const (
	defaultStateFile    = ".local/share/go-touch-grass/state.json"
	defaultTickInterval = 60
)

func main() {
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

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error finding home dir: %v\n", err)
		return
	}
	fullPath := filepath.Join(home, cfg.StateFile)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		fmt.Printf("Error creating storage dir: %v\n", err)
		return
	}

	fmt.Println("Laptop usage tracker started...")

	// listen for signal

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	intervalDuration := time.Duration(cfg.TickInterval) * time.Second
	ticker := time.NewTicker(intervalDuration)
	defer ticker.Stop() // stop ticker on exit signal from sysd or ctrl+c

	

	// track time
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
