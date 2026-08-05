// Package store handles loading, migrating, and persisting tracking state.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultStateFile is the relative state path under the user's home directory.
	DefaultStateFile = ".local/share/go-touch-grass/state.json"
	// DefaultGlobalGoal is the fallback daily goal when none is configured.
	DefaultGlobalGoal = 3 * time.Hour
	// SchemaVersion is the on-disk JSON format version.
	SchemaVersion = 1
)

// TrackingData is a single day's usage record.
type TrackingData struct {
	Date        string        `json:"date"`
	ElapsedTime time.Duration `json:"elapsed_time"`
	DailyGoal   time.Duration `json:"daily_goal,omitempty"`
}

// Envelope wraps the persisted state with schema metadata.
type Envelope struct {
	SchemaVersion int            `json:"schema_version"`
	GlobalGoal    time.Duration  `json:"global_goal"`
	History       []TrackingData `json:"history"`
}

// Default returns a fresh envelope with sensible defaults.
func Default() *Envelope {
	return &Envelope{
		SchemaVersion: SchemaVersion,
		History:       []TrackingData{},
		GlobalGoal:    DefaultGlobalGoal,
	}
}

// ResolvePath turns a user-supplied path into an absolute one.
// The built-in default is resolved under the home directory unless
// APP_ENV=development, in which case it is resolved under the working
// directory (useful for local testing).
func ResolvePath(stateFile string) (string, error) {
	if stateFile != DefaultStateFile {
		return stateFile, nil
	}
	if os.Getenv("APP_ENV") == "development" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(wd, "test_state.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("error finding home dir: %w", err)
	}
	return filepath.Join(home, stateFile), nil
}

// EnsureDir creates the parent directory of path if it does not exist.
func EnsureDir(path string) error {
	return os.MkdirAll(filepath.Dir(path), 0755)
}

// Load reads the state file and migrates legacy formats. If the file does
// not exist, a fresh default envelope is returned (no error).
func Load(path string) (*Envelope, error) {
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var env Envelope
	if err := json.Unmarshal(fileBytes, &env); err == nil && env.SchemaVersion > 0 {
		if env.GlobalGoal == 0 {
			env.GlobalGoal = DefaultGlobalGoal
		}
		if env.History == nil {
			env.History = []TrackingData{}
		}
		return &env, nil
	}

	// Legacy format: bare []TrackingData array (pre-schema).
	var legacy []TrackingData
	if err := json.Unmarshal(fileBytes, &legacy); err == nil {
		fmt.Println("Migrating legacy state file to schema v1...")
		return &Envelope{
			SchemaVersion: SchemaVersion,
			History:       legacy,
			GlobalGoal:    DefaultGlobalGoal,
		}, nil
	}

	return nil, errors.New("unknown file format or corrupted state file")
}

// Save writes the envelope as pretty-printed JSON. The write is atomic: it
// goes to a temp file which is then renamed over the target, so a crash mid
// write never leaves a truncated state file.
func Save(path string, env *Envelope) error {
	data, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return fmt.Errorf("marshalling state: %w", err)
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".gtg-state-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op if rename succeeded

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpName, 0644); err != nil {
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}

// FindToday returns the index of today's record, or -1 if none exists yet.
func (e *Envelope) FindToday(today string) int {
	for i, record := range e.History {
		if record.Date == today {
			return i
		}
	}
	return -1
}
