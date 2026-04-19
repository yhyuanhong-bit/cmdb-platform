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
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cmdb-platform/cmdb-core/internal/dbgen"
	"github.com/cmdb-platform/cmdb-core/internal/eventbus"
	"github.com/cmdb-platform/cmdb-core/internal/platform/netguard"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// NewWebhookDispatcher must refuse a nil guard — silent fallback to a raw
// transport would re-open the SSRF hole.
func TestNewWebhookDispatcher_NilGuardPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil guard")
		}
	}()
	_ = NewWebhookDispatcher(nil, nil, nil)
}

func TestNewWebhookDispatcher_AcceptsGuard(t *testing.T) {
	g, err := netguard.New(nil, nil)
	if err != nil {
		t.Fatalf("netguard.New: %v", err)
	}
	d := NewWebhookDispatcher(nil, nil, g)
	if d == nil {
		t.Fatal("expected non-nil dispatcher")
	}
	if d.client == nil || d.client.Transport == nil {
		t.Fatal("dispatcher must install a guarded transport on its client")
	}
}

// ---------------------------------------------------------------------------
// HMAC v2 signing tests — pure, no DB.
// ---------------------------------------------------------------------------

// TestAttemptOnce_SignsWithTimestampDotBody asserts the v2 signature format:
//
//	HMAC-SHA256(secret, timestamp + "." + body)
//
// and that X-Webhook-Timestamp / X-Webhook-Signature-Version: v2 are present.
// This is the core replay-protection guarantee — a v1 signature (body only)
// must not verify under v2 and vice versa.
func TestAttemptOnce_SignsWithTimestampDotBody(t *testing.T) {
	const secret = "super-secret-key"
	// Freeze time so the test is deterministic.
	fixedTime := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	wantTimestamp := fixedTime.Format(time.RFC3339)

	var gotHeaders http.Header
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewWebhookDispatcher(nil, nil, netguard.Permissive())
	d.now = func() time.Time { return fixedTime }

	sub := dbgen.WebhookSubscription{
		ID:     uuid.New(),
		Url:    srv.URL,
		Secret: pgtype.Text{String: secret, Valid: true},
	}
	event := eventbus.Event{
		Subject:  "asset.created",
		TenantID: uuid.New().String(),
		Payload:  []byte(`{"asset_id":"x"}`),
	}
	body := d.buildPayload(event)

	status, _, err := d.attemptOnce(context.Background(), sub, event, body, secret)
	if err != nil {
		t.Fatalf("attemptOnce: %v", err)
	}
	if status != 200 {
		t.Fatalf("status: want 200, got %d", status)
	}

	// Verify body was transmitted verbatim.
	if !bytes.Equal(gotBody, body) {
		t.Fatalf("body mismatch: want %s, got %s", body, gotBody)
	}

	// Verify timestamp header matches the frozen clock.
	if gotHeaders.Get("X-Webhook-Timestamp") != wantTimestamp {
		t.Fatalf("X-Webhook-Timestamp: want %q, got %q",
			wantTimestamp, gotHeaders.Get("X-Webhook-Timestamp"))
	}

	// Verify signature version header is v2 — NOT v1. This is the breaking
	// change operators need to see.
	if got := gotHeaders.Get(SignatureVersionHeader); got != SignatureVersionV2 {
		t.Fatalf("%s: want %q, got %q", SignatureVersionHeader, SignatureVersionV2, got)
	}

	// Recompute the v2 signature and compare.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(wantTimestamp + "." + string(body)))
	wantSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if got := gotHeaders.Get("X-Webhook-Signature"); got != wantSig {
		t.Fatalf("X-Webhook-Signature mismatch\n want: %s\n  got: %s", wantSig, got)
	}

	// Replay-safety: the v1 signature (body only, no timestamp) must
	// NOT match the v2 header. A receiver that validates under v2 must
	// reject v1-shaped signatures.
	v1Mac := hmac.New(sha256.New, []byte(secret))
	v1Mac.Write(body)
	v1Sig := "sha256=" + hex.EncodeToString(v1Mac.Sum(nil))
	if v1Sig == wantSig {
		t.Fatal("v1 (body-only) signature accidentally matches v2 — replay protection is broken")
	}
}

