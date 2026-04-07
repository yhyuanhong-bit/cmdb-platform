"""Database credential provider — load and decrypt credentials."""

from uuid import UUID

import asyncpg

from app.config import settings
from app.credentials.encryption import decrypt_params, get_key_from_hex


class DBCredentialProvider:
    """Reads credentials from DB and decrypts params."""

    def __init__(self, pool: asyncpg.Pool):
        self._pool = pool
        self._key = get_key_from_hex(settings.credential_encryption_key)

    async def get(self, credential_id: UUID) -> dict:
        """Load and decrypt a credential by ID. Returns full row + decrypted params."""
        async with self._pool.acquire() as conn:
            row = await conn.fetchrow(
                "SELECT id, tenant_id, name, type, params, created_by, created_at, updated_at "
                "FROM credentials WHERE id = $1",
                credential_id,
            )
        if not row:
            raise ValueError(f"Credential {credential_id} not found")

        decrypted = decrypt_params(bytes(row["params"]), self._key)
        return {
            "id": str(row["id"]),
            "tenant_id": str(row["tenant_id"]),
            "name": row["name"],
            "type": row["type"],
            "params": decrypted,
        }
