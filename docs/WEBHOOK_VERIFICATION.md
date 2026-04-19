# Webhook Signature Verification (v2)

> **BREAKING CHANGE — 2026-04-19.** The CMDB webhook dispatcher now signs
> outbound requests using scheme **v2**. Receivers that only verify v1
> (body-only) signatures will fail signature validation and reject every
> delivery. Upgrade your verifier before rolling this out to production.

## What changed

**v1 (deprecated, no longer emitted):**

```
HMAC_SHA256(secret, body)
```

**v2 (current):**

```
HMAC_SHA256(secret, timestamp + "." + body)
```

The dispatcher sends three headers on every signed request:

| Header                        | Example                                  | Required |
| ----------------------------- | ---------------------------------------- | -------- |
| `X-Webhook-Timestamp`         | `2026-04-19T10:32:15Z` (RFC 3339, UTC)   | yes      |
| `X-Webhook-Signature`         | `sha256=<64-hex>`                         | yes      |
| `X-Webhook-Signature-Version` | `v2`                                     | yes      |

Why the change: a v1 signature pinned only to the body is replayable
forever. A captured request can be re-delivered hours or days later and
the signature stays valid. Binding the signature to a timestamp and
refusing stale timestamps at the receiver closes that hole.

## Receiver verification algorithm

1. Read `X-Webhook-Signature-Version`. If it is not `v2`, reject —
   v1 signatures are no longer emitted and should not be accepted.
2. Read `X-Webhook-Timestamp`. Parse as RFC 3339.
3. If `abs(now - timestamp) > 5 minutes`, reject with `401`. This is
   the replay window.
4. Compute `expected = "sha256=" + hex(HMAC_SHA256(secret, timestamp + "." + raw_body))`.
5. Compare `expected` to `X-Webhook-Signature` using a **constant-time**
   comparison (`hmac.compare_digest` / `crypto.timingSafeEqual` /
   `hmac.Equal`). Never use `==` on the hex strings — a non-constant
   comparison leaks signature bytes via timing.
6. Read `raw_body` from the network **before** any JSON parsing or
   whitespace normalization. Re-serialized JSON will not match.

## Node.js example

```js
import crypto from "node:crypto";
import express from "express";

const app = express();
const SECRET = process.env.WEBHOOK_SECRET;

// Capture the raw body BEFORE JSON parsing — the signature is over bytes
// as-delivered, not over the re-serialized object.
app.use("/hook", express.raw({ type: "application/json" }));

app.post("/hook", (req, res) => {
  const version = req.get("X-Webhook-Signature-Version");
  const timestamp = req.get("X-Webhook-Timestamp");
  const signature = req.get("X-Webhook-Signature");

  if (version !== "v2") return res.status(401).send("unsupported sig version");
  if (!timestamp || !signature) return res.status(401).send("missing headers");

  const skewMs = Math.abs(Date.now() - Date.parse(timestamp));
  if (Number.isNaN(skewMs) || skewMs > 5 * 60 * 1000) {
    return res.status(401).send("stale timestamp");
  }

  const expected =
    "sha256=" +
    crypto.createHmac("sha256", SECRET)
      .update(timestamp + "." + req.body.toString())
      .digest("hex");

  const a = Buffer.from(signature);
  const b = Buffer.from(expected);
  if (a.length !== b.length || !crypto.timingSafeEqual(a, b)) {
    return res.status(401).send("bad signature");
  }

  // Safe to JSON.parse(req.body) now.
  res.status(204).end();
});
```

## Python example (FastAPI)

```python
import hmac
import hashlib
import time
from datetime import datetime, timezone
from fastapi import FastAPI, Request, HTTPException

app = FastAPI()
SECRET = b"replace-me"

@app.post("/hook")
async def hook(req: Request):
    version = req.headers.get("x-webhook-signature-version")
    timestamp = req.headers.get("x-webhook-timestamp")
    signature = req.headers.get("x-webhook-signature")

    if version != "v2":
        raise HTTPException(401, "unsupported sig version")
    if not timestamp or not signature:
        raise HTTPException(401, "missing headers")

    try:
        ts = datetime.fromisoformat(timestamp.replace("Z", "+00:00"))
    except ValueError:
        raise HTTPException(401, "bad timestamp")

    skew = abs((datetime.now(timezone.utc) - ts).total_seconds())
    if skew > 300:
        raise HTTPException(401, "stale timestamp")

    body = await req.body()
    expected = "sha256=" + hmac.new(
        SECRET, (timestamp + ".").encode() + body, hashlib.sha256
    ).hexdigest()

    if not hmac.compare_digest(expected, signature):
        raise HTTPException(401, "bad signature")

    return {"ok": True}
```

## Operator migration checklist

Before rolling out the dispatcher upgrade:

- [ ] Update every receiver to accept `v2` signatures.
- [ ] Keep v1 verification code **disabled** — accepting both is the
      same replay surface as v1 alone.
- [ ] Confirm clock skew between your edge receivers and the CMDB
      server is under 5 minutes. NTP is mandatory.
- [ ] Read raw body bytes before JSON parsing in your framework.
- [ ] Rotate the shared secret after the upgrade to invalidate any
      attacker-captured v1 traffic that could still be replayed against
      a stale receiver.

## Observability

The dispatcher emits Prometheus metrics operators should alert on:

| Metric                                  | Meaning                                                 |
| --------------------------------------- | ------------------------------------------------------- |
| `webhook_circuit_breaker_trips_total`   | Counter: subscription auto-disables after 3 failures.   |
| `webhook_dlq_rows_total`                | Counter: DLQ inserts (one per trip).                     |
| `webhook_retention_deletes_total{table}`| Counter: rows pruned by the daily retention sweep.      |

A flatlining `webhook_retention_deletes_total` for >24h with traffic in
the delivery log means the daily sweep goroutine has died.

A spike in `webhook_circuit_breaker_trips_total` means one or more
receivers are broken — check `webhook_deliveries` for the offending
subscription's last 3 attempts, fix the receiver, then manually clear
`disabled_at` via an admin endpoint.
