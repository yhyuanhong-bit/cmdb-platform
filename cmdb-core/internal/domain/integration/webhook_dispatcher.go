package integration

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// WebhookDispatcher delivers events to registered webhook subscriptions.
type WebhookDispatcher struct {
	queries *dbgen.Queries
	client  *http.Client
}

// NewWebhookDispatcher creates a new dispatcher.
func NewWebhookDispatcher(queries *dbgen.Queries) *WebhookDispatcher {
	return &WebhookDispatcher{
		queries: queries,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// HandleEvent processes an event and delivers it to matching webhook subscriptions.
func (d *WebhookDispatcher) HandleEvent(ctx context.Context, event eventbus.Event) error {
	// Find subscriptions matching this event type (scoped to tenant)
	tenantUUID, _ := uuid.Parse(event.TenantID)
	subs, err := d.queries.ListWebhooksByEvent(ctx, dbgen.ListWebhooksByEventParams{
		TenantID: tenantUUID,
		Column2: event.Subject,
	})
	if err != nil {
		zap.L().Error("failed to list webhooks for event", zap.String("subject", event.Subject), zap.Error(err))
		return nil // don't fail the event pipeline
	}

	for _, sub := range subs {
		sub := sub

		// Check if BIA filtering is needed
		if len(sub.FilterBia) > 0 && sub.FilterBia[0] != "" {
			go func() {
				// Extract asset_id from event payload
				var payload map[string]string
				json.Unmarshal(event.Payload, &payload)
				if assetID, ok := payload["asset_id"]; ok {
					parsed, err := uuid.Parse(assetID)
					if err == nil {
						asset, err := d.queries.GetAsset(ctx, dbgen.GetAssetParams{ID: parsed, TenantID: tenantUUID})
						if err != nil {
							zap.L().Warn("webhook BIA filter: asset lookup failed",
								zap.String("asset_id", assetID), zap.Error(err))
						} else {
							// Check if asset BIA level matches filter
							matched := false
							for _, bia := range sub.FilterBia {
								if bia == asset.BiaLevel {
									matched = true
									break
								}
							}
							if !matched {
								return
							}
						}
					}
				}
				d.deliver(sub, event)
			}()
		} else {
			go d.deliver(sub, event)
		}
	}
	return nil
}

func (d *WebhookDispatcher) deliver(sub dbgen.WebhookSubscription, event eventbus.Event) {
	ctx := context.Background()

	// Build delivery payload
	deliveryPayload := map[string]any{
		"event_type": event.Subject,
		"tenant_id":  event.TenantID,
		"payload":    json.RawMessage(event.Payload),
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(deliveryPayload)

	// Attempt delivery with retry (3 attempts: immediate, 1s, 5s)
	delays := []time.Duration{0, 1 * time.Second, 5 * time.Second}
	var statusCode int
	var respBody string

	for attempt, delay := range delays {
		if delay > 0 {
			time.Sleep(delay)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", sub.Url, bytes.NewReader(body))
		if err != nil {
			zap.L().Error("webhook request build error", zap.String("url", sub.Url), zap.Error(err))
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Webhook-Event", event.Subject)

		// HMAC signature if secret is set
		if sub.Secret.Valid && sub.Secret.String != "" {
			mac := hmac.New(sha256.New, []byte(sub.Secret.String))
			mac.Write(body)
			sig := hex.EncodeToString(mac.Sum(nil))
			req.Header.Set("X-Webhook-Signature", "sha256="+sig)
		}

		resp, err := d.client.Do(req)
		if err != nil {
			zap.L().Warn("webhook delivery failed",
				zap.String("url", sub.Url), zap.Int("attempt", attempt+1), zap.Error(err))
			statusCode = 0
			respBody = err.Error()
			continue
		}

		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		resp.Body.Close()
		statusCode = resp.StatusCode
		respBody = string(respBytes)

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			break // success
		}
		zap.L().Warn("webhook non-2xx response",
			zap.String("url", sub.Url), zap.Int("status", resp.StatusCode), zap.Int("attempt", attempt+1))
	}

	// Record delivery
	_ = d.queries.CreateDelivery(ctx, dbgen.CreateDeliveryParams{
		SubscriptionID: sub.ID,
		EventType:      event.Subject,
		Payload:        body,
		StatusCode:     pgtype.Int4{Int32: int32(statusCode), Valid: true},
		ResponseBody:   pgtype.Text{String: respBody, Valid: true},
	})
}
