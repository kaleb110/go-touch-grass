// Package gotouchgrass runs a cli app in the background
package main

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
	Report       string
	Update       bool
	SimulateNext bool // test tomorrow's rollover safely
}

const (
	defaultStateFile    = ".local/share/go-touch-grass/state.json"
	defaultTickInterval = 60
)

type ReportLength string

const (
	todayReport ReportLength = "today"
	lastWeek    ReportLength = "lastWeek"
	lastMonth   ReportLength = "lastMonth"
)

func main() {
	cfg := &FlagCfg{}

	flag.StringVar(&cfg.StateFile, "state", defaultStateFile, "state file path (JSON)")
	flag.IntVar(&cfg.TickInterval, "tick", defaultTickInterval, "ticker interval in seconds")
	flag.BoolVar(&cfg.Update, "update", false, "weather to start on updater mode or not")
	flag.StringVar(&cfg.Report, "report", string(todayReport), "view report")
	flag.BoolVar(&cfg.SimulateNext, "sim-tomorrow", false, "simulate tomorrow's date for testing rollover")

	flag.Parse()

	if err := cfg.validateFlag(); err != nil {
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

	intervalDuration := time.Duration(cfg.TickInterval) * time.Second

	if !cfg.Update {
		// track time
		cfg.trackTime(fullPath, intervalDuration)
		return
	}
	
	ticker := time.NewTicker(intervalDuration)
	defer ticker.Stop() // stop ticker on exit signal from sysd or ctrl+c

	// listen for signal

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// loop untill there is a signal
	for {
		select {
		case <-ticker.C:
			cfg.trackTime(fullPath, intervalDuration)
		case <-stop:
			fmt.Println("Shutting down tracker cleanly.")
			cfg.trackTime(fullPath, intervalDuration)
			return
		}
	}

}

// trackTime initiated time tracking and writes to disk
func (f *FlagCfg) trackTime(filePath string, tick time.Duration) {
	currentTime := time.Now()
	if f.SimulateNext {
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

	if f.Update {
		if err := tickUpdater(history, today, filePath, tick, foundIndex); err != nil {
			fmt.Println(err)
			return
		}

		return
	}

	if foundIndex != -1 {
		switch ReportLength(f.Report) {
		case todayReport:
			todayR := history[foundIndex]

			printStats(todayR)

		case lastWeek:
			start := len(history) - 7

			if start < 0 {
				start = 0
			}

			lastWeek := history[start:]

			for _, v := range lastWeek {
				printStats(v)
			}
		}
	}

}

// validate flag values
func (f *FlagCfg) validateFlag() error {
	if f.StateFile == "" {
		return errors.New("empty file path")
	}
	if f.TickInterval <= 0 {
		return errors.New("tick interval must be greater than 0")
	}

	switch ReportLength(f.Report) {
	case todayReport, lastMonth, lastWeek:
		return nil
	default:
		return errors.New("invalid report type")
	}
}

func tickUpdater(history []TrackingData, today, filePath string, tick time.Duration, foundIndex int) error {

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
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	err = os.WriteFile(filePath, updatedBytes, 0644)
	if err != nil {
		return fmt.Errorf("failed to update state. try again")
	}

	return nil
}

// helper to keep formatting clean
func printStats(data TrackingData) {
	hours := int(data.ElapsedTime.Hours())
	minutes := int(data.ElapsedTime.Minutes()) % 60
	seconds := int(data.ElapsedTime.Seconds()) % 60
	fmt.Printf("Total machine usage (%s): %dh %dm %ds\n", data.Date, hours, minutes, seconds)
}
