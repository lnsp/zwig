package utils

import (
	"fmt"
	"time"
)

// HumanTimeFormat converts time into a human readable format.
func HumanTimeFormat(t time.Time) string {
	duration := time.Since(t)
	if duration.Hours() >= 2 {
		return fmt.Sprintf("%d hours ago", int(duration.Hours()))
	} else if duration.Hours() >= 1 {
		return "an hour ago"
	} else if duration.Minutes() >= 2 {
		return fmt.Sprintf("%d minutes ago", int(duration.Minutes()))
	} else if duration.Minutes() >= 1 {
		return "a minute ago"
	} else if duration.Seconds() >= 5 {
		return fmt.Sprintf("%d seconds ago", int(duration.Seconds()))
	}
	return "just now"
}
