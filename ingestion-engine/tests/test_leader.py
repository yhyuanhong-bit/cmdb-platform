"""Tests for the Redis-backed leader election helper.

The unit under test is ``app.leader``. These tests use a fakeredis-style
in-process stand-in so we can exercise NX/EX semantics and the CAS-on-
release Lua script without a live Redis.
"""

from __future__ import annotations

import asyncio

import pytest

from app.leader import default_node_id, release, try_acquire


class _FakeAsyncRedis:
    """Minimal async fake matching the subset of the redis.asyncio API
    used by app.leader. Supports SET NX EX and Lua CAS release.

    This is intentionally NOT a full Redis clone — we only back what the
    helper actually calls so that any future API drift fails loudly at
    the test boundary instead of silently going green.
    """

    def __init__(self) -> None:
        self._store: dict[str, str] = {}

    async def set(self, key, value, *, nx=False, ex=None):  # noqa: ARG002
        if nx and key in self._store:
            return None
        self._store[key] = value
        return True

    async def eval(self, script, numkeys, *args):  # noqa: ARG002
        # Emulate the CAS-delete Lua used by app.leader.release.
        key, expected = args[0], args[1]
        if self._store.get(key) == expected:
            self._store.pop(key, None)
            return 1
        return 0

    async def get(self, key):
        return self._store.get(key)


@pytest.fixture
def fake_redis() -> _FakeAsyncRedis:
    return _FakeAsyncRedis()


def test_default_node_id_is_nonempty_and_unique() -> None:
    """Two calls must produce distinct IDs so concurrent replicas on
    the same host do not collide."""
    a = default_node_id()
    b = default_node_id()
    assert a and b
    assert a != b


async def test_first_caller_acquires_lease(fake_redis) -> None:
    got = await try_acquire(fake_redis, "lock:x", "node-1", ttl_seconds=60)
    assert got is True


async def test_second_caller_sees_locked(fake_redis) -> None:
    await try_acquire(fake_redis, "lock:x", "node-1", ttl_seconds=60)
    got = await try_acquire(fake_redis, "lock:x", "node-2", ttl_seconds=60)
    assert got is False, "NX semantics broken — second caller won"


async def test_release_lets_next_caller_acquire(fake_redis) -> None:
    await try_acquire(fake_redis, "lock:x", "node-1", ttl_seconds=60)
    await release(fake_redis, "lock:x", "node-1")
    got = await try_acquire(fake_redis, "lock:x", "node-2", ttl_seconds=60)
    assert got is True


async def test_release_is_cas_safe_against_wrong_owner(fake_redis) -> None:
    """If node-1's lease expired and node-2 re-acquired, node-1's late
    release must NOT wipe node-2's lease. This is the classic lease-
    stealing bug the Lua CAS prevents."""
    # node-1 acquires, then simulate "lease expired" by releasing it as
    # node-2 would not — we just have node-2 acquire directly afterwards.
    await try_acquire(fake_redis, "lock:x", "node-1", ttl_seconds=60)
    await release(fake_redis, "lock:x", "node-1")  # clean release
    await try_acquire(fake_redis, "lock:x", "node-2", ttl_seconds=60)

    # node-1 wakes up late and tries to release. Must be a no-op.
    await release(fake_redis, "lock:x", "node-1")
    owner = await fake_redis.get("lock:x")
    assert owner == "node-2", "stale release overwrote the new holder"


async def test_only_one_of_many_concurrent_acquires_wins(fake_redis) -> None:
    results = await asyncio.gather(
        *[try_acquire(fake_redis, "lock:x", f"n-{i}", ttl_seconds=60) for i in range(10)]
    )
    winners = [r for r in results if r]
    assert len(winners) == 1, f"expected exactly one winner, got {len(winners)}"
