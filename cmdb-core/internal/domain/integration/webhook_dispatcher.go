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
	"github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
	"github.com/cmdb-platform/cmdb-core/internal/platform/telemetry"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

// circuitBreakerThreshold is the number of consecutive delivery failures
// that trips a subscription. Chosen to mirror the inbound adapter threshold
// in integration_adapters so ops has one number to remember.
const circuitBreakerThreshold = 3

// Source label for telemetry.ErrorsSuppressedTotal emitted when the
// BIA-filter path can't parse the event payload. A broken payload
// used to silently disable the BIA filter and let the dispatcher
// fire webhooks it should have held back.
const sourceWebhookBIAFilter = "integration.webhook.bia_filter"

// SignatureVersionHeader advertises the signing scheme to receivers. v1 was
// HMAC(secret, body). v2 is HMAC(secret, timestamp + "." + body) and is
// the ONLY scheme this dispatcher emits. Receivers must branch on this
// header to keep replay-protection guarantees.
const (
	SignatureVersionHeader = "X-Webhook-Signature-Version"
	SignatureVersionV2     = "v2"
)

// WebhookDispatcher delivers events to registered webhook subscriptions.
type WebhookDispatcher struct {
	queries *dbgen.Queries
	cipher  crypto.Cipher
	guard   *netguard.Guard
	client  *http.Client
	bus     eventbus.Bus // optional; used to emit SubjectWebhookDisabled
	now     func() time.Time
	// baseCtx is the server-lifecycle context. HandleEvent fan-out
	// goroutines derive their own short-lived ctx from this so a SIGTERM
	// cancels every in-flight retry/sleep. Falls back to context.Background
	// when unset (e.g. in tests), so existing call sites keep working.
	baseCtx context.Context
}

// NewWebhookDispatcher creates a new dispatcher. The guard is required: it
// enforces SSRF protection at URL-parse time and again at DialContext time
// (defeats DNS rebinding). Pass netguard.Permissive() in tests that need
// to hit 127.0.0.1.
func NewWebhookDispatcher(queries *dbgen.Queries, cipher crypto.Cipher, guard *netguard.Guard) *WebhookDispatcher {
	if guard == nil {
		// Fail-closed: a misconfigured guard must NOT silently drop to
		// default transport that would allow loopback dials.
		panic("netguard Guard is required for WebhookDispatcher")
	}
	return &WebhookDispatcher{
		queries: queries,
		cipher:  cipher,
		guard:   guard,
		client: &http.Client{
			Transport: guard.SafeTransport(nil),
			Timeout:   10 * time.Second,
		},
		now: time.Now,
	}
}

// WithEventBus wires the eventbus so the circuit breaker can emit
// SubjectWebhookDisabled when it trips. Optional — dispatcher still works
// without it, trips just don't notify ops-admin.
func (d *WebhookDispatcher) WithEventBus(bus eventbus.Bus) *WebhookDispatcher {
	d.bus = bus
	return d
}

// WithBaseContext wires the server-lifecycle context so background
// delivery goroutines (each HandleEvent fan-out spawns one per
// subscription) cancel immediately on SIGTERM instead of running their
// full backoff schedule against a torn-down process. Optional — defaults
// to context.Background so existing tests and call sites don't need to
// plumb a ctx.
func (d *WebhookDispatcher) WithBaseContext(ctx context.Context) *WebhookDispatcher {
	d.baseCtx = ctx
	return d
}

// deliveryContext returns the context that a delivery goroutine should
// use. Prefers baseCtx when set so shutdown cancels in-flight retries.
func (d *WebhookDispatcher) deliveryContext() context.Context {
	if d.baseCtx != nil {
		return d.baseCtx
	}
	return context.Background()
}

