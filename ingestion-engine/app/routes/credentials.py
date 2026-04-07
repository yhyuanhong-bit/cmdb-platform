"""Credentials CRUD routes."""

import logging
import uuid
from typing import Any

import asyncpg
from fastapi import APIRouter, Depends, HTTPException
from pydantic import BaseModel

from app.config import settings
from app.credentials.encryption import encrypt_params, get_key_from_hex
from app.dependencies import get_db_pool

logger = logging.getLogger(__name__)

router = APIRouter(tags=["credentials"])

VALID_CREDENTIAL_TYPES = {"snmp_v2c", "snmp_v3", "ssh_password", "ssh_key", "ipmi"}


class CredentialCreate(BaseModel):
    name: str
    type: str
    params: dict[str, Any]
    tenant_id: str


class CredentialUpdate(BaseModel):
    name: str | None = None
    type: str | None = None
    params: dict[str, Any] | None = None


@router.get("/credentials")
async def list_credentials(
    tenant_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """List credentials for a tenant. Params/passwords are never returned."""
    async with pool.acquire() as conn:
        rows = await conn.fetch(
            "SELECT id, tenant_id, name, type, created_at, updated_at "
            "FROM credentials WHERE tenant_id = $1 ORDER BY name",
            uuid.UUID(tenant_id),
        )
    return [dict(row) for row in rows]


@router.post("/credentials", status_code=201)
async def create_credential(
    body: CredentialCreate,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Create a new credential with encrypted params."""
    if body.type not in VALID_CREDENTIAL_TYPES:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid credential type '{body.type}'. Must be one of: {', '.join(sorted(VALID_CREDENTIAL_TYPES))}",
        )

    key = get_key_from_hex(settings.credential_encryption_key)
    encrypted = encrypt_params(body.params, key)

    credential_id = uuid.uuid4()
    try:
        async with pool.acquire() as conn:
            row = await conn.fetchrow(
                """INSERT INTO credentials (id, tenant_id, name, type, params)
                   VALUES ($1, $2, $3, $4, $5)
                   RETURNING id, tenant_id, name, type, created_at, updated_at""",
                credential_id,
                uuid.UUID(body.tenant_id),
                body.name,
                body.type,
                encrypted,
            )
    except asyncpg.UniqueViolationError:
        raise HTTPException(
            status_code=409,
            detail=f"Credential with name '{body.name}' already exists for this tenant",
        )

    return dict(row)


@router.put("/credentials/{credential_id}")
async def update_credential(
    credential_id: str,
    body: CredentialUpdate,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Update a credential. Params are optional; if omitted, existing params are kept."""
    if body.type is not None and body.type not in VALID_CREDENTIAL_TYPES:
        raise HTTPException(
            status_code=400,
            detail=f"Invalid credential type '{body.type}'. Must be one of: {', '.join(sorted(VALID_CREDENTIAL_TYPES))}",
        )

    async with pool.acquire() as conn:
        existing = await conn.fetchrow(
            "SELECT id FROM credentials WHERE id = $1",
            uuid.UUID(credential_id),
        )
        if not existing:
            raise HTTPException(status_code=404, detail="Credential not found")

        # Build dynamic SET clause
        set_parts = []
        params: list[Any] = []
        idx = 1

        if body.name is not None:
            set_parts.append(f"name = ${idx}")
            params.append(body.name)
            idx += 1

        if body.type is not None:
            set_parts.append(f"type = ${idx}")
            params.append(body.type)
            idx += 1

        if body.params is not None:
            key = get_key_from_hex(settings.credential_encryption_key)
            encrypted = encrypt_params(body.params, key)
            set_parts.append(f"params = ${idx}")
            params.append(encrypted)
            idx += 1

        if not set_parts:
            raise HTTPException(status_code=400, detail="No fields to update")

        set_parts.append(f"updated_at = now()")
        params.append(uuid.UUID(credential_id))

        query = (
            f"UPDATE credentials SET {', '.join(set_parts)} "
            f"WHERE id = ${idx} "
            f"RETURNING id, tenant_id, name, type, created_at, updated_at"
        )

        try:
            row = await conn.fetchrow(query, *params)
        except asyncpg.UniqueViolationError:
            raise HTTPException(
                status_code=409,
                detail=f"Credential with name '{body.name}' already exists for this tenant",
            )

    return dict(row)


@router.delete("/credentials/{credential_id}", status_code=204)
async def delete_credential(
    credential_id: str,
    pool: asyncpg.Pool = Depends(get_db_pool),
):
    """Delete a credential. Rejected with 409 if referenced by scan_targets."""
    async with pool.acquire() as conn:
        existing = await conn.fetchrow(
            "SELECT id FROM credentials WHERE id = $1",
            uuid.UUID(credential_id),
        )
        if not existing:
            raise HTTPException(status_code=404, detail="Credential not found")

        ref_count = await conn.fetchval(
            "SELECT count(*) FROM scan_targets WHERE credential_id = $1",
            uuid.UUID(credential_id),
        )
        if ref_count and ref_count > 0:
            raise HTTPException(
                status_code=409,
                detail="Credential is referenced by one or more scan targets and cannot be deleted",
            )

        await conn.execute(
            "DELETE FROM credentials WHERE id = $1",
            uuid.UUID(credential_id),
        )
