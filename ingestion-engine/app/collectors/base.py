"""Collector protocol and registry."""

from typing import Protocol, runtime_checkable

from app.models.common import CollectTarget, ConnectionResult, FieldMapping, RawAssetData


@runtime_checkable
class Collector(Protocol):
    """Protocol that all collectors must implement."""

    name: str
    collect_type: str

    async def collect(self, target: CollectTarget) -> list[RawAssetData]: ...

    async def test_connection(self, target: CollectTarget) -> ConnectionResult: ...

    def supported_fields(self) -> list[FieldMapping]: ...


class CollectorRegistry:
    """Registry of available collectors."""

    def __init__(self):
        self._collectors: dict[str, Collector] = {}

    def register(self, collector: Collector) -> None:
        """Register a collector by name."""
        self._collectors[collector.name] = collector

    def get(self, name: str) -> Collector | None:
        """Get a collector by name."""
        return self._collectors.get(name)

    def list_all(self) -> list[dict]:
        """List all registered collectors as dicts."""
        return [
            {
                "name": c.name,
                "collect_type": c.collect_type,
                "supported_fields": [f.model_dump() for f in c.supported_fields()],
            }
            for c in self._collectors.values()
        ]


registry = CollectorRegistry()
