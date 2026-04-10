package maintenance

import "fmt"

// Status constants for the work order lifecycle.
const (
	StatusSubmitted  = "submitted"
	StatusApproved   = "approved"
	StatusRejected   = "rejected"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusVerified   = "verified"
)

// validTransitions defines the allowed status transitions for work orders.
var validTransitions = map[string][]string{
	StatusSubmitted:  {StatusApproved, StatusRejected},
	StatusApproved:   {StatusInProgress},
	StatusInProgress: {StatusCompleted},
	StatusCompleted:  {StatusVerified},
}

// approvalTransitions are transitions that require approval permissions.
var approvalTransitions = map[string]bool{
	StatusApproved: true,
	StatusRejected: true,
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

// RequiresApproval returns true if the target status requires approval permissions.
func RequiresApproval(toStatus string) bool {
	return approvalTransitions[toStatus]
}
