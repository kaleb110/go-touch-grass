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

// AppVersion represents the current build release version
const AppVersion string = "1.0.2"
// CurrentSchemaVersion keeps track of the JSON storage format style
const CurrentSchemaVersion int = 1

type TrackingData struct {
	Date        string        `json:"date"`
	ElapsedTime time.Duration `json:"elapsed_time"`
}

// StateEnvelope acts as the outer container to protect backward compatibility
type StateEnvelope struct {
	SchemaVersion int            `json:"schema_version"`
	History       []TrackingData `json:"history"`
}

type FlagCfg struct {
	StateFile    string
	TickInterval int
	Report       string
	Update       bool
	SimulateNext bool
	ShowVersion  bool // New flag
}

const (
	defaultStateFile    = ".local/share/go-touch-grass/state.json"
	defaultTickInterval = 60
)

type ReportLength string
const (
	todayReport ReportLength = "today"
	lastWeek    ReportLength = "lastWeek"
)

func main() {
	cfg := &FlagCfg{}

	flag.StringVar(&cfg.StateFile, "state", defaultStateFile, "state file path (JSON)")
	flag.IntVar(&cfg.TickInterval, "tick", defaultTickInterval, "ticker interval in seconds")
	flag.BoolVar(&cfg.Update, "update", false, "whether to start on updater mode or not")
	flag.StringVar(&cfg.Report, "report", string(todayReport), "view report")
	flag.BoolVar(&cfg.SimulateNext, "sim-tomorrow", false, "simulate tomorrow's date for testing rollover")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "print current binary version")

	flag.Parse()

	// Handle version print immediately before running validations
	if cfg.ShowVersion {
		fmt.Printf("go-touch-grass version %s (Schema v%d)\n", AppVersion, CurrentSchemaVersion)
		return
	}

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

	// Early exit for reporting viewer (read-only mode)
	if !cfg.Update {
		cfg.trackTime(fullPath, time.Duration(cfg.TickInterval)*time.Second)
		return
	}

	fmt.Println("Laptop usage tracker started...")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	intervalDuration := time.Duration(cfg.TickInterval) * time.Second
	ticker := time.NewTicker(intervalDuration)
	defer ticker.Stop()

	cfg.trackTime(fullPath, intervalDuration)

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

func (f *FlagCfg) trackTime(filePath string, tick time.Duration) {
	currentTime := time.Now()
	if f.SimulateNext {
		currentTime = currentTime.AddDate(0, 0, 1)
	}
	today := currentTime.Format("2006-01-02")

	// Read state using the migration-safe parsing utility
	envelope, err := loadStateAndMigrate(filePath)
	if err != nil {
		fmt.Printf("State initialization error: %v, continuing with fresh slate...\n", err)
		envelope = &StateEnvelope{SchemaVersion: CurrentSchemaVersion, History: []TrackingData{}}
	}

	foundIndex := -1
	for i, record := range envelope.History {
		if record.Date == today {
			foundIndex = i
			break
		}
	}

	if f.Update {
		if foundIndex != -1 {
			envelope.History[foundIndex].ElapsedTime += tick
			printStats(envelope.History[foundIndex])
		} else {
			fmt.Printf("New day detected (%s). Starting new tracking record.\n", today)
			newRecord := TrackingData{Date: today, ElapsedTime: tick}
			envelope.History = append(envelope.History, newRecord)
			printStats(newRecord)
		}

		// Write unified updated envelope structures directly back to filesystems
		updatedBytes, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			fmt.Printf("failed to marshal JSON payload: %v\n", err)
			return
		}
		_ = os.WriteFile(filePath, updatedBytes, 0644)
		return
	}

	// Read-only reporting logic
	if foundIndex != -1 {
		switch ReportLength(f.Report) {
		case todayReport:
			printStats(envelope.History[foundIndex])
		case lastWeek:
			start := len(envelope.History) - 7
			if start < 0 {
				start = 0
			}
			for _, v := range envelope.History[start:] {
				printStats(v)
			}
		}
	}
}

// loadStateAndMigrate intercepts raw data logs and maps legacy layouts forward seamlessly
func loadStateAndMigrate(filePath string) (*StateEnvelope, error) {
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return &StateEnvelope{SchemaVersion: CurrentSchemaVersion, History: []TrackingData{}}, nil
	}

	// Step A: Attempt to parse file as an Envelope structure (v1.1+)
	var env StateEnvelope
	if err := json.Unmarshal(fileBytes, &env); err == nil && env.SchemaVersion > 0 {
		return &env, nil
	}

	// Step B: FALLBACK FOR BACKWARD COMPATIBILITY
	// If it fails, the user is upgrading from v1.0.0 where the file was a plain array (`[]TrackingData`).
	var legacyHistory []TrackingData
	if err := json.Unmarshal(fileBytes, &legacyHistory); err == nil {
		fmt.Println("Migrating legacy state history schema file to v1 schema protocol...")
		return &StateEnvelope{
			SchemaVersion: CurrentSchemaVersion,
			History:       legacyHistory,
		}, nil
	}

	return nil, errors.New("unknown file format or corrupted structure")
}

func (f *FlagCfg) validateFlag() error {
	if f.StateFile == "" {
		return errors.New("empty file path")
	}
	if f.TickInterval <= 0 {
		return errors.New("tick interval must be greater than 0")
	}
	switch ReportLength(f.Report) {
	case todayReport, lastWeek:
		return nil
	default:
		return errors.New("invalid report type")
	}
}

func printStats(data TrackingData) {
	hours := int(data.ElapsedTime.Hours())
	minutes := int(data.ElapsedTime.Minutes()) % 60
	seconds := int(data.ElapsedTime.Seconds()) % 60
	fmt.Printf("Total machine usage (%s): %dh %dm %ds\n", data.Date, hours, minutes, seconds)
}