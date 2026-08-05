// Package tui renders the terminal dashboard for go-touch-grass.
package tui

import (
	"fmt"
	"strings"
	"time"
)

const (
	BarWidth = 20

	ColorGreen = "\033[32m"
	ColorRed   = "\033[31m"
	ColorBold  = "\033[1;36m"
	ColorDim   = "\033[2m"
	ColorReset = "\033[0m"

	FullBlock  = "█"
	LightBlock = "░"
)

// RenderBar returns a progress bar showing elapsed against goal.
func RenderBar(elapsed, goal time.Duration) string {
	pct := 0.0
	if goal > 0 {
		pct = float64(elapsed) / float64(goal)
	}
	if pct > 1.0 {
		pct = 1.0
	}
	if pct < 0.0 {
		pct = 0.0
	}

	filled := int(pct * BarWidth)
	bar := ColorGreen + strings.Repeat(FullBlock, filled) +
		ColorRed + strings.Repeat(LightBlock, BarWidth-filled) +
		ColorReset

	return fmt.Sprintf("  🌱 [%s] %s%.1f%%%s", bar, ColorBold, pct*100, ColorReset)
}

// RenderStats returns the usage summary lines beneath the bar.
func RenderStats(elapsed, goal time.Duration) string {
	lines := []struct{ label, value string }{
		{"Usage", fmt.Sprintf("%v / %v", elapsed, goal)},
		{"Goal", fmt.Sprintf("%v", goal)},
	}

	var b strings.Builder
	for _, l := range lines {
		b.WriteString(fmt.Sprintf("  %-8s %s\n", l.label+":", l.value))
	}
	return b.String()
}

// RenderDashboard prints the full dashboard: header, bar, and stats.
func RenderDashboard(username, host string, elapsed, goal time.Duration) {
	fmt.Println()
	fmt.Printf("  %s%s@%s%s\n", ColorBold, strings.ToUpper(username), host, ColorReset)
	fmt.Printf("  %s%s%s\n", ColorDim, strings.Repeat("─", 32), ColorReset)
	fmt.Println(RenderBar(elapsed, goal))
	fmt.Printf("  %s%s%s\n", ColorDim, strings.Repeat("─", 32), ColorReset)
	fmt.Println(RenderStats(elapsed, goal))
}