// HandleEvent processes an event and delivers it to matching webhook subscriptions.
func (d *WebhookDispatcher) HandleEvent(ctx context.Context, event eventbus.Event) error {
	// Find subscriptions matching this event type (scoped to tenant,
	// enabled, and not circuit-broken — the SQL filters disabled_at).
	tenantUUID, _ := uuid.Parse(event.TenantID)
	subs, err := d.queries.ListWebhooksByEvent(ctx, dbgen.ListWebhooksByEventParams{
		TenantID: tenantUUID,
		Column2:  event.Subject,
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
				// Extract asset_id from event payload. A malformed
				// payload here used to be a silent no-op which then
				// dispatched the webhook without its BIA filter
				// applied — either over- or under-firing. Warn +
				// counter, then fall through to the missing-asset_id
				// branch which returns below.
				var payload map[string]string
				if err := json.Unmarshal(event.Payload, &payload); err != nil {
					zap.L().Warn("webhook BIA filter: payload parse failed",
						zap.String("subject", event.Subject), zap.Error(err))
					telemetry.ErrorsSuppressedTotal.WithLabelValues(sourceWebhookBIAFilter, telemetry.ReasonJSONUnmarshal).Inc()
				}
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

// deliver attempts up to 3 HTTP POSTs with backoff. Each attempt records its
// own webhook_deliveries row. On terminal failure the subscription's
// consecutive_failures counter is incremented and the breaker may trip.
//
// The dispatcher's baseCtx (set by WithBaseContext from main) is used so a
// SIGTERM cancels in-flight deliveries and pending backoff sleeps rather
// than stranding goroutines until the 10s per-request HTTP timeout fires
// three times.
func (d *WebhookDispatcher) deliver(sub dbgen.WebhookSubscription, event eventbus.Event) {
	ctx := d.deliveryContext()

	// Defensive re-check: even though ListWebhooksByEvent filters on
	// disabled_at, a race could allow a sub to be tripped between list
	// and deliver. Skip cleanly instead of hitting a dead receiver.
	if sub.DisabledAt.Valid {
		return
	}

	body := d.buildPayload(event)

	// SSRF pre-check: reject loopback/metadata/private targets before we
	// ever dial. Record a synthetic delivery row so operators see the
	// rejection in the delivery log instead of silent drops.
	if err := d.guard.ValidateURL(sub.Url); err != nil {
		zap.L().Warn("webhook url rejected by netguard",
			zap.String("url", sub.Url), zap.String("webhook_id", sub.ID.String()), zap.Error(err))
		_ = d.queries.CreateDelivery(ctx, dbgen.CreateDeliveryParams{
			SubscriptionID: sub.ID,
			EventType:      event.Subject,
			Payload:        body,
			StatusCode:     pgtype.Int4{Int32: 0, Valid: true},
			ResponseBody:   pgtype.Text{String: "url_rejected: " + err.Error(), Valid: true},
			AttemptNumber:  1,
		})
		d.recordFailure(ctx, sub, event, body, "url_rejected: "+err.Error(), 1)
		return
	}

	secret, secretErr := DecryptSecretWithFallback(d.cipher, sub.SecretEncrypted, sub.Secret.String)
	if secretErr != nil {
		zap.L().Error("webhook secret decrypt failed — skipping signature",
			zap.String("webhook_id", sub.ID.String()), zap.Error(secretErr))
		secret = ""
	}

	// 3 attempts: immediate, 1s, 5s.
	delays := []time.Duration{0, 1 * time.Second, 5 * time.Second}
	var lastStatus int
	var lastBody string

	for attempt, delay := range delays {
		if delay > 0 {
			// Respect shutdown: a cancelled baseCtx must interrupt the
			// backoff wait instead of pinning the goroutine for 5s after
			// SIGTERM. On cancellation, record the last status and return
			// without continuing to the next attempt.
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				zap.L().Info("webhook delivery aborted by shutdown",
					zap.String("url", sub.Url),
					zap.Int("attempt", attempt+1))
				return
			case <-timer.C:
			}
		}
		attemptNumber := int32(attempt + 1)

		status, respBody, err := d.attemptOnce(ctx, sub, event, body, secret)
		lastStatus = status
		lastBody = respBody

		// Record every attempt as its own row — operators need the full
		// retry timeline, not just the last state.
		_ = d.queries.CreateDelivery(ctx, dbgen.CreateDeliveryParams{
			SubscriptionID: sub.ID,
			EventType:      event.Subject,
			Payload:        body,
			StatusCode:     pgtype.Int4{Int32: int32(status), Valid: true},
			ResponseBody:   pgtype.Text{String: respBody, Valid: true},
			AttemptNumber:  attemptNumber,
		})

		if err == nil && status >= 200 && status < 300 {
			d.recordSuccess(ctx, sub)
			return
		}
		zap.L().Warn("webhook delivery attempt failed",
			zap.String("url", sub.Url),
			zap.Int("attempt", attempt+1),
			zap.Int("status", status))
	}

	// All retries exhausted.
	reason := lastBody
	if reason == "" {
		reason = "no response"
	}
	d.recordFailure(ctx, sub, event, body, reason, lastStatus)
}

// attemptOnce signs, sends, and reads one HTTP request. Returns the status
// code (0 on transport error), truncated response body, and any error.
func (d *WebhookDispatcher) attemptOnce(
	ctx context.Context,
	sub dbgen.WebhookSubscription,
	event eventbus.Event,
	body []byte,
	secret string,
) (int, string, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", sub.Url, bytes.NewReader(body))
	if err != nil {
		zap.L().Error("webhook request build error", zap.String("url", sub.Url), zap.Error(err))
		return 0, err.Error(), err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Webhook-Event", event.Subject)

	if secret != "" {
		timestamp := d.now().UTC().Format(time.RFC3339)
		signed := timestamp + "." + string(body)
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write([]byte(signed))
		req.Header.Set("X-Webhook-Timestamp", timestamp)
		req.Header.Set("X-Webhook-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
		req.Header.Set(SignatureVersionHeader, SignatureVersionV2)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err.Error(), err
	}
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
	return resp.StatusCode, string(respBytes), nil
}

// buildPayload renders the webhook JSON body for an event. Extracted so
// tests can assert the shape without reaching into deliver().
func (d *WebhookDispatcher) buildPayload(event eventbus.Event) []byte {
	deliveryPayload := map[string]any{
		"event_type": event.Subject,
		"tenant_id":  event.TenantID,
		"payload":    json.RawMessage(event.Payload),
		"timestamp":  d.now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(deliveryPayload)
	return body
}

// recordSuccess clears the failure counter. Guarded by "AND disabled_at IS
// NULL" in SQL so a late-succeeding retry can't silently re-enable a
// manually-disabled sub.
func (d *WebhookDispatcher) recordSuccess(ctx context.Context, sub dbgen.WebhookSubscription) {
	if err := d.queries.RecordWebhookSuccess(ctx, sub.ID); err != nil {
		zap.L().Warn("webhook: failed to clear failure state",
			zap.String("webhook_id", sub.ID.String()), zap.Error(err))
	}
}

// recordFailure increments the counter and trips the breaker when the
// threshold is reached. DLQ insert + SubjectWebhookDisabled publish happen
// together so ops-admin notification and the DLQ row always appear as a
// pair — never one without the other.
func (d *WebhookDispatcher) recordFailure(
	ctx context.Context,
	sub dbgen.WebhookSubscription,
	event eventbus.Event,
	body []byte,
	lastError string,
	attemptCount int,
) {
	row, err := d.queries.RecordWebhookFailure(ctx, sub.ID)
	if err != nil {
		zap.L().Error("webhook: failed to record failure",
			zap.String("webhook_id", sub.ID.String()), zap.Error(err))
		return
	}

	if row.ConsecutiveFailures < circuitBreakerThreshold {
		return
	}

	// Threshold reached — trip the breaker.
	if err := d.queries.DisableWebhook(ctx, sub.ID); err != nil {
		zap.L().Error("webhook: failed to disable after threshold",
			zap.String("webhook_id", sub.ID.String()), zap.Error(err))
		return
	}
	telemetry.WebhookCircuitBreakerTripsTotal.Inc()

	// Park the payload in the DLQ so ops can replay once the receiver
	// is fixed. Truncate last_error to keep DLQ rows bounded.
	if len(lastError) > 2000 {
		lastError = lastError[:2000]
	}
	if err := d.queries.CreateDLQEntry(ctx, dbgen.CreateDLQEntryParams{
		SubscriptionID: pgtype.UUID{Bytes: sub.ID, Valid: true},
		EventType:      event.Subject,
		Payload:        body,
		LastError:      lastError,
		AttemptCount:   int32(attemptCount),
		TenantID:       row.TenantID,
	}); err != nil {
		zap.L().Error("webhook: failed to create DLQ entry",
			zap.String("webhook_id", sub.ID.String()), zap.Error(err))
	} else {
		telemetry.WebhookDLQRowsTotal.Inc()
	}

	zap.L().Warn("webhook circuit breaker tripped",
		zap.String("webhook_id", sub.ID.String()),
		zap.String("tenant_id", row.TenantID.String()),
		zap.Int32("consecutive_failures", row.ConsecutiveFailures))

	// Notify ops-admin via eventbus. The workflow subscriber is already
	// wired to translate this into a notification row.
	if d.bus != nil {
		notifyPayload, _ := json.Marshal(map[string]any{
			"webhook_id":           sub.ID.String(),
			"tenant_id":            row.TenantID.String(),
			"name":                 sub.Name,
			"url":                  sub.Url,
			"consecutive_failures": row.ConsecutiveFailures,
			"last_error":           lastError,
		})
		_ = d.bus.Publish(ctx, eventbus.Event{
			Subject:  eventbus.SubjectWebhookDisabled,
			TenantID: row.TenantID.String(),
			Payload:  notifyPayload,
		})
	}
}
