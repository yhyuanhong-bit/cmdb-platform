#!/usr/bin/env bash
set -euo pipefail

export INGESTION_DATABASE_URL="${INGESTION_DATABASE_URL:-postgresql://cmdb:cmdb@localhost:5432/cmdb}"
export INGESTION_REDIS_URL="${INGESTION_REDIS_URL:-redis://localhost:6379/0}"
export INGESTION_NATS_URL="${INGESTION_NATS_URL:-nats://localhost:4222}"
export INGESTION_CELERY_BROKER_URL="${INGESTION_CELERY_BROKER_URL:-redis://localhost:6379/1}"
export INGESTION_DEPLOY_MODE="${INGESTION_DEPLOY_MODE:-development}"
export INGESTION_TENANT_ID="${INGESTION_TENANT_ID:-a0000000-0000-0000-0000-000000000001}"

uvicorn app.main:app --reload --host 0.0.0.0 --port 8081
