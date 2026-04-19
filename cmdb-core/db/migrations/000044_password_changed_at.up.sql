-- Track the timestamp of each user's last password change. The auth
-- middleware rejects any access token whose `iat` claim is older than this
-- value (error code TOKEN_OUTDATED), so rotating a password also effectively
-- revokes every access token issued before the rotation.
--
-- DEFAULT now() on the column lets the column backfill atomically for every
-- existing row; new users minted after this migration will have the column
-- set implicitly on INSERT, so no user-creation path needs to change.
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMPTZ NOT NULL DEFAULT now();
