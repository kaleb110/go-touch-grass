package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"
	"time"

	"github.com/kaleb110/go-touch-grass/report"
	"github.com/kaleb110/go-touch-grass/store"
	"github.com/kaleb110/go-touch-grass/tui"
)

// TrackerCMD implements the `tracker` subcommand.
type TrackerCMD struct {
	stateFile    string
	tickInterval int
	reportType   string
	filterBy     string
	update       bool
	globalGoal   *int // nil = unset
	dailyGoal    int
	simulateNext bool
	showVersion  bool
}

func NewTracker() *TrackerCMD {
	return &TrackerCMD{}
}

func (tc *TrackerCMD) Name() string        { return "tracker" }
func (tc *TrackerCMD) Description() string { return "track machine usage" }

func (tc *TrackerCMD) Init(args []string) error {
	fs := flag.NewFlagSet(tc.Name(), flag.ExitOnError)

	fs.StringVar(&tc.stateFile, "state", store.DefaultStateFile, "state file path (JSON)")
	fs.IntVar(&tc.tickInterval, "tick", 60, "ticker interval in seconds")
	fs.BoolVar(&tc.update, "update", false, "run as the daemon updater")
	fs.StringVar(&tc.reportType, "report", "", "report range: today|lastWeek|lastMonth|lastYear|allTime")
	fs.StringVar(&tc.filterBy, "filter-by", string(report.FilterList), "filter for multi-day reports: total|list")
	var globalGoalVal int
	fs.IntVar(&globalGoalVal, "global-goal", 0, "global goal in hours (e.g. 3)")
	fs.IntVar(&tc.dailyGoal, "daily-goal", 0, "set today's goal in hours (e.g. 5)")
	fs.BoolVar(&tc.simulateNext, "sim-tomorrow", false, "simulate tomorrow's date for testing rollover")
	fs.BoolVar(&tc.showVersion, "version", false, "print current binary version")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: go-touch-grass %s [flags]\n\nFlags:\n", tc.Name())
		fs.PrintDefaults()
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	fs.Visit(func(f *flag.Flag) {
		if f.Name == "global-goal" && globalGoalVal > 0 {
			v := globalGoalVal
			tc.globalGoal = &v
		}
	})

	return tc.validate()
}

func (tc *TrackerCMD) validate() error {
	if tc.stateFile == "" {
		return errors.New("empty file path")
	}
	if tc.tickInterval <= 0 {
		return errors.New("tick interval must be greater than 0")
	}
	if tc.globalGoal != nil && *tc.globalGoal <= 0 {
		return errors.New("global goal must be greater than 0")
	}
	if tc.reportType != "" && !report.ParseLength(tc.reportType).Valid() {
		return fmt.Errorf("invalid report type: %s", tc.reportType)
	}
	if !report.ParseFilter(tc.filterBy).Valid() {
		return fmt.Errorf("invalid filter format: %s", tc.filterBy)
	}
	return nil
}

func (tc *TrackerCMD) Run() error {
	if tc.showVersion {
		fmt.Printf("go-touch-grass version %s (Schema v%d)\n", AppVersion, store.SchemaVersion)
		return nil
	}

	path, err := store.ResolvePath(tc.stateFile)
	if err != nil {
		return err
	}
	if err := store.EnsureDir(path); err != nil {
		return fmt.Errorf("error creating storage dir: %w", err)
	}

	if !tc.update {
		tc.tickOnce(path, 0)
		return nil
	}

	return tc.runDaemon(path)
}

// runDaemon is the long-running service loop.
func (tc *TrackerCMD) runDaemon(path string) error {
	fmt.Printf("Machine usage tracker started... [Store: %s]\n", path)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	interval := time.Duration(tc.tickInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tc.tickOnce(path, interval)
		case <-stop:
			fmt.Println("\nShutting down tracker cleanly.")
			tc.tickOnce(path, interval)
			return nil
		}
	}
}

