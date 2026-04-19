"""Shared pytest configuration for ingestion-engine tests.

`app.config` now requires INGESTION_DEPLOY_MODE to be set at import time.
Most test modules import sub-packages (e.g. `app.tasks.discovery_task`) that
transitively import `app.config`, so we seed a safe development default here
for the whole session. Individual tests that need to exercise the missing-
or invalid-env paths should use `monkeypatch.delenv` / `monkeypatch.setenv`
inside the test — those changes are undone when the test finishes.
"""

import os

os.environ.setdefault("INGESTION_DEPLOY_MODE", "development")