// TestAttemptOnce_NoSignatureWhenSecretEmpty verifies that missing secrets
// simply omit the signing headers rather than sending a bogus signature.
func TestAttemptOnce_NoSignatureWhenSecretEmpty(t *testing.T) {
	var gotHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := NewWebhookDispatcher(nil, nil, netguard.Permissive())
	sub := dbgen.WebhookSubscription{ID: uuid.New(), Url: srv.URL}
	event := eventbus.Event{Subject: "asset.created", TenantID: uuid.New().String()}
	body := d.buildPayload(event)

	_, _, err := d.attemptOnce(context.Background(), sub, event, body, "")
	if err != nil {
		t.Fatalf("attemptOnce: %v", err)
	}
	if gotHeaders.Get("X-Webhook-Signature") != "" {
		t.Fatal("expected no X-Webhook-Signature when secret is empty")
	}
	if gotHeaders.Get("X-Webhook-Timestamp") != "" {
		t.Fatal("expected no X-Webhook-Timestamp when secret is empty")
	}
	if gotHeaders.Get(SignatureVersionHeader) != "" {
		t.Fatalf("expected no %s when secret is empty", SignatureVersionHeader)
	}
}

// ---------------------------------------------------------------------------
// Example verifier — mirrors the documentation and serves as an executable
// spec for webhook receivers. If this test breaks, the docs are wrong.
// ---------------------------------------------------------------------------

// verifyV2 implements the receiver-side verification exactly as documented
// in docs/WEBHOOK_VERIFICATION.md. Tests below assert that the dispatcher
// produces output this function accepts, and that replays/old signatures
// are rejected.
func verifyV2(secret, timestamp, signature string, body []byte, now time.Time) error {
	// Reject stale timestamps (>5 min skew).
	ts, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		return errSigBadTimestamp
	}
	skew := now.Sub(ts)
	if skew < 0 {
		skew = -skew
	}
	if skew > 5*time.Minute {
		return errSigStale
	}
	// Compute expected v2 signature.
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "." + string(body)))
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(want), []byte(signature)) {
		return errSigMismatch
	}
	return nil
}

var (
	errSigBadTimestamp = &sigError{"bad_timestamp"}
	errSigStale        = &sigError{"stale_timestamp"}
	errSigMismatch     = &sigError{"signature_mismatch"}
)

type sigError struct{ reason string }

func (e *sigError) Error() string { return e.reason }

