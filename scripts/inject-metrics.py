#!/usr/bin/env python3
"""
Metrics simulator — injects realistic time-series data into CMDB metrics table.

Usage:
  python3 scripts/inject-metrics.py --backfill 24h
  python3 scripts/inject-metrics.py --continuous --interval 60
  python3 scripts/inject-metrics.py  # default: backfill 24h then continuous
"""

import argparse
import math
import random
import time
from datetime import datetime, timedelta, timezone

import psycopg2

DB_URL = "postgresql://cmdb:changeme@localhost:5432/cmdb"

METRICS = {
    "cpu_usage":    {"base": 45, "amplitude": 25, "noise": 10},
    "temperature":  {"base": 32, "amplitude": 8,  "noise": 3},
    "power_kw":     {"base": 3.5, "amplitude": 1.5, "noise": 0.5},
    "memory_usage": {"base": 60, "amplitude": 15, "noise": 8},
}

PUE_BASE = 1.35
PUE_NOISE = 0.08

def generate_value(cfg, t):
    hour = t.hour + t.minute / 60.0
    daily = math.sin((hour - 4) / 24 * 2 * math.pi)
    return max(0, round(cfg["base"] + cfg["amplitude"] * daily + random.gauss(0, cfg["noise"]), 2))

def get_assets(conn):
    with conn.cursor() as cur:
        cur.execute("SELECT id, tenant_id, type FROM assets")
        return cur.fetchall()

def inject_batch(conn, assets, ts):
    rows = []
    for asset_id, tenant_id, asset_type in assets:
        for name, cfg in METRICS.items():
            if name == "power_kw" and asset_type not in ("server", "power"):
                continue
            rows.append((ts, str(asset_id), str(tenant_id), name, generate_value(cfg, ts), "{}"))
    if assets:
        pue = round(PUE_BASE + random.gauss(0, PUE_NOISE), 3)
        rows.append((ts, str(assets[0][0]), str(assets[0][1]), "pue", pue, '{"scope":"campus"}'))
    with conn.cursor() as cur:
        cur.executemany(
            "INSERT INTO metrics (time, asset_id, tenant_id, name, value, labels) VALUES (%s, %s::uuid, %s::uuid, %s, %s, %s::jsonb)",
            rows)
    conn.commit()
    return len(rows)

def backfill(conn, assets, hours):
    now = datetime.now(timezone.utc)
    total = 0
    for m in range(hours * 60, 0, -1):
        ts = now - timedelta(minutes=m)
        total += inject_batch(conn, assets, ts)
        if m % 60 == 0:
            print(f"  backfill: {hours - m//60}h / {hours}h ({total} rows)")
    print(f"Backfill complete: {total} rows")

def continuous(conn, assets, interval):
    print(f"Continuous: every {interval}s (Ctrl+C to stop)")
    while True:
        ts = datetime.now(timezone.utc)
        n = inject_batch(conn, assets, ts)
        print(f"  {ts.isoformat()} — {n} metrics")
        time.sleep(interval)

def main():
    p = argparse.ArgumentParser()
    p.add_argument("--backfill", type=str, help="e.g., 24h, 7d")
    p.add_argument("--continuous", action="store_true")
    p.add_argument("--interval", type=int, default=60)
    p.add_argument("--db-url", default=DB_URL)
    args = p.parse_args()

    conn = psycopg2.connect(args.db_url)
    assets = get_assets(conn)
    print(f"Found {len(assets)} assets")

    if args.backfill:
        unit = args.backfill[-1]
        num = int(args.backfill[:-1])
        hours = num * 24 if unit == "d" else num
        backfill(conn, assets, hours)
    elif args.continuous:
        continuous(conn, assets, args.interval)
    else:
        backfill(conn, assets, 24)
        continuous(conn, assets, args.interval)
    conn.close()

if __name__ == "__main__":
    main()
