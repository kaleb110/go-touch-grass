// Package report defines report types and renders usage summaries.
package report

import (
	"fmt"
	"time"

	"github.com/kaleb110/go-touch-grass/store"
)

// Length selects the date range of a report.
type Length string

const (
	Today Length = "today"
	Week  Length = "lastWeek"
	Month Length = "lastMonth"
	Year  Length = "lastYear"
	All   Length = "allTime"
)

// Filter selects the presentation style of a multi-day report.
type Filter string

const (
	FilterTotal Filter = "total" // one aggregated line
	FilterList  Filter = "list"  // one line per day
)

// ParseLength returns the Length for s, or "" if it is not a known value.
func ParseLength(s string) Length {
	return Length(s)
}

// ParseFilter returns the Filter for s, or "" if it is not a known value.
func ParseFilter(s string) Filter {
	return Filter(s)
}

// Valid reports whether l is a recognized length.
func (l Length) Valid() bool {
	switch l {
	case Today, Week, Month, Year, All:
		return true
	}
	return false
}

// Valid reports whether f is a recognized filter.
func (f Filter) Valid() bool {
	switch f {
	case FilterTotal, FilterList:
		return true
	}
	return false
}

// Range returns the slice of history covered by the given length. Days that
// don't exist yet are simply omitted (the slice is clamped to >= 0).
func Range(history []store.TrackingData, l Length) []store.TrackingData {
	n := len(history)
	var start int
	switch l {
	case Week:
		start = clampStart(n - 7)
	case Month:
		start = clampStart(n - 30)
	case Year:
		start = clampStart(n - 365)
	case All:
		start = 0
	default:
		start = 0
	}
	return history[start:]
}

func clampStart(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

// PrintDay prints a single day's usage line.
func PrintDay(d store.TrackingData) {
	hours := int(d.ElapsedTime.Hours())
	minutes := int(d.ElapsedTime.Minutes()) % 60
	seconds := int(d.ElapsedTime.Seconds()) % 60
	fmt.Printf("Total machine usage (%s): %dh %dm %ds\n", d.Date, hours, minutes, seconds)
}

// PrintTotal prints an aggregated total across the given records, including
// a span in days from the last record to now.
func PrintTotal(data []store.TrackingData, now time.Time) {
	var total time.Duration
	for _, v := range data {
		total += v.ElapsedTime
	}

	days := 0
	if len(data) > 0 {
		if last, err := time.Parse(time.DateOnly, data[len(data)-1].Date); err == nil {
			days = DaysBetween(now, last)
		}
	}

	totalSeconds := int(total.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60
	fmt.Printf("Total machine usage: %dd %dh %dm %ds\n", days, hours, minutes, seconds)
}

// PrintMulti renders a multi-day range according to the filter style.
func PrintMulti(data []store.TrackingData, f Filter, now time.Time) {
	if f == FilterList {
		for _, v := range data {
			PrintDay(v)
		}
		return
	}
	PrintTotal(data, now)
}

// DaysBetween returns the number of whole days between a and b.
func DaysBetween(a, b time.Time) int {
	aMid := time.Date(a.Year(), a.Month(), a.Day(), 0, 0, 0, 0, a.Location())
	bMid := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, b.Location())
	hours := bMid.Sub(aMid).Hours()
	return int(hours/24 + 0.5)
}
