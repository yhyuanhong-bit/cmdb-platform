"""Redis-backed leader election for singleton background tasks.

Context:
    ingestion-engine runs inside a FastAPI lifespan with one or more
    periodic background loops (notably the MAC-table scan). Before
    multi-replica support, those loops lived in-process on "the one"
    server. Running the same loop in every replica would:

      - hammer switches and store duplicate arp rows
      - double-publish NATS events that downstream consumers key by time
      - inflate Celery queue depth with near-identical jobs

    Moving every loop to Celery beat is the long-term answer, but the
    short-term fix is cheaper and safer: keep the in-process loop, but
    gate it on a lease held in Redis. Exactly one replica wins the
    lease; the others no-op until it expires or is released.

Guarantees:
    * Safety: `SET key value NX EX ttl` is atomic in Redis. Two replicas
      racing will see exactly one OK.
    * Liveness: the lease has a TTL; a dying leader's lock auto-expires
      so a successor can take over within `ttl` seconds.
    * No split-brain risk for our use case: the protected workload
      (MAC scan) is idempotent-ish, and the worst case is a double run
      during a failover window — acceptable.

The helper is deliberately tiny (no Redlock, no watchdog). The window
between "lease expired in Redis" and "old leader notices" is bounded by
the lease TTL, which is the caller's to tune against their interval.
"""

from __future__ import annotations

import os
import socket
import uuid

import redis.asyncio as redis


def default_node_id() -> str:
    """Stable-ish identifier for this process instance.

    Uses hostname + pid so pod name and container id are both visible in
    logs, plus a short random tail to disambiguate two processes on the
    same host. Not used for correctness — only for observability.
    """
    return f"{socket.gethostname()}:{os.getpid()}:{uuid.uuid4().hex[:6]}"


async def try_acquire(
    client: redis.Redis,
    key: str,
    node_id: str,
    ttl_seconds: int,
) -> bool:
    """Attempt to acquire the lease; return True iff this caller is now leader.

    The lease is stored as the caller's node_id so logs on all replicas
    agree about who is currently leading. We deliberately do NOT refresh
    or extend the lease — callers re-acquire on every tick, which keeps
    the failover interval == ttl.
    """
    ok = await client.set(key, node_id, nx=True, ex=ttl_seconds)
    return bool(ok)


async def release(client: redis.Redis, key: str, node_id: str) -> None:
    """Release the lease if and only if this caller still holds it.

    Uses a Lua CAS to avoid the classic bug: lease expires, another
    replica acquires, original leader's DEL would wipe the new holder.
    Safe to call unconditionally — a lost lease is a silent no-op.
    """
    script = (
        "if redis.call('GET', KEYS[1]) == ARGV[1] then "
        "return redis.call('DEL', KEYS[1]) "
        "else return 0 end"
    )
    await client.eval(script, 1, key, node_id)
