package settings

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/cmdb-platform/cmdb-core/internal/platform/database"
)

// Service owns reads and writes to tenant_settings. The struct holds
// only a pool — every call routes through database.Scope so the
// tenant guard (and tenantlint) catch missing tenant filters at
// compile/lint time.
type Service struct {
	pool *pgxpool.Pool
}

// NewService constructs a Service. The pool MUST be non-nil — a nil
// pool indicates a wiring bug in main.go and we want it to crash
// immediately rather than silently no-op every settings call.
func NewService(pool *pgxpool.Pool) *Service {
	if pool == nil {
		panic("settings.NewService: nil pool")
	}
	return &Service{pool: pool}
}

// ErrInvalidLifespan is returned when a caller submits an out-of-range
// lifespan value. The handler maps it to HTTP 400.
var ErrInvalidLifespan = errors.New("settings: invalid lifespan value")

// Get returns the merged Settings for a tenant. If the tenant has no
// row yet, it returns Defaults() with no error — the caller does not
// need to distinguish "row missing" from "all defaults".
//
// The merge layer means partial customisations (e.g. only server
// lifespan set) come back filled in for the unset fields, so the
// frontend always receives a complete object.
func (s *Service) Get(ctx context.Context, tenantID uuid.UUID) (Settings, error) {
	if tenantID == uuid.Nil {
		return Settings{}, fmt.Errorf("settings.Get: tenant_id is required")
	}

	sc := database.Scope(s.pool, tenantID)

	var raw []byte
	err := sc.QueryRow(ctx, `
		SELECT settings
		FROM tenant_settings
		WHERE tenant_id = $1
	`).Scan(&raw)

	if errors.Is(err, pgx.ErrNoRows) {
		return Defaults(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("settings.Get: query: %w", err)
	}

	out := Defaults()
	if len(raw) > 0 {
		// Decode whatever is stored, then overlay it on top of
		// Defaults so missing keys remain populated.
		var stored Settings
		if err := json.Unmarshal(raw, &stored); err != nil {
			// Treat a corrupt blob as "all defaults". Logging would
			// belong here in a follow-up — the surface is small
			// enough that a panic-free degrade is the right tradeoff.
			return Defaults(), nil
		}
		merged := stored.AssetLifespan.MergedWithDefaults()
		out.AssetLifespan = merged
	}
	return out, nil
}

// UpdateAssetLifespan upserts the asset_lifespan_config block of a
// tenant's settings. Other top-level keys in the JSONB blob are
// preserved via jsonb_set. Returns ErrInvalidLifespan if any value
// is out of range.
//
// updatedBy may be uuid.Nil for system-driven writes (background
// migrations, fixture loaders); the column is nullable on purpose.
func (s *Service) UpdateAssetLifespan(
	ctx context.Context,
	tenantID uuid.UUID,
	cfg AssetLifespanConfig,
	updatedBy uuid.UUID,
) error {
	if tenantID == uuid.Nil {
		return fmt.Errorf("settings.UpdateAssetLifespan: tenant_id is required")
	}
	if field, reason := cfg.Validate(); field != "" {
		return fmt.Errorf("%w: %s %s", ErrInvalidLifespan, field, reason)
	}

	// Strip zero-valued fields before persisting. The merge logic in
	// Get() back-fills defaults; we don't want to bake the current
	// defaults into the row because that would freeze the tenant on
	// today's defaults forever. Storing only what the user actually
	// changed is also a clean migration path if defaults shift later.
	persisted := map[string]int64{}
	if cfg.Server > 0 {
		persisted["server"] = cfg.Server
	}
	if cfg.Network > 0 {
		persisted["network"] = cfg.Network
	}
	if cfg.Storage > 0 {
		persisted["storage"] = cfg.Storage
	}
	if cfg.Power > 0 {
		persisted["power"] = cfg.Power
	}

	payload, err := json.Marshal(persisted)
	if err != nil {
		return fmt.Errorf("settings.UpdateAssetLifespan: marshal: %w", err)
	}

	sc := database.Scope(s.pool, tenantID)

	// updated_by is passed as $3 and may be the zero UUID. We translate
	// uuid.Nil to NULL via NULLIF to honour the FK (no synthetic user).
	_, err = sc.Exec(ctx, `
		INSERT INTO tenant_settings (tenant_id, settings, updated_at, updated_by)
		VALUES ($1, jsonb_build_object('asset_lifespan_config', $2::jsonb), now(), NULLIF($3, '00000000-0000-0000-0000-000000000000'::uuid))
		ON CONFLICT (tenant_id) DO UPDATE SET
			settings   = jsonb_set(
				COALESCE(tenant_settings.settings, '{}'::jsonb),
				'{asset_lifespan_config}',
				$2::jsonb,
				true
			),
			updated_at = now(),
			updated_by = NULLIF($3, '00000000-0000-0000-0000-000000000000'::uuid)
		WHERE tenant_settings.tenant_id = $1
	`, payload, updatedBy)
	if err != nil {
		return fmt.Errorf("settings.UpdateAssetLifespan: upsert: %w", err)
	}
	return nil
}

// GetAssetLifespan is a convenience wrapper used by call sites that
// only need the lifespan config (e.g. the prediction RUL handler).
// It always returns a fully-merged config — no zero values.
func (s *Service) GetAssetLifespan(ctx context.Context, tenantID uuid.UUID) (AssetLifespanConfig, error) {
	full, err := s.Get(ctx, tenantID)
	if err != nil {
		return AssetLifespanConfig{}, err
	}
	return full.AssetLifespan, nil
}
