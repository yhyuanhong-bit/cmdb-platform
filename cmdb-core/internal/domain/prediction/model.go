package prediction

import "github.com/google/uuid"

// CreateRCARequest carries the input for creating a root-cause analysis.
type CreateRCARequest struct {
	IncidentID uuid.UUID `json:"incident_id" binding:"required"`
	ModelName  string    `json:"model_name"`
	Context    string    `json:"context"`
}

// VerifyRCARequest carries the input for verifying an RCA.
type VerifyRCARequest struct {
	VerifiedBy uuid.UUID `json:"verified_by" binding:"required"`
}
