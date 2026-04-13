package maintenance

import (
	"time"

	"github.com/google/uuid"
)

// TransitionRequest is the payload for transitioning a work order's status.
type TransitionRequest struct {
	Status  string `json:"status" binding:"required"`
	Comment string `json:"comment"`
}

// CreateOrderRequest is the payload for creating a new work order.
type CreateOrderRequest struct {
	Title          string     `json:"title" binding:"required"`
	Type           string     `json:"type" binding:"required"`
	Priority       string     `json:"priority"`
	LocationID     *uuid.UUID `json:"location_id"`
	AssetID        *uuid.UUID `json:"asset_id"`
	AssigneeID     *uuid.UUID `json:"assignee_id"`
	Description    string     `json:"description"`
	Reason         string     `json:"reason"`
	ScheduledStart *time.Time `json:"scheduled_start"`
	ScheduledEnd   *time.Time `json:"scheduled_end"`
}
