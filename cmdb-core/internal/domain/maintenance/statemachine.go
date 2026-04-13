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
	StatusRejected:  {StatusSubmitted},
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

// Execution status constants — controlled by Edge nodes.
const (
	ExecPending = "pending"
	ExecWorking = "working"
	ExecDone    = "done"
)

// Governance status constants — controlled by Central.
const (
	GovSubmitted = "submitted"
	GovApproved  = "approved"
	GovRejected  = "rejected"
	GovVerified  = "verified"
)

// validExecTransitions defines allowed execution_status transitions.
var validExecTransitions = map[string][]string{
	ExecPending: {ExecWorking},
	ExecWorking: {ExecDone},
}

// validGovTransitions defines allowed governance_status transitions.
var validGovTransitions = map[string][]string{
	GovSubmitted: {GovApproved, GovRejected},
	GovApproved:  {GovVerified},
	GovRejected:  {GovSubmitted},
}

// DeriveStatus computes the unified status from execution and governance dimensions.
// Tolerates dirty backfill data (gov="in_progress" or "completed" mapped to "approved").
// Priority: verified > rejected > execution mapping.
func DeriveStatus(exec, gov string) (string, error) {
	switch gov {
	case "in_progress", "completed":
		gov = GovApproved
	}

	validExec := exec == ExecPending || exec == ExecWorking || exec == ExecDone
	validGov := gov == GovSubmitted || gov == GovApproved || gov == GovRejected || gov == GovVerified
	if !validExec {
		return "", fmt.Errorf("invalid execution_status %q", exec)
	}
	if !validGov {
		return "", fmt.Errorf("invalid governance_status %q", gov)
	}

	if gov == GovVerified {
		return StatusVerified, nil
	}
	if gov == GovRejected {
		return StatusRejected, nil
	}
	switch exec {
	case ExecPending:
		if gov == GovApproved {
			return StatusApproved, nil
		}
		return StatusSubmitted, nil
	case ExecWorking:
		return StatusInProgress, nil
	case ExecDone:
		return StatusCompleted, nil
	}
	return "", fmt.Errorf("unreachable: exec=%q gov=%q", exec, gov)
}

// ValidateExecTransition checks whether an execution_status transition is allowed.
func ValidateExecTransition(from, to string) error {
	allowed, ok := validExecTransitions[from]
	if !ok {
		return fmt.Errorf("unknown execution_status %q", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid execution transition from %q to %q", from, to)
}

// ValidateGovTransition checks whether a governance_status transition is allowed.
func ValidateGovTransition(from, to string) error {
	allowed, ok := validGovTransitions[from]
	if !ok {
		return fmt.Errorf("unknown governance_status %q", from)
	}
	for _, s := range allowed {
		if s == to {
			return nil
		}
	}
	return fmt.Errorf("invalid governance transition from %q to %q", from, to)
}
