"""Collector manager: tracks running state and delegates operations."""

from dataclasses import dataclass, field
from datetime import datetime

from app.collectors.base import registry
from app.models.common import CollectTarget, ConnectionResult


@dataclass
class CollectorStatus:
    name: str
    running: bool = False
    last_run: datetime | None = None
    last_error: str | None = None
    items_collected: int = 0


class CollectorManager:
    """Manages collector lifecycle and status."""

    def __init__(self):
        self._status: dict[str, CollectorStatus] = {}

    def _ensure_status(self, name: str) -> CollectorStatus:
        if name not in self._status:
            self._status[name] = CollectorStatus(name=name)
        return self._status[name]

    def get_status(self, name: str) -> CollectorStatus:
        return self._ensure_status(name)

    def list_all(self) -> list[dict]:
        """Merge registry info with runtime status."""
        result = []
        for info in registry.list_all():
            status = self._ensure_status(info["name"])
            result.append({
                **info,
                "running": status.running,
                "last_run": status.last_run.isoformat() if status.last_run else None,
                "last_error": status.last_error,
                "items_collected": status.items_collected,
            })
        return result

    def start(self, name: str) -> CollectorStatus:
        """Mark a collector as running."""
        collector = registry.get(name)
        if not collector:
            raise ValueError(f"Collector '{name}' not found in registry")
        status = self._ensure_status(name)
        status.running = True
        return status

    def stop(self, name: str) -> CollectorStatus:
        """Mark a collector as stopped."""
        status = self._ensure_status(name)
        status.running = False
        return status

    async def test_connection(self, name: str, target: CollectTarget) -> ConnectionResult:
        """Delegate connection test to the collector."""
        collector = registry.get(name)
        if not collector:
            raise ValueError(f"Collector '{name}' not found in registry")
        return await collector.test_connection(target)


manager = CollectorManager()
