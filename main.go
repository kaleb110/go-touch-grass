// Package gotouchgrass runs a cli app in the background
package gotouchgrass

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

// TrackingData is tracking object
type TrackingData struct {
	Date        string        `json:"date"`
	ElapsedTime time.Duration `json:"elapsed_time"`
}

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
	trackTime(fullPath, intervalDuration, cfg.SimulateNext)

	// loop untill there is a signal
	for {
		select {
		case <-ticker.C:
			trackTime(fullPath, intervalDuration, cfg.SimulateNext)
		case <-stop:
			fmt.Println("Shutting down tracker cleanly.")
			trackTime(fullPath, intervalDuration, cfg.SimulateNext)
			return
		}
	}

}

// trackTime initiated time tracking and writes to disk
func trackTime(filePath string, tick time.Duration, simTomorrow bool) {
	currentTime := time.Now()
	if simTomorrow {
		currentTime = currentTime.AddDate(0, 0, 1)
	}
	today := currentTime.Format("2006-01-02")

	// store in slice, we can get history
	var history []TrackingData

	// try reading existing history file
	fileBytes, err := os.ReadFile(filePath)
	if err == nil {
		err := json.Unmarshal(fileBytes, &history)
		if err != nil {
			fmt.Println("failed to parse, continued...")
		}
	}

	// look to see if we already have an entry for today
	// on reboot, login it will continue from last stored time
	foundIndex := -1
	for i, record := range history {
		if record.Date == today {
			foundIndex = i
			break
		}
	}

	if foundIndex != -1 {
		// today already exists, just add the tick time to it
		history[foundIndex].ElapsedTime += tick

		// Print current stats
		printStats(history[foundIndex])
	} else {
		// new day detected, Create a fresh record and append it to history
		fmt.Printf("New day detected (%s). Starting new tracking record.\n", today)
		newRecord := TrackingData{
			Date:        today,
			ElapsedTime: tick, // start with the initial tick
		}
		history = append(history, newRecord)

		printStats(newRecord)
	}

	// write the entire updated history array back to disk
	updatedBytes, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		fmt.Printf("Failed to marshal JSON: %v\n", err)
		return
	}

	err = os.WriteFile(filePath, updatedBytes, 0644)
	if err != nil {
		fmt.Println("failed to update state. try again.")
	}
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

// helper to keep formatting clean
func printStats(data TrackingData) {
	hours := int(data.ElapsedTime.Hours())
	minutes := int(data.ElapsedTime.Minutes()) % 60
	seconds := int(data.ElapsedTime.Seconds()) % 60
	fmt.Printf("Total laptop usage today (%s): %dh %dm %ds\n", data.Date, hours, minutes, seconds)
}
