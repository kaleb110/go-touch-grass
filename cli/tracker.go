package cli

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

	"github.com/kaleb110/go-touch-grass/report"
)

const (
	defaultStateFile    = ".local/share/go-touch-grass/state.json"
	defaultTickInterval = 60
)

type TrackingData struct {
	Date        string        `json:"date"`
	ElapsedTime time.Duration `json:"elapsed_time"`
}

// StateEnvelope acts as the outer container to protect backward compatibility
type StateEnvelope struct {
	SchemaVersion int            `json:"schema_version"`
	History       []TrackingData `json:"history"`
}

type TrackerCMD struct {
	StateFile    string
	TickInterval int
	Report       string
	FilterBy     string
	Update       bool
	SimulateNext bool
	ShowVersion  bool
}

func NewTracker() *TrackerCMD {
	return &TrackerCMD{}
}

func (tc *TrackerCMD) Name() string {
	return "tracker"
}

func (tc *TrackerCMD) Description() string {
	return "track machine usage"
}

func (tc *TrackerCMD) Init(args []string) (*flag.FlagSet, error) {
	fs := flag.NewFlagSet(tc.Name(), flag.ExitOnError)

	// attack the flags to local flag group
	fs.StringVar(&tc.StateFile, "state", defaultStateFile, "state file path (JSON)")
	fs.IntVar(&tc.TickInterval, "tick", defaultTickInterval, "ticker interval in seconds")
	fs.BoolVar(&tc.Update, "update", false, "whether to start on updater mode or not")
	fs.StringVar(&tc.Report, "report", string(report.TodayReport), "view report")
	fs.StringVar(&tc.FilterBy, "filter-by", string(report.ListTime), "filter output by time format")
	fs.BoolVar(&tc.SimulateNext, "sim-tomorrow", false, "simulate tomorrow's date for testing rollover")
	fs.BoolVar(&tc.ShowVersion, "version", false, "print current binary version")

	// Hook custom usage into Go's native flag framework
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: app %s [flags]\n", tc.Name())
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		fs.PrintDefaults() // Prints this subcommand flags formatting automatically
	}

	return fs, nil
}

func (tc *TrackerCMD) Run() error {
	if tc.ShowVersion {
		fmt.Printf("go-touch-grass version %s (Schema v%d)\n", AppVersion, CurrentSchemaVersion)
		return nil
	}

	if err := tc.validateFlag(); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n\n", err)
		// TODO: show usage
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("error finding home dir: %v", err)
	}
	fullPath := filepath.Join(home, tc.StateFile)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		return fmt.Errorf("error creating storage dir: %v", err)
	}

	// Early exit for reporting viewer (read-only mode)
	if !tc.Update {
		tc.trackTime(fullPath, time.Duration(tc.TickInterval)*time.Second)
		return nil
	}

	/* running in update mode, should be the task of systemd service */

	fmt.Println("Machine usage tracker started...")

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	intervalDuration := time.Duration(tc.TickInterval) * time.Second
	ticker := time.NewTicker(intervalDuration)
	defer ticker.Stop()

	// start tracking
	tc.trackTime(fullPath, intervalDuration)

	/* listen for interuptions/tick */

	for {
		select {
		case <-ticker.C:
			tc.trackTime(fullPath, intervalDuration)
		case <-stop:
			fmt.Println("Shutting down tracker cleanly.")
			tc.trackTime(fullPath, intervalDuration)
			return nil
		}
	}
}

func (tc *TrackerCMD) validateFlag() error {
	if tc.StateFile == "" {
		return errors.New("empty file path")
	}
	if tc.TickInterval <= 0 {
		return errors.New("tick interval must be greater than 0")
	}
	switch report.ReportLength(tc.Report) {
	case report.TodayReport, report.LastWeek, report.LastMonth, report.LastYear, report.AllTime:
	default:
		return fmt.Errorf("invalid report type: %v", tc.Report)
	}

	switch report.FilterFormat(tc.FilterBy) {
	case report.TotalTime, report.ListTime:
		return nil
	default:
		return fmt.Errorf("invalid filter format: %v", tc.FilterBy)
	}
}

func (f *TrackerCMD) trackTime(filePath string, tick time.Duration) {
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
		switch report.ReportLength(f.Report) {
		case report.TodayReport:
			printStats(envelope.History[foundIndex])
		case report.LastWeek:
			start := len(envelope.History) - 7
			if start < 0 {
				start = 0
			}

			f.printFilterBy(envelope.History[start:], currentTime)

		case report.LastMonth:
			start := len(envelope.History) - 30

			if start < 0 {
				start = 0
			}

			f.printFilterBy(envelope.History[start:], currentTime)

		case report.LastYear:
			start := len(envelope.History) - 365

			if start < 0 {
				start = 0
			}

			printStatsTotal(envelope.History[start:], currentTime)

		case report.AllTime:
			start := 0

			printStatsTotal(envelope.History[start:], currentTime)
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

func (f *TrackerCMD) printFilterBy(data []TrackingData, cur time.Time) {
	if report.FilterFormat(f.FilterBy) == report.ListTime {
		for _, v := range data {
			printStats(v)
		}

		return
	}

	printStatsTotal(data, cur)
}

func printStats(data TrackingData) {
	hours := int(data.ElapsedTime.Hours())
	minutes := int(data.ElapsedTime.Minutes()) % 60
	seconds := int(data.ElapsedTime.Seconds()) % 60
	fmt.Printf("Total machine usage (%s): %dh %dm %ds\n", data.Date, hours, minutes, seconds)
}

func printStatsTotal(data []TrackingData, curTime time.Time) {
	var days, hours, minutes, seconds int

	parsedTime, err := time.Parse(time.DateOnly, data[len(data)-1].Date)

	if err == nil {
		days += daysBetween(curTime, parsedTime)
	}

	for _, v := range data {
		hours += int(v.ElapsedTime.Hours())
		minutes += int(v.ElapsedTime.Minutes())
		seconds += int(v.ElapsedTime.Seconds())
	}

	fmt.Printf("Total machine usage: %dd %dh %dm %ds\n", days, hours, minutes, seconds)
}

func daysBetween(a, b time.Time) int {
	// Truncate both times to midnight in their respective locations
	aMidnight := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, a.Location())
	bMidnight := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, b.Location())

	// Subtraction here returns a duration, which we divide by 24 hours
	// We add 0.5 to handle potential 23 or 25-hour DST days cleanly
	hours := bMidnight.Sub(aMidnight).Hours()
	return int(hours/24 + 0.5)
}
