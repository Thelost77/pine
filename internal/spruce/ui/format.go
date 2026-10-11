package ui

import "fmt"

// FormatTimestamp formats seconds as H:MM:SS or M:SS.
func FormatTimestamp(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	min := (total % 3600) / 60
	sec := total % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, min, sec)
	}
	return fmt.Sprintf("%d:%02d", min, sec)
}

// FormatDuration formats seconds into a human-readable string like "2h 30m" or "45m".
func FormatDuration(seconds float64) string {
	total := int(seconds)
	h := total / 3600
	m := (total % 3600) / 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}