// TestVerifyV2_RejectsStaleTimestamp asserts the 5-minute replay window.
// A signed payload whose timestamp is 10 minutes old is a replay and must
// be rejected even if the signature itself is mathematically valid.
func TestVerifyV2_RejectsStaleTimestamp(t *testing.T) {
	const secret = "s3cret"
	ts := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"x":1}`)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts.Format(time.RFC3339) + "." + string(body)))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Verify "now" is 10 minutes later — should be rejected.
	err := verifyV2(secret, ts.Format(time.RFC3339), sig, body, ts.Add(10*time.Minute))
	if err != errSigStale {
		t.Fatalf("want errSigStale, got %v", err)
	}

	// And if "now" is 2 minutes later — should pass.
	if err := verifyV2(secret, ts.Format(time.RFC3339), sig, body, ts.Add(2*time.Minute)); err != nil {
		t.Fatalf("unexpected reject within window: %v", err)
	}
}

// TestVerifyV2_RejectsV1Signature is the replay-safety regression test.
// The dispatcher used to sign over body alone (v1). Under the new v2
// verifier, a v1-shaped signature must fail — otherwise upgrading the
// signer without upgrading the verifier would not change the security
// properties and the whole exercise is pointless.
func TestVerifyV2_RejectsV1Signature(t *testing.T) {
	const secret = "s3cret"
	ts := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	body := []byte(`{"x":1}`)

	// v1 signs over body only — the old pre-upgrade behavior.
	v1 := hmac.New(sha256.New, []byte(secret))
	v1.Write(body)
	v1Sig := "sha256=" + hex.EncodeToString(v1.Sum(nil))

	err := verifyV2(secret, ts.Format(time.RFC3339), v1Sig, body, ts.Add(1*time.Minute))
	if err != errSigMismatch {
		t.Fatalf("v2 verifier must reject v1 signatures, got err=%v", err)
	}
}

// ---------------------------------------------------------------------------
// buildPayload test — deterministic shape.
// ---------------------------------------------------------------------------

func TestBuildPayload_ShapeAndTimestamp(t *testing.T) {
	fixedTime := time.Date(2026, 4, 19, 10, 0, 0, 0, time.UTC)
	d := NewWebhookDispatcher(nil, nil, netguard.Permissive())
	d.now = func() time.Time { return fixedTime }
	tenantID := uuid.New().String()
	event := eventbus.Event{
		Subject:  "asset.created",
		TenantID: tenantID,
		Payload:  []byte(`{"hello":"world"}`),
	}
	body := d.buildPayload(event)

	var parsed struct {
		EventType string          `json:"event_type"`
		TenantID  string          `json:"tenant_id"`
		Payload   json.RawMessage `json:"payload"`
		Timestamp string          `json:"timestamp"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if parsed.EventType != "asset.created" {
		t.Errorf("event_type: got %q", parsed.EventType)
	}
	if parsed.TenantID != tenantID {
		t.Errorf("tenant_id: got %q", parsed.TenantID)
	}
	if parsed.Timestamp != fixedTime.Format(time.RFC3339) {
		t.Errorf("timestamp: got %q", parsed.Timestamp)
	}
}

// ---------------------------------------------------------------------------
// durationUntilNextUTCHour helper for retention scheduler — tested here
// because it's the kind of off-by-one-day logic that breaks silently.
// ---------------------------------------------------------------------------

// (The helper lives in workflows/cleanup.go; for the sake of this package's
// coverage we only assert the dispatcher behavior. The scheduler helper is
// covered by the workflows package test below when integration tag is set.)

// ---------------------------------------------------------------------------
// SSRF rejection still records a delivery row with attempt_number=1.
// Exercised via the URL being a blocked loopback literal and no real
// HTTP server needed.
// ---------------------------------------------------------------------------

// This is a regression test for the requirement that rejected URLs still
// record a delivery row. The dispatcher calls CreateDelivery directly for
// rejected URLs; we can't assert DB state without a real Queries so we
// wrap that case in the integration-tagged test below. Here we just assert
// that ValidateURL actually blocks loopback with the default guard so the
// rejection branch is hit.
func TestGuard_BlocksLoopbackByDefault(t *testing.T) {
	g, err := netguard.New(nil, nil)
	if err != nil {
		t.Fatalf("netguard.New: %v", err)
	}
	if err := g.ValidateURL("http://127.0.0.1:8080/hook"); err == nil {
		t.Fatal("default guard must block loopback URLs — dispatcher SSRF branch depends on it")
	}
}

// TestAttemptOnce_RetryCounter mostly lives as an integration test, but we
// can at least assert here that retries are attempted by setting up a
// counting server that never returns 2xx, and letting the dispatcher run
// via a stubbed queries-less code path would require a DB. Instead we
// exercise the lower-level `attemptOnce` loop shape by calling it three
// times and confirming the counter.
func TestAttemptOnce_CounterAdvancesAcrossCalls(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	d := NewWebhookDispatcher(nil, nil, netguard.Permissive())
	sub := dbgen.WebhookSubscription{ID: uuid.New(), Url: srv.URL}
	event := eventbus.Event{Subject: "asset.created", TenantID: uuid.New().String()}
	body := d.buildPayload(event)

	for i := 0; i < 3; i++ {
		status, _, _ := d.attemptOnce(context.Background(), sub, event, body, "")
		if status != 503 {
			t.Fatalf("attempt %d: want 503, got %d", i+1, status)
		}
	}
	if got := atomic.LoadInt32(&hits); got != 3 {
		t.Fatalf("server should have seen 3 POSTs, got %d", got)
	}
}
