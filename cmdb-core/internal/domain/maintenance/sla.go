package maintenance

import "time"

// SLA durations by priority level.
var slaDurations = map[string]time.Duration{
	"critical": 4 * time.Hour,
	"high":     8 * time.Hour,
	"medium":   24 * time.Hour,
	"low":      72 * time.Hour,
}

// SLADeadline returns the SLA deadline given a priority and approval time.
func SLADeadline(priority string, approvedAt time.Time) time.Time {
	d, ok := slaDurations[priority]
	if !ok {
		d = 24 * time.Hour // default to medium
	}
	return approvedAt.Add(d)
}

// SLAWarningThreshold returns 75% of the SLA duration.
func SLAWarningThreshold(priority string) time.Duration {
	d, ok := slaDurations[priority]
	if !ok {
		d = 24 * time.Hour
	}
	return time.Duration(float64(d) * 0.75)
}
