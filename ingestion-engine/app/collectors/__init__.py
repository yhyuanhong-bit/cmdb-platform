"""Register all collectors on import."""

from app.collectors.base import registry
from app.collectors.snmp import SNMPCollector

# SSH and IPMI auto-register at module level, but SNMP doesn't.
# Import them to trigger registration, then register SNMP.
import app.collectors.ssh  # noqa: F401
import app.collectors.ipmi  # noqa: F401
# OneView + Dell OME also self-register on import.
import app.collectors.oneview  # noqa: F401
import app.collectors.dell_ome  # noqa: F401

registry.register(SNMPCollector())
