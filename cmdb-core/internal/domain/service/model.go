// Package service implements the Business Service entity — user-visible
// business functions (e.g. "Order API", "Payment Gateway") and their
// N:M membership of underlying CIs.
//
// Spec: cmdb-core/db/specs/services.md (Approved 2026-04-23).
package service

import (
	"regexp"

	"github.com/google/uuid"
)

// codePattern mirrors the DB CHECK constraint (migration 000063). We
// validate at the service layer too so callers get a well-typed error
// before the DB rejects the insert, matching the Q1 sign-off.
var codePattern = regexp.MustCompile(`^[A-Z][A-Z0-9_-]{1,63}$`)

// IsValidCode returns true when code satisfies the business-ID format
// agreed in spec Q1: starts with uppercase letter, 2-64 chars of
// [A-Z0-9_-].
func IsValidCode(code string) bool { return codePattern.MatchString(code) }

// Tier values match bia_scoring_rules.tier_name so tier-driven policy
// (alert priority, SLA defaults) stays consistent between BIA and services.
const (
	TierCritical  = "critical"
	TierImportant = "important"
	TierNormal    = "normal"
	TierLow       = "low"
	TierMinor     = "minor" // kept for BIA backfill compatibility
)

// Status lifecycle.
const (
	StatusActive         = "active"
	StatusDeprecated     = "deprecated"
	StatusDecommissioned = "decommissioned"
)

// Roles describe the function of an asset within a service. Q3 sign-off
// locks the 7-value set; additions require a spec revision.
const (
	RolePrimary    = "primary"
	RoleReplica    = "replica"
	RoleCache      = "cache"
	RoleProxy      = "proxy"
	RoleStorage    = "storage"
	RoleDependency = "dependency"
	RoleComponent  = "component"
)

// HealthStatus is the computed aggregate health of a service, derived
// from its critical assets. Only 'healthy' means every critical asset
// is in a live status; any unhealthy critical asset degrades the whole.
type HealthStatus string

const (
	HealthHealthy  HealthStatus = "healthy"
	HealthDegraded HealthStatus = "degraded"
	HealthUnknown  HealthStatus = "unknown" // no critical assets tagged yet
)

// CreateParams is the input shape for Service.Create. Optional fields use
// zero values; required fields are validated by Service.Create.
type CreateParams struct {
	TenantID        uuid.UUID
	Code            string
	Name            string
	Description     string
	Tier            string
	OwnerTeam       string
	BIAAssessmentID *uuid.UUID
	Tags            []string
	CreatedBy       uuid.UUID
}

// UpdateParams uses pointers for partial updates — nil means "don't change".
// Matches the existing asset/service update contract.
type UpdateParams struct {
	TenantID        uuid.UUID
	ID              uuid.UUID
	Name            *string
	Description     *string
	Tier            *string
	OwnerTeam       *string
	BIAAssessmentID *uuid.UUID
	Status          *string
	Tags            *[]string
}
