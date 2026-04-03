package maintenance

import "fmt"

// validTransitions defines the allowed status transitions for work orders.
var validTransitions = map[string][]string{
	"draft":       {"pending"},
	"pending":     {"approved", "rejected"},
	"approved":    {"in_progress"},
	"in_progress": {"completed"},
	"completed":   {"closed"},
}

// ValidateTransition checks whether a transition from one status to another is allowed.
func ValidateTransition(from, to string) error {
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("unknown status %q", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid transition from %q to %q", from, to)
}
