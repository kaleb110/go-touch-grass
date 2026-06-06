// Package report have type enums for our flag
package report

type ReportLength string

const (
	TodayReport ReportLength = "today"
	LastWeek    ReportLength = "lastWeek"
	LastMonth   ReportLength = "lastMonth"
	LastYear    ReportLength = "lastYear"
	AllTime     ReportLength = "allTime"
)

type FilterFormat string

const (
	TotalTime FilterFormat = "total"
	ListTime  FilterFormat = "list"
)
