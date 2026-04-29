// Package settings owns the tenant-scoped configuration store backed
// by the tenant_settings table (migration 000078).
//
// The package intentionally does not reach for sqlc — there is one
// table, two query shapes, and a JSONB column whose payload schema
// belongs to the application, not Postgres. Direct database.TenantScoped
// queries keep the surface small and the wire format obvious.
package settings

import "strings"

// AssetLifespanConfig is the per-asset-type expected useful life in
// years that drives the RUL ("remaining useful life") and upgrade-
// recommendation calculations. Values are deliberately int64 to match
// the type historically used in impl_prediction_upgrades.go.
//
// All fields are optional in the wire format; missing fields fall back
// to DefaultAssetLifespan when GetForType is invoked. This means the
// PUT handler can accept partial updates without losing unset keys.
type AssetLifespanConfig struct {
	Server  int64 `json:"server,omitempty"`
	Network int64 `json:"network,omitempty"`
	Storage int64 `json:"storage,omitempty"`
	Power   int64 `json:"power,omitempty"`
}

// MinLifespanYears and MaxLifespanYears bracket the values we accept
// from clients. These are the same bounds the validator enforces so
// callers (and tests) can reference them by name.
const (
	MinLifespanYears int64 = 1
	MaxLifespanYears int64 = 30

	// DefaultLifespanFallbackYears is the value GetForType returns for
	// an asset type that does not match any of the four supported
	// categories. Mirrors the int64(5) fallback that
	// impl_prediction_upgrades.go used pre-W3.2.
	DefaultLifespanFallbackYears int64 = 5
)

// DefaultAssetLifespan returns the canonical defaults — exactly the
// values the hardcoded map in impl_prediction_upgrades.go used to
// hold. A tenant that never opens the settings page sees identical
// behaviour to the pre-W3.2 build.
func DefaultAssetLifespan() AssetLifespanConfig {
	return AssetLifespanConfig{
		Server:  5,
		Network: 7,
		Storage: 5,
		Power:   10,
	}
}

// GetForType returns the lifespan in years for an asset type string.
//
// Matching mirrors the substring-based lookup the original code used
// (e.g. "rack_server" still matches "server"), so swapping the
// hardcoded map for this call is behaviour-preserving by default.
//
// Resolution order, per field:
//  1. value from c (if > 0)
//  2. value from DefaultAssetLifespan (if > 0)
//  3. DefaultLifespanFallbackYears (5)
//
// Step 2 lets a tenant who only customises "server" still get sensible
// values for the other types instead of the global fallback.
func (c AssetLifespanConfig) GetForType(assetType string) int64 {
	t := strings.ToLower(assetType)
	defaults := DefaultAssetLifespan()

	pick := func(custom, def int64) int64 {
		if custom > 0 {
			return custom
		}
		if def > 0 {
			return def
		}
		return DefaultLifespanFallbackYears
	}

	switch {
	case strings.Contains(t, "server"):
		return pick(c.Server, defaults.Server)
	case strings.Contains(t, "network"):
		return pick(c.Network, defaults.Network)
	case strings.Contains(t, "storage"):
		return pick(c.Storage, defaults.Storage)
	case strings.Contains(t, "power"):
		return pick(c.Power, defaults.Power)
	default:
		return DefaultLifespanFallbackYears
	}
}

// MergedWithDefaults returns a copy of c where any zero-valued field
// has been replaced by the canonical default. Useful for the GET
// handler so the API always returns a complete object regardless of
// what is stored in the JSONB column.
func (c AssetLifespanConfig) MergedWithDefaults() AssetLifespanConfig {
	defaults := DefaultAssetLifespan()
	out := c
	if out.Server <= 0 {
		out.Server = defaults.Server
	}
	if out.Network <= 0 {
		out.Network = defaults.Network
	}
	if out.Storage <= 0 {
		out.Storage = defaults.Storage
	}
	if out.Power <= 0 {
		out.Power = defaults.Power
	}
	return out
}

// Validate enforces the [MinLifespanYears, MaxLifespanYears] range on
// every populated field. A zero value is treated as "use default" and
// is allowed; the bound check applies once a caller commits to a
// specific value. Returns the offending field name and a human-readable
// reason for the first violation, or empty strings if the config is OK.
func (c AssetLifespanConfig) Validate() (string, string) {
	check := func(name string, v int64) (string, string) {
		if v == 0 {
			return "", ""
		}
		if v < MinLifespanYears || v > MaxLifespanYears {
			return name, "must be between 1 and 30 years"
		}
		return "", ""
	}
	for _, pair := range [...]struct {
		name string
		v    int64
	}{
		{"server", c.Server},
		{"network", c.Network},
		{"storage", c.Storage},
		{"power", c.Power},
	} {
		if field, reason := check(pair.name, pair.v); field != "" {
			return field, reason
		}
	}
	return "", ""
}

// Settings is the full per-tenant settings object. New top-level
// settings groups are added as additional fields here and as
// additional keys under settingsJSON in service.go.
type Settings struct {
	AssetLifespan AssetLifespanConfig `json:"asset_lifespan_config"`
}

// Defaults returns a Settings populated entirely with code defaults.
// Used by the service layer when a tenant has no row yet.
func Defaults() Settings {
	return Settings{
		AssetLifespan: DefaultAssetLifespan(),
	}
}
