package integration

import (
	"context"
	"fmt"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/google/uuid"
)

// Service provides integration adapter and webhook operations.
type Service struct {
	queries *dbgen.Queries
}

// NewService creates a new integration Service.
func NewService(queries *dbgen.Queries) *Service {
	return &Service{queries: queries}
}

// ListAdapters returns all integration adapters for a tenant.
func (s *Service) ListAdapters(ctx context.Context, tenantID uuid.UUID) ([]dbgen.IntegrationAdapter, error) {
	adapters, err := s.queries.ListAdapters(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list adapters: %w", err)
	}
	return adapters, nil
}

// CreateAdapter creates a new integration adapter.
func (s *Service) CreateAdapter(ctx context.Context, params dbgen.CreateAdapterParams) (dbgen.IntegrationAdapter, error) {
	adapter, err := s.queries.CreateAdapter(ctx, params)
	if err != nil {
		return dbgen.IntegrationAdapter{}, fmt.Errorf("create adapter: %w", err)
	}
	return adapter, nil
}

// GetAdapterByID returns an adapter scoped to the given tenant.
func (s *Service) GetAdapterByID(ctx context.Context, id, tenantID uuid.UUID) (dbgen.IntegrationAdapter, error) {
	adapter, err := s.queries.GetAdapterByID(ctx, dbgen.GetAdapterByIDParams{ID: id, TenantID: tenantID})
	if err != nil {
		return dbgen.IntegrationAdapter{}, fmt.Errorf("get adapter: %w", err)
	}
	return adapter, nil
}

// UpdateAdapter applies a partial update to an integration adapter.
// Tenant scoping is enforced inside the SQL query via the TenantID param.
func (s *Service) UpdateAdapter(ctx context.Context, params dbgen.UpdateAdapterParams) (dbgen.IntegrationAdapter, error) {
	adapter, err := s.queries.UpdateAdapter(ctx, params)
	if err != nil {
		return dbgen.IntegrationAdapter{}, fmt.Errorf("update adapter: %w", err)
	}
	return adapter, nil
}

// DeleteAdapter removes an adapter scoped to the given tenant.
func (s *Service) DeleteAdapter(ctx context.Context, id, tenantID uuid.UUID) error {
	if err := s.queries.DeleteAdapter(ctx, dbgen.DeleteAdapterParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("delete adapter: %w", err)
	}
	return nil
}

// ListWebhooks returns all webhook subscriptions for a tenant.
func (s *Service) ListWebhooks(ctx context.Context, tenantID uuid.UUID) ([]dbgen.WebhookSubscription, error) {
	webhooks, err := s.queries.ListWebhooks(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("list webhooks: %w", err)
	}
	return webhooks, nil
}

// CreateWebhook creates a new webhook subscription.
func (s *Service) CreateWebhook(ctx context.Context, params dbgen.CreateWebhookParams) (dbgen.WebhookSubscription, error) {
	webhook, err := s.queries.CreateWebhook(ctx, params)
	if err != nil {
		return dbgen.WebhookSubscription{}, fmt.Errorf("create webhook: %w", err)
	}
	return webhook, nil
}

// UpdateWebhook applies a partial update to a webhook subscription.
// Tenant scoping is enforced inside the SQL query via the TenantID param.
func (s *Service) UpdateWebhook(ctx context.Context, params dbgen.UpdateWebhookParams) (dbgen.WebhookSubscription, error) {
	webhook, err := s.queries.UpdateWebhook(ctx, params)
	if err != nil {
		return dbgen.WebhookSubscription{}, fmt.Errorf("update webhook: %w", err)
	}
	return webhook, nil
}

// DeleteWebhook removes a webhook subscription scoped to the given tenant.
func (s *Service) DeleteWebhook(ctx context.Context, id, tenantID uuid.UUID) error {
	if err := s.queries.DeleteWebhook(ctx, dbgen.DeleteWebhookParams{ID: id, TenantID: tenantID}); err != nil {
		return fmt.Errorf("delete webhook: %w", err)
	}
	return nil
}

// GetWebhookByID returns a webhook scoped to the given tenant.
func (s *Service) GetWebhookByID(ctx context.Context, id, tenantID uuid.UUID) (dbgen.WebhookSubscription, error) {
	webhook, err := s.queries.GetWebhookByID(ctx, dbgen.GetWebhookByIDParams{ID: id, TenantID: tenantID})
	if err != nil {
		return dbgen.WebhookSubscription{}, fmt.Errorf("get webhook: %w", err)
	}
	return webhook, nil
}

// ListDeliveries returns recent webhook deliveries for a subscription.
func (s *Service) ListDeliveries(ctx context.Context, webhookID uuid.UUID, limit int) ([]dbgen.WebhookDelivery, error) {
	deliveries, err := s.queries.ListDeliveries(ctx, dbgen.ListDeliveriesParams{
		SubscriptionID: webhookID,
		Limit:          int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list deliveries: %w", err)
	}
	return deliveries, nil
}