// tickOnce performs one load → mutate → save cycle. When delta is non-zero
// (daemon mode) it adds elapsed time; when delta is zero (display mode) it
// only reads and renders.
func (tc *TrackerCMD) tickOnce(path string, delta time.Duration) {
	now := time.Now()
	if tc.simulateNext {
		now = now.AddDate(0, 0, 1)
	}
	today := now.Format(time.DateOnly)

	env, err := store.Load(path)
	if err != nil {
		fmt.Printf("State error: %v — continuing with fresh slate.\n", err)
		env = store.Default()
	}

	// Apply a live global-goal override.
	if tc.globalGoal != nil {
		target := time.Duration(*tc.globalGoal) * time.Hour
		if env.GlobalGoal != target {
			env.GlobalGoal = target
			_ = store.Save(path, env)
		}
	}

	idx := env.FindToday(today)

	// Daemon mode: accumulate time and persist.
	if tc.update {
		tc.recordTick(env, path, idx, today, delta)
		return
	}

	// Display mode.
	tc.render(env, idx, today, now)
}

func (tc *TrackerCMD) recordTick(env *store.Envelope, path string, idx int, today string, delta time.Duration) {
	goalDur := time.Duration(tc.dailyGoal) * time.Hour
	if idx >= 0 {
		env.History[idx].ElapsedTime += delta
		if goalDur > 0 {
			env.History[idx].DailyGoal = goalDur
		}
		report.PrintDay(env.History[idx])
	} else {
		fmt.Printf("New day detected (%s). Starting new tracking record.\n", today)
		rec := store.TrackingData{Date: today, ElapsedTime: delta, DailyGoal: goalDur}
		env.History = append(env.History, rec)
		report.PrintDay(rec)
	}
	if err := store.Save(path, env); err != nil {
		fmt.Printf("failed to save state: %v\n", err)
	}
}

func (tc *TrackerCMD) render(env *store.Envelope, idx int, today string, now time.Time) {
	var tracked store.TrackingData
	if idx >= 0 {
		tracked = env.History[idx]
	} else {
		tracked = store.TrackingData{Date: today, DailyGoal: time.Duration(tc.dailyGoal) * time.Hour}
	}

	switch report.ParseLength(tc.reportType) {
	case report.Today:
		report.PrintDay(tracked)
	case report.Week, report.Month:
		sub := report.Range(env.History, report.ParseLength(tc.reportType))
		report.PrintMulti(sub, report.ParseFilter(tc.filterBy), now)
	case report.Year, report.All:
		sub := report.Range(env.History, report.ParseLength(tc.reportType))
		report.PrintTotal(sub, now)
	default:
		elapsed := tracked.ElapsedTime
		goal := tc.targetGoal(tracked, env)
		username, host := hostInfo()
		tui.RenderDashboard(username, host, elapsed, goal)
	}
}

// targetGoal resolves the effective goal in priority order:
// runtime flags → per-day record → persisted global → default.
func (tc *TrackerCMD) targetGoal(record store.TrackingData, env *store.Envelope) time.Duration {
	if tc.dailyGoal > 0 {
		return time.Duration(tc.dailyGoal) * time.Hour
	}
	if tc.globalGoal != nil {
		return time.Duration(*tc.globalGoal) * time.Hour
	}
	if record.DailyGoal > 0 {
		return record.DailyGoal
	}
	if env.GlobalGoal > 0 {
		return env.GlobalGoal
	}
	return store.DefaultGlobalGoal
}

func hostInfo() (username, host string) {
	host = "localhost"
	if h, err := os.Hostname(); err == nil {
		host = h
	}
	username = "user"
	if u, err := user.Current(); err == nil {
		username = u.Username
	}
	return username, host
}

// installPaths returns the conventional user-local install locations used by
// the install command and by the generated unit file.
// installPaths returns the conventional user-local install locations used by
// the install command and by the generated unit file. name is the installed
// binary name (and systemd unit stem), defaulting to "go-touch-grass".
func installPaths(name string) (binDir, binPath, unitPath string, err error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", fmt.Errorf("finding home dir: %w", err)
	}
	binDir = filepath.Join(home, ".local", "bin")
	binPath = filepath.Join(binDir, name)
	unitDir := filepath.Join(home, ".config", "systemd", "user")
	unitPath = filepath.Join(unitDir, name+".service")
	return binDir, binPath, unitPath, nil
}
