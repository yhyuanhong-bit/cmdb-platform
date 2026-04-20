"""Tests for Celery task retry configuration.

Every Celery task under app.tasks.* must declare:
  - bind=True
  - autoretry_for = a non-empty tuple of transient-infrastructure exceptions
  - retry_backoff = True (exponential backoff)
  - retry_backoff_max <= 600 seconds (cap the exponential growth)
  - max_retries = 5

These tests assert the decorator-level configuration, and also exercise
Celery's retry semantics via apply() with a mocked worker path that raises
a retryable exception.
"""

from __future__ import annotations

import asyncpg
import httpx
import nats.errors
import pytest

from app.tasks import discovery_task, import_task


def _throws(exc_factory):
    """Build a replacement for ``_run_async`` that discards the coroutine and
    raises an exception — safely closing the coroutine avoids Python's
    ``coroutine was never awaited`` runtime warning."""

    def _runner(coro):
        coro.close()
        raise exc_factory()

    return _runner


TASKS_UNDER_TEST = [
    import_task.process_import_task,
    discovery_task.process_discovery_task,
]


# ──────────────────────────────────────────────
# Decorator-level config assertions
# ──────────────────────────────────────────────


@pytest.mark.parametrize("task", TASKS_UNDER_TEST, ids=lambda t: t.name)
def test_task_has_autoretry_for_configured(task):
    """Every task must declare a non-empty autoretry_for tuple."""
    autoretry_for = getattr(task, "autoretry_for", None)
    assert autoretry_for is not None, f"{task.name} missing autoretry_for"
    assert isinstance(autoretry_for, tuple), (
        f"{task.name} autoretry_for must be a tuple, got {type(autoretry_for)!r}"
    )
    assert len(autoretry_for) > 0, f"{task.name} autoretry_for is empty"


@pytest.mark.parametrize("task", TASKS_UNDER_TEST, ids=lambda t: t.name)
def test_task_does_not_blanket_retry_on_exception(task):
    """Retries must target transient infra failures only — never bare Exception."""
    assert Exception not in task.autoretry_for, (
        f"{task.name} must not blanket-retry on Exception"
    )
    assert BaseException not in task.autoretry_for


@pytest.mark.parametrize("task", TASKS_UNDER_TEST, ids=lambda t: t.name)
def test_task_retry_backoff_enabled(task):
    assert getattr(task, "retry_backoff", False) is True


@pytest.mark.parametrize("task", TASKS_UNDER_TEST, ids=lambda t: t.name)
def test_task_retry_backoff_max_capped(task):
    backoff_max = getattr(task, "retry_backoff_max", None)
    assert backoff_max is not None, f"{task.name} missing retry_backoff_max"
    assert backoff_max <= 600, (
        f"{task.name} retry_backoff_max={backoff_max} exceeds 600s cap"
    )


@pytest.mark.parametrize("task", TASKS_UNDER_TEST, ids=lambda t: t.name)
def test_task_max_retries_is_five(task):
    assert getattr(task, "max_retries", None) == 5, (
        f"{task.name} max_retries must be 5"
    )


# ──────────────────────────────────────────────
# Exception-tuple membership — per task
# ──────────────────────────────────────────────


def test_import_task_retries_on_postgres_error():
    assert asyncpg.PostgresError in import_task.process_import_task.autoretry_for


def test_discovery_task_retries_on_http_error():
    assert httpx.HTTPError in discovery_task.process_discovery_task.autoretry_for


def test_discovery_task_retries_on_postgres_error():
    assert asyncpg.PostgresError in discovery_task.process_discovery_task.autoretry_for


def test_discovery_task_retries_on_nats_error():
    """Discovery task publishes to NATS — transient NATS errors must retry."""
    assert nats.errors.Error in discovery_task.process_discovery_task.autoretry_for


# ──────────────────────────────────────────────
# Celery retry semantics — via apply() with mocked work
# ──────────────────────────────────────────────


def _spy_on_retry(monkeypatch, task):
    """Install a spy on ``task.retry`` that records every call.

    Returns a list that will be populated with (args, kwargs) for each
    retry invocation. The spy re-raises the inbound exception so the
    Celery trace machinery terminates after the first retry instead of
    looping ``max_retries`` times with real sleeps.
    """
    calls: list[dict] = []

    def spy(*args, **kwargs):
        calls.append({"args": args, "kwargs": kwargs})
        exc = kwargs.get("exc")
        if exc is not None:
            raise exc
        raise RuntimeError("retry called without exc")

    monkeypatch.setattr(task, "retry", spy)
    return calls


def test_import_task_autoretries_on_postgres_error(monkeypatch):
    """autoretry_for must invoke Task.retry when a PostgresError is raised.

    We spy on the task's ``retry`` method — the fact that Celery calls it
    (with the original exception) is the canonical proof that autoretry_for
    matched the raised exception class.
    """
    monkeypatch.setattr(
        import_task,
        "_run_async",
        _throws(lambda: asyncpg.PostgresError("simulated connection reset")),
    )
    retry_calls = _spy_on_retry(monkeypatch, import_task.process_import_task)

    import_task.process_import_task.apply(
        args=(
            "00000000-0000-0000-0000-000000000001",
            "a0000000-0000-0000-0000-000000000001",
            "/tmp/nope.xlsx",
            "xlsx",
        ),
    )

    assert len(retry_calls) == 1, (
        f"expected exactly one retry invocation, got {len(retry_calls)}"
    )
    assert isinstance(retry_calls[0]["kwargs"]["exc"], asyncpg.PostgresError)


def test_discovery_task_autoretries_on_http_error(monkeypatch):
    """Same shape, different task + exception class."""
    monkeypatch.setattr(
        discovery_task,
        "_run_async",
        _throws(lambda: httpx.ConnectError("cmdb-core unreachable")),
    )
    retry_calls = _spy_on_retry(
        monkeypatch, discovery_task.process_discovery_task
    )

    discovery_task.process_discovery_task.apply(
        args=(
            "00000000-0000-0000-0000-000000000001",
            "a0000000-0000-0000-0000-000000000001",
            "snmp",
            ["10.0.0.0/24"],
            "cred-1",
            "auto",
        ),
    )

    assert len(retry_calls) == 1
    assert isinstance(retry_calls[0]["kwargs"]["exc"], httpx.HTTPError)


def test_import_task_does_not_retry_on_non_retryable_error(monkeypatch):
    """ValueError is deterministic — it must propagate, not trigger retry."""
    monkeypatch.setattr(
        import_task,
        "_run_async",
        _throws(lambda: ValueError("bad file_type")),
    )
    retry_calls = _spy_on_retry(monkeypatch, import_task.process_import_task)

    result = import_task.process_import_task.apply(
        args=(
            "00000000-0000-0000-0000-000000000001",
            "a0000000-0000-0000-0000-000000000001",
            "/tmp/nope.xlsx",
            "xlsx",
        ),
    )

    assert retry_calls == [], (
        "ValueError incorrectly triggered a retry — autoretry_for is too broad"
    )
    assert result.failed()
    assert isinstance(result.result, ValueError)
