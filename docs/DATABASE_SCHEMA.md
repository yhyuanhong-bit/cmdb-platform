# CMDB 数据库表结构文档

> 生成时间: 2026-04-17 08:39:55
> 数据库: `cmdb` (PostgreSQL)
> 容器: `deploy-postgres-1`

## 概览

- 总表数: 52
- PostgreSQL 版本: PostgreSQL 17.7 on x86_64-pc-linux-musl
- 扩展: ltree 1.3, uuid-ossp 1.1, pgcrypto 1.3, pg_trgm 1.6, timescaledb 2.26.1

## 目录

- [`alert_events`](#alert_events)
- [`alert_rules`](#alert_rules)
- [`asset_dependencies`](#asset_dependencies)
- [`asset_field_authorities`](#asset_field_authorities)
- [`asset_location_history`](#asset_location_history)
- [`assets`](#assets)
- [`audit_events`](#audit_events)
- [`bia_assessments`](#bia_assessments)
- [`bia_dependencies`](#bia_dependencies)
- [`bia_scoring_rules`](#bia_scoring_rules)
- [`credentials`](#credentials)
- [`departments`](#departments)
- [`discovered_assets`](#discovered_assets)
- [`discovery_candidates`](#discovery_candidates)
- [`discovery_tasks`](#discovery_tasks)
- [`import_conflicts`](#import_conflicts)
- [`import_jobs`](#import_jobs)
- [`incidents`](#incidents)
- [`integration_adapters`](#integration_adapters)
- [`inventory_items`](#inventory_items)
- [`inventory_notes`](#inventory_notes)
- [`inventory_scan_history`](#inventory_scan_history)
- [`inventory_tasks`](#inventory_tasks)
- [`locations`](#locations)
- [`mac_address_cache`](#mac_address_cache)
- [`metrics`](#metrics)
- [`notifications`](#notifications)
- [`prediction_models`](#prediction_models)
- [`prediction_results`](#prediction_results)
- [`quality_rules`](#quality_rules)
- [`quality_scores`](#quality_scores)
- [`rack_network_connections`](#rack_network_connections)
- [`rack_slots`](#rack_slots)
- [`racks`](#racks)
- [`rca_analyses`](#rca_analyses)
- [`roles`](#roles)
- [`scan_targets`](#scan_targets)
- [`schema_migrations`](#schema_migrations)
- [`sensors`](#sensors)
- [`switch_port_mapping`](#switch_port_mapping)
- [`sync_conflicts`](#sync_conflicts)
- [`sync_state`](#sync_state)
- [`tenants`](#tenants)
- [`upgrade_rules`](#upgrade_rules)
- [`user_roles`](#user_roles)
- [`user_sessions`](#user_sessions)
- [`users`](#users)
- [`webhook_deliveries`](#webhook_deliveries)
- [`webhook_subscriptions`](#webhook_subscriptions)
- [`work_order_comments`](#work_order_comments)
- [`work_order_logs`](#work_order_logs)
- [`work_orders`](#work_orders)

---

## alert_events

**当前行数:** 8

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `rule_id` | `uuid` | YES |  |  |
| 4 | `asset_id` | `uuid` | YES |  |  |
| 5 | `status` | `varchar(20)` | NO | `'firing'::character varying` |  |
| 6 | `severity` | `varchar(20)` | NO |  |  |
| 7 | `message` | `text` | YES |  |  |
| 8 | `trigger_value` | `numeric(12,4)` | YES |  |  |
| 9 | `fired_at` | `timestamp with time zone` | NO | `now()` |  |
| 10 | `acked_at` | `timestamp with time zone` | YES |  |  |
| 11 | `resolved_at` | `timestamp with time zone` | YES |  |  |
| 12 | `sync_version` | `bigint` | NO | `0` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `alert_events_pkey` | `CREATE UNIQUE INDEX alert_events_pkey ON public.alert_events USING btree (id)` |
| `idx_alert_events_asset_id` | `CREATE INDEX idx_alert_events_asset_id ON public.alert_events USING btree (asset_id)` |
| `idx_alert_events_rule_id` | `CREATE INDEX idx_alert_events_rule_id ON public.alert_events USING btree (rule_id)` |
| `idx_alert_events_sync_version` | `CREATE INDEX idx_alert_events_sync_version ON public.alert_events USING btree (tenant_id, sync_version)` |
| `idx_alert_events_tenant_fired` | `CREATE INDEX idx_alert_events_tenant_fired ON public.alert_events USING btree (tenant_id, fired_at DESC)` |
| `idx_alert_events_tenant_status` | `CREATE INDEX idx_alert_events_tenant_status ON public.alert_events USING btree (tenant_id, status)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `alert_events_asset_id_fkey` | `asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `alert_events_rule_id_fkey` | `rule_id -> alert_rules(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `alert_events_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

### CHECK 约束

| 约束名 | 定义 |
|--------|------|
| `chk_alert_events_status` | `CHECK (((status)::text = ANY ((ARRAY['firing'::character varying, 'acknowledged'::character varying, 'resolved'::character varying])::text[])))` |

---

## alert_rules

**当前行数:** 5

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `name` | `varchar(255)` | NO |  |  |
| 4 | `metric_name` | `varchar(100)` | NO |  |  |
| 5 | `condition` | `jsonb` | NO | `'{}'::jsonb` |  |
| 6 | `severity` | `varchar(20)` | NO |  |  |
| 7 | `enabled` | `boolean` | NO | `true` |  |
| 8 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 9 | `sync_version` | `bigint` | NO | `0` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `alert_rules_pkey` | `CREATE UNIQUE INDEX alert_rules_pkey ON public.alert_rules USING btree (id)` |
| `idx_alert_rules_sync_version` | `CREATE INDEX idx_alert_rules_sync_version ON public.alert_rules USING btree (tenant_id, sync_version)` |
| `idx_alert_rules_tenant_id` | `CREATE INDEX idx_alert_rules_tenant_id ON public.alert_rules USING btree (tenant_id)` |
| `idx_alert_rules_unique_name` | `CREATE UNIQUE INDEX idx_alert_rules_unique_name ON public.alert_rules USING btree (tenant_id, name)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `alert_rules_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## asset_dependencies

**当前行数:** 12

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `source_asset_id` | `uuid` | NO |  |  |
| 4 | `target_asset_id` | `uuid` | NO |  |  |
| 5 | `dependency_type` | `varchar(50)` | NO | `'depends_on'::character varying` |  |
| 6 | `description` | `text` | YES |  |  |
| 7 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `asset_dependencies_pkey` | `CREATE UNIQUE INDEX asset_dependencies_pkey ON public.asset_dependencies USING btree (id)` |
| `asset_dependencies_source_asset_id_target_asset_id_dependen_key` | `CREATE UNIQUE INDEX asset_dependencies_source_asset_id_target_asset_id_dependen_key ON public.asset_dependencies USING btree (source_asset_id, target_asset_id, dependency_type)` |
| `idx_asset_deps_target` | `CREATE INDEX idx_asset_deps_target ON public.asset_dependencies USING btree (target_asset_id)` |
| `idx_asset_deps_tenant` | `CREATE INDEX idx_asset_deps_tenant ON public.asset_dependencies USING btree (tenant_id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `asset_dependencies_source_asset_id_fkey` | `source_asset_id -> assets(id) ON DELETE CASCADE ON UPDATE NO ACTION` |
| `asset_dependencies_target_asset_id_fkey` | `target_asset_id -> assets(id) ON DELETE RESTRICT ON UPDATE NO ACTION` |
| `asset_dependencies_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

### CHECK 约束

| 约束名 | 定义 |
|--------|------|
| `asset_dependencies_check` | `CHECK ((source_asset_id <> target_asset_id))` |

---

## asset_field_authorities

**当前行数:** 16

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `field_name` | `varchar(50)` | NO |  |  |
| 4 | `source_type` | `varchar(30)` | NO |  |  |
| 5 | `priority` | `integer` | NO |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `asset_field_authorities_pkey` | `CREATE UNIQUE INDEX asset_field_authorities_pkey ON public.asset_field_authorities USING btree (id)` |
| `asset_field_authorities_tenant_id_field_name_source_type_key` | `CREATE UNIQUE INDEX asset_field_authorities_tenant_id_field_name_source_type_key ON public.asset_field_authorities USING btree (tenant_id, field_name, source_type)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `asset_field_authorities_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## asset_location_history

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `asset_id` | `uuid` | NO |  |  |
| 4 | `from_rack_id` | `uuid` | YES |  |  |
| 5 | `to_rack_id` | `uuid` | YES |  |  |
| 6 | `detected_by` | `varchar(20)` | NO |  |  |
| 7 | `work_order_id` | `uuid` | YES |  |  |
| 8 | `detected_at` | `timestamp with time zone` | YES | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `asset_location_history_pkey` | `CREATE UNIQUE INDEX asset_location_history_pkey ON public.asset_location_history USING btree (id)` |
| `idx_location_history_asset` | `CREATE INDEX idx_location_history_asset ON public.asset_location_history USING btree (asset_id, detected_at DESC)` |
| `idx_location_history_tenant` | `CREATE INDEX idx_location_history_tenant ON public.asset_location_history USING btree (tenant_id, detected_at DESC)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `asset_location_history_asset_id_fkey` | `asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `asset_location_history_from_rack_id_fkey` | `from_rack_id -> racks(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `asset_location_history_to_rack_id_fkey` | `to_rack_id -> racks(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `asset_location_history_work_order_id_fkey` | `work_order_id -> work_orders(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## assets

**当前行数:** 43

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `asset_tag` | `varchar(100)` | NO |  |  |
| 4 | `property_number` | `varchar(100)` | YES |  |  |
| 5 | `control_number` | `varchar(100)` | YES |  |  |
| 6 | `name` | `varchar(255)` | NO |  |  |
| 7 | `type` | `varchar(50)` | NO |  |  |
| 8 | `sub_type` | `varchar(50)` | YES |  |  |
| 9 | `status` | `varchar(30)` | NO | `'inventoried'::character varying` |  |
| 10 | `bia_level` | `varchar(20)` | NO | `'normal'::character varying` |  |
| 11 | `location_id` | `uuid` | YES |  |  |
| 12 | `rack_id` | `uuid` | YES |  |  |
| 13 | `vendor` | `varchar(255)` | YES |  |  |
| 14 | `model` | `varchar(255)` | YES |  |  |
| 15 | `serial_number` | `varchar(255)` | YES |  |  |
| 16 | `attributes` | `jsonb` | NO | `'{}'::jsonb` |  |
| 17 | `tags` | `text[]` | YES |  |  |
| 18 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 19 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |
| 20 | `ip_address` | `varchar(50)` | YES |  |  |
| 21 | `deleted_at` | `timestamp with time zone` | YES |  |  |
| 22 | `sync_version` | `bigint` | NO | `0` |  |
| 23 | `bmc_ip` | `varchar(45)` | YES |  |  |
| 24 | `bmc_type` | `varchar(20)` | YES |  |  |
| 25 | `bmc_firmware` | `varchar(100)` | YES |  |  |
| 26 | `purchase_date` | `date` | YES |  |  |
| 27 | `purchase_cost` | `numeric(12,2)` | YES |  |  |
| 28 | `warranty_start` | `date` | YES |  |  |
| 29 | `warranty_end` | `date` | YES |  |  |
| 30 | `warranty_vendor` | `varchar(200)` | YES |  |  |
| 31 | `warranty_contract` | `varchar(100)` | YES |  |  |
| 32 | `expected_lifespan_months` | `integer` | YES |  |  |
| 33 | `eol_date` | `date` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `assets_asset_tag_key` | `CREATE UNIQUE INDEX assets_asset_tag_key ON public.assets USING btree (asset_tag)` |
| `assets_pkey` | `CREATE UNIQUE INDEX assets_pkey ON public.assets USING btree (id)` |
| `idx_assets_attributes` | `CREATE INDEX idx_assets_attributes ON public.assets USING gin (attributes)` |
| `idx_assets_bmc_ip` | `CREATE INDEX idx_assets_bmc_ip ON public.assets USING btree (bmc_ip) WHERE (bmc_ip IS NOT NULL)` |
| `idx_assets_control_number` | `CREATE UNIQUE INDEX idx_assets_control_number ON public.assets USING btree (tenant_id, control_number) WHERE (control_number IS NOT NULL)` |
| `idx_assets_ip_address` | `CREATE INDEX idx_assets_ip_address ON public.assets USING btree (tenant_id, ip_address)` |
| `idx_assets_location_id` | `CREATE INDEX idx_assets_location_id ON public.assets USING btree (location_id)` |
| `idx_assets_name_trgm` | `CREATE INDEX idx_assets_name_trgm ON public.assets USING gin (name gin_trgm_ops)` |
| `idx_assets_not_deleted` | `CREATE INDEX idx_assets_not_deleted ON public.assets USING btree (tenant_id) WHERE (deleted_at IS NULL)` |
| `idx_assets_property_number` | `CREATE UNIQUE INDEX idx_assets_property_number ON public.assets USING btree (tenant_id, property_number) WHERE (property_number IS NOT NULL)` |
| `idx_assets_rack_id` | `CREATE INDEX idx_assets_rack_id ON public.assets USING btree (rack_id)` |
| `idx_assets_serial_number` | `CREATE INDEX idx_assets_serial_number ON public.assets USING btree (serial_number)` |
| `idx_assets_sync_version` | `CREATE INDEX idx_assets_sync_version ON public.assets USING btree (tenant_id, sync_version)` |
| `idx_assets_tag_trgm` | `CREATE INDEX idx_assets_tag_trgm ON public.assets USING gin (asset_tag gin_trgm_ops)` |
| `idx_assets_tags` | `CREATE INDEX idx_assets_tags ON public.assets USING gin (tags)` |
| `idx_assets_tenant_id` | `CREATE INDEX idx_assets_tenant_id ON public.assets USING btree (tenant_id)` |
| `idx_assets_tenant_status` | `CREATE INDEX idx_assets_tenant_status ON public.assets USING btree (tenant_id, status)` |
| `idx_assets_tenant_type_subtype` | `CREATE INDEX idx_assets_tenant_type_subtype ON public.assets USING btree (tenant_id, type, sub_type)` |
| `idx_assets_warranty_end` | `CREATE INDEX idx_assets_warranty_end ON public.assets USING btree (warranty_end) WHERE ((warranty_end IS NOT NULL) AND (deleted_at IS NULL))` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `assets_location_id_fkey` | `location_id -> locations(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `assets_rack_id_fkey` | `rack_id -> racks(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `assets_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

### CHECK 约束

| 约束名 | 定义 |
|--------|------|
| `chk_assets_status` | `CHECK (((status)::text = ANY ((ARRAY['procurement'::character varying, 'inventoried'::character varying, 'deploying'::character varying, 'deployed'::character varying, 'operational'::character varying, 'active'::character varying, 'maintenance'::character varying, 'decommission'::character varying, 'retired'::character varying, 'disposed'::character varying, 'offline'::character varying])::text[])))` |

---

## audit_events

**当前行数:** 73

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `action` | `varchar(50)` | NO |  |  |
| 4 | `module` | `varchar(30)` | YES |  |  |
| 5 | `target_type` | `varchar(30)` | YES |  |  |
| 6 | `target_id` | `uuid` | YES |  |  |
| 7 | `operator_id` | `uuid` | YES |  |  |
| 8 | `diff` | `jsonb` | YES |  |  |
| 9 | `source` | `varchar(20)` | NO | `'web'::character varying` |  |
| 10 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `audit_events_pkey` | `CREATE UNIQUE INDEX audit_events_pkey ON public.audit_events USING btree (id)` |
| `idx_audit_events_operator` | `CREATE INDEX idx_audit_events_operator ON public.audit_events USING btree (operator_id)` |
| `idx_audit_events_target` | `CREATE INDEX idx_audit_events_target ON public.audit_events USING btree (target_type, target_id)` |
| `idx_audit_events_tenant_created` | `CREATE INDEX idx_audit_events_tenant_created ON public.audit_events USING btree (tenant_id, created_at DESC)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `audit_events_operator_id_fkey` | `operator_id -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `audit_events_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

### 触发器

| 触发器 | 定义 |
|--------|------|
| `audit_events_no_delete` | `DELETE BEFORE -> EXECUTE FUNCTION prevent_audit_mutation()` |
| `audit_events_no_update` | `UPDATE BEFORE -> EXECUTE FUNCTION prevent_audit_mutation()` |

---

## bia_assessments

**当前行数:** 4

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `system_name` | `varchar(255)` | NO |  |  |
| 4 | `system_code` | `varchar(100)` | NO |  |  |
| 5 | `owner` | `varchar(255)` | YES |  |  |
| 6 | `bia_score` | `integer` | NO | `0` |  |
| 7 | `tier` | `varchar(20)` | NO | `'normal'::character varying` |  |
| 8 | `rto_hours` | `numeric(10,2)` | YES |  |  |
| 9 | `rpo_minutes` | `numeric(10,2)` | YES |  |  |
| 10 | `mtpd_hours` | `numeric(10,2)` | YES |  |  |
| 11 | `data_compliance` | `boolean` | YES | `false` |  |
| 12 | `asset_compliance` | `boolean` | YES | `false` |  |
| 13 | `audit_compliance` | `boolean` | YES | `false` |  |
| 14 | `description` | `text` | YES |  |  |
| 15 | `last_assessed` | `timestamp with time zone` | YES | `now()` |  |
| 16 | `assessed_by` | `uuid` | YES |  |  |
| 17 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 18 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `bia_assessments_pkey` | `CREATE UNIQUE INDEX bia_assessments_pkey ON public.bia_assessments USING btree (id)` |
| `idx_bia_assessments_tenant` | `CREATE INDEX idx_bia_assessments_tenant ON public.bia_assessments USING btree (tenant_id)` |
| `idx_bia_assessments_tier` | `CREATE INDEX idx_bia_assessments_tier ON public.bia_assessments USING btree (tenant_id, tier)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `bia_assessments_assessed_by_fkey` | `assessed_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `bia_assessments_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## bia_dependencies

**当前行数:** 8

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `assessment_id` | `uuid` | NO |  |  |
| 4 | `asset_id` | `uuid` | NO |  |  |
| 5 | `dependency_type` | `varchar(50)` | NO | `'runs_on'::character varying` |  |
| 6 | `criticality` | `varchar(20)` | YES | `'high'::character varying` |  |
| 7 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `bia_dependencies_assessment_id_asset_id_key` | `CREATE UNIQUE INDEX bia_dependencies_assessment_id_asset_id_key ON public.bia_dependencies USING btree (assessment_id, asset_id)` |
| `bia_dependencies_pkey` | `CREATE UNIQUE INDEX bia_dependencies_pkey ON public.bia_dependencies USING btree (id)` |
| `idx_bia_deps_assessment` | `CREATE INDEX idx_bia_deps_assessment ON public.bia_dependencies USING btree (assessment_id)` |
| `idx_bia_deps_asset` | `CREATE INDEX idx_bia_deps_asset ON public.bia_dependencies USING btree (asset_id)` |
| `idx_bia_deps_tenant` | `CREATE INDEX idx_bia_deps_tenant ON public.bia_dependencies USING btree (tenant_id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `bia_dependencies_assessment_id_fkey` | `assessment_id -> bia_assessments(id) ON DELETE CASCADE ON UPDATE NO ACTION` |
| `bia_dependencies_asset_id_fkey` | `asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `bia_dependencies_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## bia_scoring_rules

**当前行数:** 4

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `tier_name` | `varchar(20)` | NO |  |  |
| 4 | `tier_level` | `integer` | NO |  |  |
| 5 | `display_name` | `varchar(100)` | NO |  |  |
| 6 | `min_score` | `integer` | NO |  |  |
| 7 | `max_score` | `integer` | NO |  |  |
| 8 | `rto_threshold` | `numeric(10,2)` | YES |  |  |
| 9 | `rpo_threshold` | `numeric(10,2)` | YES |  |  |
| 10 | `description` | `text` | YES |  |  |
| 11 | `color` | `varchar(20)` | YES |  |  |
| 12 | `icon` | `varchar(50)` | YES |  |  |
| 13 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `bia_scoring_rules_pkey` | `CREATE UNIQUE INDEX bia_scoring_rules_pkey ON public.bia_scoring_rules USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `bia_scoring_rules_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## credentials

**当前行数:** 3

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `name` | `varchar(100)` | NO |  |  |
| 4 | `type` | `varchar(30)` | NO |  |  |
| 5 | `params` | `bytea` | NO |  |  |
| 6 | `created_by` | `uuid` | YES |  |  |
| 7 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 8 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `credentials_pkey` | `CREATE UNIQUE INDEX credentials_pkey ON public.credentials USING btree (id)` |
| `credentials_tenant_id_name_key` | `CREATE UNIQUE INDEX credentials_tenant_id_name_key ON public.credentials USING btree (tenant_id, name)` |
| `idx_credentials_tenant` | `CREATE INDEX idx_credentials_tenant ON public.credentials USING btree (tenant_id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `credentials_created_by_fkey` | `created_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `credentials_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## departments

**当前行数:** 4

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `name` | `varchar(255)` | NO |  |  |
| 4 | `slug` | `varchar(100)` | NO |  |  |
| 5 | `permissions` | `jsonb` | NO | `'{}'::jsonb` |  |
| 6 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `departments_pkey` | `CREATE UNIQUE INDEX departments_pkey ON public.departments USING btree (id)` |
| `idx_departments_tenant_id` | `CREATE INDEX idx_departments_tenant_id ON public.departments USING btree (tenant_id)` |
| `uq_departments_tenant_slug` | `CREATE UNIQUE INDEX uq_departments_tenant_slug ON public.departments USING btree (tenant_id, slug)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `departments_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## discovered_assets

**当前行数:** 5

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `source` | `varchar(50)` | NO |  |  |
| 4 | `external_id` | `varchar(255)` | YES |  |  |
| 5 | `hostname` | `varchar(255)` | YES |  |  |
| 6 | `ip_address` | `varchar(50)` | YES |  |  |
| 7 | `raw_data` | `jsonb` | NO | `'{}'::jsonb` |  |
| 8 | `status` | `varchar(20)` | NO | `'pending'::character varying` |  |
| 9 | `matched_asset_id` | `uuid` | YES |  |  |
| 10 | `diff_details` | `jsonb` | YES |  |  |
| 11 | `discovered_at` | `timestamp with time zone` | NO | `now()` |  |
| 12 | `reviewed_by` | `uuid` | YES |  |  |
| 13 | `reviewed_at` | `timestamp with time zone` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `discovered_assets_pkey` | `CREATE UNIQUE INDEX discovered_assets_pkey ON public.discovered_assets USING btree (id)` |
| `idx_discovered_assets_status` | `CREATE INDEX idx_discovered_assets_status ON public.discovered_assets USING btree (tenant_id, status)` |
| `idx_discovered_assets_tenant` | `CREATE INDEX idx_discovered_assets_tenant ON public.discovered_assets USING btree (tenant_id)` |
| `idx_discovered_assets_unique_external` | `CREATE UNIQUE INDEX idx_discovered_assets_unique_external ON public.discovered_assets USING btree (tenant_id, source, external_id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `discovered_assets_matched_asset_id_fkey` | `matched_asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `discovered_assets_reviewed_by_fkey` | `reviewed_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `discovered_assets_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## discovery_candidates

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `task_id` | `uuid` | NO |  |  |
| 3 | `raw_data` | `jsonb` | NO |  |  |
| 4 | `matched_asset_id` | `uuid` | YES |  |  |
| 5 | `status` | `varchar(20)` | NO | `'pending'::character varying` |  |
| 6 | `reviewed_by` | `uuid` | YES |  |  |
| 7 | `reviewed_at` | `timestamp with time zone` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `discovery_candidates_pkey` | `CREATE UNIQUE INDEX discovery_candidates_pkey ON public.discovery_candidates USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `discovery_candidates_matched_asset_id_fkey` | `matched_asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `discovery_candidates_reviewed_by_fkey` | `reviewed_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `discovery_candidates_task_id_fkey` | `task_id -> discovery_tasks(id) ON DELETE CASCADE ON UPDATE NO ACTION` |

---

## discovery_tasks

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `type` | `varchar(30)` | NO |  |  |
| 4 | `status` | `varchar(20)` | NO | `'running'::character varying` |  |
| 5 | `config` | `jsonb` | YES |  |  |
| 6 | `stats` | `jsonb` | NO | `'{}'::jsonb` |  |
| 7 | `triggered_by` | `uuid` | YES |  |  |
| 8 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 9 | `completed_at` | `timestamp with time zone` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `discovery_tasks_pkey` | `CREATE UNIQUE INDEX discovery_tasks_pkey ON public.discovery_tasks USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `discovery_tasks_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `discovery_tasks_triggered_by_fkey` | `triggered_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## import_conflicts

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `asset_id` | `uuid` | NO |  |  |
| 4 | `source_type` | `varchar(30)` | NO |  |  |
| 5 | `field_name` | `varchar(50)` | NO |  |  |
| 6 | `current_value` | `text` | YES |  |  |
| 7 | `incoming_value` | `text` | YES |  |  |
| 8 | `status` | `varchar(20)` | NO | `'pending'::character varying` |  |
| 9 | `resolved_by` | `uuid` | YES |  |  |
| 10 | `resolved_at` | `timestamp with time zone` | YES |  |  |
| 11 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_import_conflicts_pending` | `CREATE INDEX idx_import_conflicts_pending ON public.import_conflicts USING btree (tenant_id, status) WHERE ((status)::text = 'pending'::text)` |
| `import_conflicts_pkey` | `CREATE UNIQUE INDEX import_conflicts_pkey ON public.import_conflicts USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `import_conflicts_asset_id_fkey` | `asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `import_conflicts_resolved_by_fkey` | `resolved_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `import_conflicts_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## import_jobs

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `type` | `varchar(20)` | NO |  |  |
| 4 | `filename` | `varchar(200)` | NO |  |  |
| 5 | `status` | `varchar(20)` | NO | `'parsing'::character varying` |  |
| 6 | `total_rows` | `integer` | YES |  |  |
| 7 | `processed_rows` | `integer` | NO | `0` |  |
| 8 | `stats` | `jsonb` | NO | `'{}'::jsonb` |  |
| 9 | `error_details` | `jsonb` | NO | `'[]'::jsonb` |  |
| 10 | `uploaded_by` | `uuid` | YES |  |  |
| 11 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 12 | `completed_at` | `timestamp with time zone` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_import_jobs_tenant_status` | `CREATE INDEX idx_import_jobs_tenant_status ON public.import_jobs USING btree (tenant_id, status)` |
| `import_jobs_pkey` | `CREATE UNIQUE INDEX import_jobs_pkey ON public.import_jobs USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `import_jobs_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `import_jobs_uploaded_by_fkey` | `uploaded_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## incidents

**当前行数:** 3

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `title` | `varchar(255)` | NO |  |  |
| 4 | `status` | `varchar(20)` | NO | `'open'::character varying` |  |
| 5 | `severity` | `varchar(20)` | NO |  |  |
| 6 | `started_at` | `timestamp with time zone` | NO | `now()` |  |
| 7 | `resolved_at` | `timestamp with time zone` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_incidents_tenant_id` | `CREATE INDEX idx_incidents_tenant_id ON public.incidents USING btree (tenant_id)` |
| `idx_incidents_tenant_status` | `CREATE INDEX idx_incidents_tenant_status ON public.incidents USING btree (tenant_id, status, started_at DESC)` |
| `incidents_pkey` | `CREATE UNIQUE INDEX incidents_pkey ON public.incidents USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `incidents_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## integration_adapters

**当前行数:** 6

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `name` | `varchar(100)` | NO |  |  |
| 4 | `type` | `varchar(30)` | NO |  |  |
| 5 | `direction` | `varchar(20)` | NO |  |  |
| 6 | `endpoint` | `varchar(500)` | YES |  |  |
| 7 | `config` | `jsonb` | YES | `'{}'::jsonb` |  |
| 8 | `enabled` | `boolean` | YES | `true` |  |
| 9 | `created_at` | `timestamp with time zone` | YES | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `integration_adapters_pkey` | `CREATE UNIQUE INDEX integration_adapters_pkey ON public.integration_adapters USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `integration_adapters_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## inventory_items

**当前行数:** 10

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `task_id` | `uuid` | NO |  |  |
| 3 | `asset_id` | `uuid` | YES |  |  |
| 4 | `rack_id` | `uuid` | YES |  |  |
| 5 | `expected` | `jsonb` | YES |  |  |
| 6 | `actual` | `jsonb` | YES |  |  |
| 7 | `status` | `varchar(20)` | NO | `'pending'::character varying` |  |
| 8 | `scanned_at` | `timestamp with time zone` | YES |  |  |
| 9 | `scanned_by` | `uuid` | YES |  |  |
| 10 | `sync_version` | `bigint` | NO | `0` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_inventory_items_asset_id` | `CREATE INDEX idx_inventory_items_asset_id ON public.inventory_items USING btree (asset_id)` |
| `idx_inventory_items_rack_id` | `CREATE INDEX idx_inventory_items_rack_id ON public.inventory_items USING btree (rack_id)` |
| `idx_inventory_items_status` | `CREATE INDEX idx_inventory_items_status ON public.inventory_items USING btree (status)` |
| `idx_inventory_items_sync_version` | `CREATE INDEX idx_inventory_items_sync_version ON public.inventory_items USING btree (task_id, sync_version)` |
| `idx_inventory_items_task_id` | `CREATE INDEX idx_inventory_items_task_id ON public.inventory_items USING btree (task_id)` |
| `idx_inventory_items_task_status` | `CREATE INDEX idx_inventory_items_task_status ON public.inventory_items USING btree (task_id, status)` |
| `inventory_items_pkey` | `CREATE UNIQUE INDEX inventory_items_pkey ON public.inventory_items USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `inventory_items_asset_id_fkey` | `asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `inventory_items_rack_id_fkey` | `rack_id -> racks(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `inventory_items_scanned_by_fkey` | `scanned_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `inventory_items_task_id_fkey` | `task_id -> inventory_tasks(id) ON DELETE CASCADE ON UPDATE NO ACTION` |

---

## inventory_notes

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `item_id` | `uuid` | NO |  |  |
| 3 | `author_id` | `uuid` | YES |  |  |
| 4 | `severity` | `varchar(20)` | NO | `'info'::character varying` |  |
| 5 | `text` | `text` | NO |  |  |
| 6 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_inventory_notes_item` | `CREATE INDEX idx_inventory_notes_item ON public.inventory_notes USING btree (item_id)` |
| `inventory_notes_pkey` | `CREATE UNIQUE INDEX inventory_notes_pkey ON public.inventory_notes USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `inventory_notes_author_id_fkey` | `author_id -> users(id) ON DELETE SET NULL ON UPDATE NO ACTION` |
| `inventory_notes_item_id_fkey` | `item_id -> inventory_items(id) ON DELETE CASCADE ON UPDATE NO ACTION` |

---

## inventory_scan_history

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `item_id` | `uuid` | NO |  |  |
| 3 | `scanned_at` | `timestamp with time zone` | NO | `now()` |  |
| 4 | `scanned_by` | `uuid` | YES |  |  |
| 5 | `method` | `varchar(20)` | NO |  |  |
| 6 | `result` | `varchar(20)` | NO |  |  |
| 7 | `note` | `text` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_scan_history_item_time` | `CREATE INDEX idx_scan_history_item_time ON public.inventory_scan_history USING btree (item_id, scanned_at DESC)` |
| `inventory_scan_history_pkey` | `CREATE UNIQUE INDEX inventory_scan_history_pkey ON public.inventory_scan_history USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `inventory_scan_history_item_id_fkey` | `item_id -> inventory_items(id) ON DELETE CASCADE ON UPDATE NO ACTION` |
| `inventory_scan_history_scanned_by_fkey` | `scanned_by -> users(id) ON DELETE SET NULL ON UPDATE NO ACTION` |

---

## inventory_tasks

**当前行数:** 4

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `code` | `varchar(100)` | NO |  |  |
| 4 | `name` | `varchar(255)` | NO |  |  |
| 5 | `scope_location_id` | `uuid` | YES |  |  |
| 6 | `status` | `varchar(20)` | NO | `'planned'::character varying` |  |
| 7 | `method` | `varchar(50)` | YES |  |  |
| 8 | `planned_date` | `date` | YES |  |  |
| 9 | `completed_date` | `date` | YES |  |  |
| 10 | `assigned_to` | `uuid` | YES |  |  |
| 11 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 12 | `deleted_at` | `timestamp with time zone` | YES |  |  |
| 13 | `sync_version` | `bigint` | NO | `0` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_inventory_tasks_deleted_at` | `CREATE INDEX idx_inventory_tasks_deleted_at ON public.inventory_tasks USING btree (deleted_at) WHERE (deleted_at IS NULL)` |
| `idx_inventory_tasks_sync_version` | `CREATE INDEX idx_inventory_tasks_sync_version ON public.inventory_tasks USING btree (tenant_id, sync_version)` |
| `idx_inventory_tasks_tenant_id` | `CREATE INDEX idx_inventory_tasks_tenant_id ON public.inventory_tasks USING btree (tenant_id)` |
| `inventory_tasks_code_key` | `CREATE UNIQUE INDEX inventory_tasks_code_key ON public.inventory_tasks USING btree (code)` |
| `inventory_tasks_pkey` | `CREATE UNIQUE INDEX inventory_tasks_pkey ON public.inventory_tasks USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `inventory_tasks_assigned_to_fkey` | `assigned_to -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `inventory_tasks_scope_location_id_fkey` | `scope_location_id -> locations(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `inventory_tasks_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

### CHECK 约束

| 约束名 | 定义 |
|--------|------|
| `chk_inventory_tasks_status` | `CHECK (((status)::text = ANY ((ARRAY['planned'::character varying, 'in_progress'::character varying, 'completed'::character varying])::text[])))` |

---

## locations

**当前行数:** 24

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `name` | `varchar(255)` | NO |  |  |
| 4 | `name_en` | `varchar(255)` | YES |  |  |
| 5 | `slug` | `varchar(100)` | NO |  |  |
| 6 | `level` | `varchar(20)` | NO |  |  |
| 7 | `parent_id` | `uuid` | YES |  |  |
| 8 | `path` | `ltree` | YES |  |  |
| 9 | `status` | `varchar(20)` | NO | `'active'::character varying` |  |
| 10 | `metadata` | `jsonb` | NO | `'{}'::jsonb` |  |
| 11 | `sort_order` | `integer` | NO | `0` |  |
| 12 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 13 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |
| 14 | `deleted_at` | `timestamp with time zone` | YES |  |  |
| 15 | `sync_version` | `bigint` | NO | `0` |  |
| 16 | `latitude` | `double precision` | YES |  |  |
| 17 | `longitude` | `double precision` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_locations_not_deleted` | `CREATE INDEX idx_locations_not_deleted ON public.locations USING btree (tenant_id) WHERE (deleted_at IS NULL)` |
| `idx_locations_parent_id` | `CREATE INDEX idx_locations_parent_id ON public.locations USING btree (parent_id)` |
| `idx_locations_path` | `CREATE INDEX idx_locations_path ON public.locations USING gist (path)` |
| `idx_locations_sync_version` | `CREATE INDEX idx_locations_sync_version ON public.locations USING btree (tenant_id, sync_version)` |
| `idx_locations_tenant_id` | `CREATE INDEX idx_locations_tenant_id ON public.locations USING btree (tenant_id)` |
| `idx_locations_tenant_slug` | `CREATE INDEX idx_locations_tenant_slug ON public.locations USING btree (tenant_id, slug)` |
| `locations_pkey` | `CREATE UNIQUE INDEX locations_pkey ON public.locations USING btree (id)` |
| `uq_locations_tenant_slug_level` | `CREATE UNIQUE INDEX uq_locations_tenant_slug_level ON public.locations USING btree (tenant_id, slug, level)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `locations_parent_id_fkey` | `parent_id -> locations(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `locations_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## mac_address_cache

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `mac_address` | `varchar(17)` | NO |  |  |
| 4 | `switch_asset_id` | `uuid` | NO |  |  |
| 5 | `port_name` | `varchar(50)` | NO |  |  |
| 6 | `vlan_id` | `integer` | YES |  |  |
| 7 | `asset_id` | `uuid` | YES |  |  |
| 8 | `detected_rack_id` | `uuid` | YES |  |  |
| 9 | `first_seen` | `timestamp with time zone` | YES | `now()` |  |
| 10 | `last_seen` | `timestamp with time zone` | YES | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_mac_cache_asset` | `CREATE INDEX idx_mac_cache_asset ON public.mac_address_cache USING btree (asset_id)` |
| `idx_mac_cache_switch` | `CREATE INDEX idx_mac_cache_switch ON public.mac_address_cache USING btree (switch_asset_id, port_name)` |
| `mac_address_cache_pkey` | `CREATE UNIQUE INDEX mac_address_cache_pkey ON public.mac_address_cache USING btree (id)` |
| `mac_address_cache_tenant_id_mac_address_key` | `CREATE UNIQUE INDEX mac_address_cache_tenant_id_mac_address_key ON public.mac_address_cache USING btree (tenant_id, mac_address)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `mac_address_cache_asset_id_fkey` | `asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `mac_address_cache_detected_rack_id_fkey` | `detected_rack_id -> racks(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `mac_address_cache_switch_asset_id_fkey` | `switch_asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## metrics

**当前行数:** 26495

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `time` | `timestamp with time zone` | NO |  |  |
| 2 | `asset_id` | `uuid` | YES |  |  |
| 3 | `tenant_id` | `uuid` | YES |  |  |
| 4 | `name` | `varchar(100)` | NO |  |  |
| 5 | `value` | `double precision` | YES |  |  |
| 6 | `labels` | `jsonb` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_metrics_asset_name_time` | `CREATE INDEX idx_metrics_asset_name_time ON public.metrics USING btree (asset_id, name, "time" DESC)` |
| `idx_metrics_labels` | `CREATE INDEX idx_metrics_labels ON public.metrics USING gin (labels)` |
| `idx_metrics_tenant_time` | `CREATE INDEX idx_metrics_tenant_time ON public.metrics USING btree (tenant_id, "time" DESC)` |
| `metrics_time_idx` | `CREATE INDEX metrics_time_idx ON public.metrics USING btree ("time" DESC)` |

---

## notifications

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `user_id` | `uuid` | NO |  |  |
| 4 | `type` | `varchar(50)` | NO |  |  |
| 5 | `title` | `varchar(255)` | NO |  |  |
| 6 | `body` | `text` | YES |  |  |
| 7 | `resource_type` | `varchar(50)` | YES |  |  |
| 8 | `resource_id` | `uuid` | YES |  |  |
| 9 | `is_read` | `boolean` | NO | `false` |  |
| 10 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_notifications_tenant` | `CREATE INDEX idx_notifications_tenant ON public.notifications USING btree (tenant_id)` |
| `idx_notifications_user_unread` | `CREATE INDEX idx_notifications_user_unread ON public.notifications USING btree (user_id, is_read) WHERE (is_read = false)` |
| `notifications_pkey` | `CREATE UNIQUE INDEX notifications_pkey ON public.notifications USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `notifications_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `notifications_user_id_fkey` | `user_id -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## prediction_models

**当前行数:** 3

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `name` | `varchar(100)` | NO |  |  |
| 3 | `type` | `varchar(30)` | NO |  |  |
| 4 | `provider` | `varchar(30)` | NO |  |  |
| 5 | `config` | `jsonb` | NO | `'{}'::jsonb` |  |
| 6 | `enabled` | `boolean` | NO | `true` |  |
| 7 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `prediction_models_pkey` | `CREATE UNIQUE INDEX prediction_models_pkey ON public.prediction_models USING btree (id)` |

---

## prediction_results

**当前行数:** 5

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `model_id` | `uuid` | NO |  |  |
| 4 | `asset_id` | `uuid` | NO |  |  |
| 5 | `prediction_type` | `varchar(30)` | NO |  |  |
| 6 | `result` | `jsonb` | NO |  |  |
| 7 | `severity` | `varchar(20)` | YES |  |  |
| 8 | `recommended_action` | `text` | YES |  |  |
| 9 | `expires_at` | `timestamp with time zone` | YES |  |  |
| 10 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_prediction_results_asset` | `CREATE INDEX idx_prediction_results_asset ON public.prediction_results USING btree (asset_id)` |
| `idx_prediction_results_tenant` | `CREATE INDEX idx_prediction_results_tenant ON public.prediction_results USING btree (tenant_id, created_at DESC)` |
| `prediction_results_pkey` | `CREATE UNIQUE INDEX prediction_results_pkey ON public.prediction_results USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `prediction_results_asset_id_fkey` | `asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `prediction_results_model_id_fkey` | `model_id -> prediction_models(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `prediction_results_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## quality_rules

**当前行数:** 5

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `ci_type` | `varchar(50)` | YES |  |  |
| 4 | `dimension` | `varchar(20)` | NO |  |  |
| 5 | `field_name` | `varchar(50)` | NO |  |  |
| 6 | `rule_type` | `varchar(20)` | NO |  |  |
| 7 | `rule_config` | `jsonb` | YES | `'{}'::jsonb` |  |
| 8 | `weight` | `integer` | YES | `10` |  |
| 9 | `enabled` | `boolean` | YES | `true` |  |
| 10 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `quality_rules_pkey` | `CREATE UNIQUE INDEX quality_rules_pkey ON public.quality_rules USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `quality_rules_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## quality_scores

**当前行数:** 60

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `asset_id` | `uuid` | NO |  |  |
| 4 | `completeness` | `numeric(5,2)` | YES | `0` |  |
| 5 | `accuracy` | `numeric(5,2)` | YES | `0` |  |
| 6 | `timeliness` | `numeric(5,2)` | YES | `0` |  |
| 7 | `consistency` | `numeric(5,2)` | YES | `0` |  |
| 8 | `total_score` | `numeric(5,2)` | YES | `0` |  |
| 9 | `issue_details` | `jsonb` | YES | `'[]'::jsonb` |  |
| 10 | `scan_date` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_quality_scores_asset` | `CREATE INDEX idx_quality_scores_asset ON public.quality_scores USING btree (asset_id)` |
| `idx_quality_scores_date` | `CREATE INDEX idx_quality_scores_date ON public.quality_scores USING btree (scan_date DESC)` |
| `idx_quality_scores_tenant_id` | `CREATE INDEX idx_quality_scores_tenant_id ON public.quality_scores USING btree (tenant_id)` |
| `quality_scores_pkey` | `CREATE UNIQUE INDEX quality_scores_pkey ON public.quality_scores USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `quality_scores_asset_id_fkey` | `asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `quality_scores_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## rack_network_connections

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `rack_id` | `uuid` | NO |  |  |
| 4 | `source_port` | `varchar(50)` | NO |  |  |
| 5 | `connected_asset_id` | `uuid` | YES |  |  |
| 6 | `external_device` | `varchar(255)` | YES |  |  |
| 7 | `speed` | `varchar(20)` | YES |  |  |
| 8 | `status` | `varchar(20)` | YES | `'UP'::character varying` |  |
| 9 | `vlans` | `integer[]` | YES |  |  |
| 10 | `connection_type` | `varchar(50)` | YES | `'network'::character varying` |  |
| 11 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 12 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_rack_net_conn_asset` | `CREATE INDEX idx_rack_net_conn_asset ON public.rack_network_connections USING btree (connected_asset_id)` |
| `idx_rack_net_conn_rack` | `CREATE INDEX idx_rack_net_conn_rack ON public.rack_network_connections USING btree (rack_id)` |
| `idx_rack_net_conn_tenant` | `CREATE INDEX idx_rack_net_conn_tenant ON public.rack_network_connections USING btree (tenant_id)` |
| `idx_rack_net_conn_vlans` | `CREATE INDEX idx_rack_net_conn_vlans ON public.rack_network_connections USING gin (vlans)` |
| `rack_network_connections_pkey` | `CREATE UNIQUE INDEX rack_network_connections_pkey ON public.rack_network_connections USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `rack_network_connections_connected_asset_id_fkey` | `connected_asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `rack_network_connections_rack_id_fkey` | `rack_id -> racks(id) ON DELETE CASCADE ON UPDATE NO ACTION` |
| `rack_network_connections_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

### CHECK 约束

| 约束名 | 定义 |
|--------|------|
| `rack_network_connections_check` | `CHECK ((NOT ((connected_asset_id IS NOT NULL) AND (external_device IS NOT NULL))))` |

---

## rack_slots

**当前行数:** 19

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `rack_id` | `uuid` | NO |  |  |
| 3 | `asset_id` | `uuid` | NO |  |  |
| 4 | `start_u` | `integer` | NO |  |  |
| 5 | `end_u` | `integer` | NO |  |  |
| 6 | `side` | `varchar(5)` | NO | `'front'::character varying` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_rack_slots_asset_id` | `CREATE INDEX idx_rack_slots_asset_id ON public.rack_slots USING btree (asset_id)` |
| `rack_slots_pkey` | `CREATE UNIQUE INDEX rack_slots_pkey ON public.rack_slots USING btree (id)` |
| `rack_slots_rack_id_start_u_side_key` | `CREATE UNIQUE INDEX rack_slots_rack_id_start_u_side_key ON public.rack_slots USING btree (rack_id, start_u, side)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `rack_slots_asset_id_fkey` | `asset_id -> assets(id) ON DELETE SET NULL ON UPDATE NO ACTION` |
| `rack_slots_rack_id_fkey` | `rack_id -> racks(id) ON DELETE CASCADE ON UPDATE NO ACTION` |

### CHECK 约束

| 约束名 | 定义 |
|--------|------|
| `rack_slots_check` | `CHECK ((end_u >= start_u))` |

---

## racks

**当前行数:** 13

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `location_id` | `uuid` | NO |  |  |
| 4 | `name` | `varchar(255)` | NO |  |  |
| 5 | `row_label` | `varchar(50)` | YES |  |  |
| 6 | `total_u` | `integer` | NO | `42` |  |
| 7 | `power_capacity_kw` | `numeric(8,2)` | YES |  |  |
| 8 | `status` | `varchar(20)` | NO | `'active'::character varying` |  |
| 9 | `tags` | `text[]` | YES |  |  |
| 10 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 11 | `deleted_at` | `timestamp with time zone` | YES |  |  |
| 12 | `sync_version` | `bigint` | NO | `0` |  |
| 13 | `updated_at` | `timestamp with time zone` | YES | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_racks_location_id` | `CREATE INDEX idx_racks_location_id ON public.racks USING btree (location_id)` |
| `idx_racks_not_deleted` | `CREATE INDEX idx_racks_not_deleted ON public.racks USING btree (tenant_id, location_id) WHERE (deleted_at IS NULL)` |
| `idx_racks_sync_version` | `CREATE INDEX idx_racks_sync_version ON public.racks USING btree (tenant_id, sync_version)` |
| `idx_racks_tenant_id` | `CREATE INDEX idx_racks_tenant_id ON public.racks USING btree (tenant_id)` |
| `idx_racks_unique_name_per_location` | `CREATE UNIQUE INDEX idx_racks_unique_name_per_location ON public.racks USING btree (tenant_id, location_id, name) WHERE (deleted_at IS NULL)` |
| `racks_pkey` | `CREATE UNIQUE INDEX racks_pkey ON public.racks USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `racks_location_id_fkey` | `location_id -> locations(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `racks_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## rca_analyses

**当前行数:** 2

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `incident_id` | `uuid` | NO |  |  |
| 4 | `model_id` | `uuid` | YES |  |  |
| 5 | `reasoning` | `jsonb` | NO |  |  |
| 6 | `conclusion_asset_id` | `uuid` | YES |  |  |
| 7 | `confidence` | `numeric(3,2)` | YES |  |  |
| 8 | `human_verified` | `boolean` | YES | `false` |  |
| 9 | `verified_by` | `uuid` | YES |  |  |
| 10 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_rca_analyses_incident_id` | `CREATE INDEX idx_rca_analyses_incident_id ON public.rca_analyses USING btree (incident_id)` |
| `idx_rca_analyses_tenant_id` | `CREATE INDEX idx_rca_analyses_tenant_id ON public.rca_analyses USING btree (tenant_id)` |
| `rca_analyses_pkey` | `CREATE UNIQUE INDEX rca_analyses_pkey ON public.rca_analyses USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `rca_analyses_conclusion_asset_id_fkey` | `conclusion_asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `rca_analyses_incident_id_fkey` | `incident_id -> incidents(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `rca_analyses_model_id_fkey` | `model_id -> prediction_models(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `rca_analyses_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `rca_analyses_verified_by_fkey` | `verified_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## roles

**当前行数:** 3

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | YES |  |  |
| 3 | `name` | `varchar(100)` | NO |  |  |
| 4 | `description` | `text` | YES |  |  |
| 5 | `permissions` | `jsonb` | NO | `'{}'::jsonb` |  |
| 6 | `is_system` | `boolean` | NO | `false` |  |
| 7 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_roles_tenant_id` | `CREATE INDEX idx_roles_tenant_id ON public.roles USING btree (tenant_id)` |
| `roles_pkey` | `CREATE UNIQUE INDEX roles_pkey ON public.roles USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `roles_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## scan_targets

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `name` | `varchar(200)` | NO |  |  |
| 4 | `cidrs` | `text[]` | NO |  |  |
| 5 | `collector_type` | `varchar(30)` | NO |  |  |
| 6 | `credential_id` | `uuid` | NO |  |  |
| 7 | `mode` | `varchar(20)` | NO | `'smart'::character varying` |  |
| 8 | `location_id` | `uuid` | YES |  |  |
| 9 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 10 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_scan_targets_tenant` | `CREATE INDEX idx_scan_targets_tenant ON public.scan_targets USING btree (tenant_id)` |
| `scan_targets_pkey` | `CREATE UNIQUE INDEX scan_targets_pkey ON public.scan_targets USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `scan_targets_credential_id_fkey` | `credential_id -> credentials(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `scan_targets_location_id_fkey` | `location_id -> locations(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `scan_targets_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## schema_migrations

**当前行数:** 16

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `version` | `bigint` | NO |  |  |
| 2 | `dirty` | `boolean` | NO |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `schema_migrations_pkey` | `CREATE UNIQUE INDEX schema_migrations_pkey ON public.schema_migrations USING btree (version)` |

---

## sensors

**当前行数:** 1

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `asset_id` | `uuid` | YES |  |  |
| 4 | `name` | `varchar(200)` | NO |  |  |
| 5 | `type` | `varchar(50)` | NO |  |  |
| 6 | `location` | `varchar(200)` | YES |  |  |
| 7 | `polling_interval` | `integer` | NO | `30` |  |
| 8 | `enabled` | `boolean` | NO | `true` |  |
| 9 | `status` | `varchar(20)` | NO | `'offline'::character varying` |  |
| 10 | `last_heartbeat` | `timestamp with time zone` | YES |  |  |
| 11 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 12 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_sensors_asset` | `CREATE INDEX idx_sensors_asset ON public.sensors USING btree (asset_id)` |
| `idx_sensors_tenant` | `CREATE INDEX idx_sensors_tenant ON public.sensors USING btree (tenant_id)` |
| `sensors_pkey` | `CREATE UNIQUE INDEX sensors_pkey ON public.sensors USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `sensors_asset_id_fkey` | `asset_id -> assets(id) ON DELETE SET NULL ON UPDATE NO ACTION` |
| `sensors_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## switch_port_mapping

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `switch_asset_id` | `uuid` | NO |  |  |
| 4 | `port_name` | `varchar(50)` | NO |  |  |
| 5 | `connected_rack_id` | `uuid` | YES |  |  |
| 6 | `connected_u_position` | `integer` | YES |  |  |
| 7 | `description` | `text` | YES |  |  |
| 8 | `updated_at` | `timestamp with time zone` | YES | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_switch_port_rack` | `CREATE INDEX idx_switch_port_rack ON public.switch_port_mapping USING btree (connected_rack_id)` |
| `switch_port_mapping_pkey` | `CREATE UNIQUE INDEX switch_port_mapping_pkey ON public.switch_port_mapping USING btree (id)` |
| `switch_port_mapping_tenant_id_switch_asset_id_port_name_key` | `CREATE UNIQUE INDEX switch_port_mapping_tenant_id_switch_asset_id_port_name_key ON public.switch_port_mapping USING btree (tenant_id, switch_asset_id, port_name)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `switch_port_mapping_connected_rack_id_fkey` | `connected_rack_id -> racks(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `switch_port_mapping_switch_asset_id_fkey` | `switch_asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `switch_port_mapping_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## sync_conflicts

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `entity_type` | `varchar(50)` | NO |  |  |
| 4 | `entity_id` | `uuid` | NO |  |  |
| 5 | `local_version` | `bigint` | NO |  |  |
| 6 | `remote_version` | `bigint` | NO |  |  |
| 7 | `local_diff` | `jsonb` | NO |  |  |
| 8 | `remote_diff` | `jsonb` | NO |  |  |
| 9 | `resolution` | `varchar(20)` | YES | `'pending'::character varying` |  |
| 10 | `resolved_by` | `uuid` | YES |  |  |
| 11 | `resolved_at` | `timestamp with time zone` | YES |  |  |
| 12 | `created_at` | `timestamp with time zone` | YES | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_sync_conflicts_pending` | `CREATE INDEX idx_sync_conflicts_pending ON public.sync_conflicts USING btree (tenant_id, resolution) WHERE ((resolution)::text = 'pending'::text)` |
| `sync_conflicts_pkey` | `CREATE UNIQUE INDEX sync_conflicts_pkey ON public.sync_conflicts USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `sync_conflicts_resolved_by_fkey` | `resolved_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## sync_state

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `node_id` | `varchar(100)` | NO |  |  |
| 3 | `tenant_id` | `uuid` | NO |  |  |
| 4 | `entity_type` | `varchar(50)` | NO |  |  |
| 5 | `last_sync_version` | `bigint` | YES | `0` |  |
| 6 | `last_sync_at` | `timestamp with time zone` | YES |  |  |
| 7 | `status` | `varchar(20)` | YES | `'active'::character varying` |  |
| 8 | `error_message` | `text` | YES |  |  |
| 9 | `created_at` | `timestamp with time zone` | YES | `now()` |  |
| 10 | `updated_at` | `timestamp with time zone` | YES | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `sync_state_node_id_entity_type_key` | `CREATE UNIQUE INDEX sync_state_node_id_entity_type_key ON public.sync_state USING btree (node_id, entity_type)` |
| `sync_state_pkey` | `CREATE UNIQUE INDEX sync_state_pkey ON public.sync_state USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `sync_state_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## tenants

**当前行数:** 1

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `name` | `varchar(255)` | NO |  |  |
| 3 | `slug` | `varchar(100)` | NO |  |  |
| 4 | `status` | `varchar(20)` | NO | `'active'::character varying` |  |
| 5 | `settings` | `jsonb` | NO | `'{}'::jsonb` |  |
| 6 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 7 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `tenants_pkey` | `CREATE UNIQUE INDEX tenants_pkey ON public.tenants USING btree (id)` |
| `tenants_slug_key` | `CREATE UNIQUE INDEX tenants_slug_key ON public.tenants USING btree (slug)` |

---

## upgrade_rules

**当前行数:** 8

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `asset_type` | `varchar(50)` | NO |  |  |
| 4 | `category` | `varchar(30)` | NO |  |  |
| 5 | `metric_name` | `varchar(100)` | NO |  |  |
| 6 | `threshold` | `numeric(10,2)` | NO |  |  |
| 7 | `duration_days` | `integer` | NO | `7` |  |
| 8 | `priority` | `varchar(20)` | NO | `'medium'::character varying` |  |
| 9 | `recommendation` | `text` | NO |  |  |
| 10 | `enabled` | `boolean` | NO | `true` |  |
| 11 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 12 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_upgrade_rules_tenant` | `CREATE INDEX idx_upgrade_rules_tenant ON public.upgrade_rules USING btree (tenant_id)` |
| `upgrade_rules_pkey` | `CREATE UNIQUE INDEX upgrade_rules_pkey ON public.upgrade_rules USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `upgrade_rules_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## user_roles

**当前行数:** 3

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `user_id` | `uuid` | NO |  |  |
| 2 | `role_id` | `uuid` | NO |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_user_roles_role_id` | `CREATE INDEX idx_user_roles_role_id ON public.user_roles USING btree (role_id)` |
| `user_roles_pkey` | `CREATE UNIQUE INDEX user_roles_pkey ON public.user_roles USING btree (user_id, role_id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `user_roles_role_id_fkey` | `role_id -> roles(id) ON DELETE CASCADE ON UPDATE NO ACTION` |
| `user_roles_user_id_fkey` | `user_id -> users(id) ON DELETE CASCADE ON UPDATE NO ACTION` |

---

## user_sessions

**当前行数:** 33

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `user_id` | `uuid` | NO |  |  |
| 3 | `ip_address` | `varchar(50)` | YES |  |  |
| 4 | `user_agent` | `text` | YES |  |  |
| 5 | `device_type` | `varchar(30)` | YES |  |  |
| 6 | `browser` | `varchar(50)` | YES |  |  |
| 7 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 8 | `last_active_at` | `timestamp with time zone` | NO | `now()` |  |
| 9 | `expired_at` | `timestamp with time zone` | YES |  |  |
| 10 | `is_current` | `boolean` | NO | `false` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_user_sessions_user` | `CREATE INDEX idx_user_sessions_user ON public.user_sessions USING btree (user_id, created_at DESC)` |
| `user_sessions_pkey` | `CREATE UNIQUE INDEX user_sessions_pkey ON public.user_sessions USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `user_sessions_user_id_fkey` | `user_id -> users(id) ON DELETE CASCADE ON UPDATE NO ACTION` |

---

## users

**当前行数:** 7

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `dept_id` | `uuid` | YES |  |  |
| 4 | `username` | `varchar(100)` | NO |  |  |
| 5 | `display_name` | `varchar(255)` | NO |  |  |
| 6 | `email` | `varchar(255)` | NO |  |  |
| 7 | `phone` | `varchar(50)` | YES |  |  |
| 8 | `password_hash` | `text` | NO |  |  |
| 9 | `status` | `varchar(20)` | NO | `'active'::character varying` |  |
| 10 | `source` | `varchar(20)` | NO | `'local'::character varying` |  |
| 11 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 12 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |
| 13 | `last_login_at` | `timestamp with time zone` | YES |  |  |
| 14 | `last_login_ip` | `varchar(50)` | YES |  |  |
| 15 | `deleted_at` | `timestamp with time zone` | YES |  |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_users_dept_id` | `CREATE INDEX idx_users_dept_id ON public.users USING btree (dept_id)` |
| `idx_users_not_deleted` | `CREATE INDEX idx_users_not_deleted ON public.users USING btree (tenant_id) WHERE (deleted_at IS NULL)` |
| `idx_users_tenant_id` | `CREATE INDEX idx_users_tenant_id ON public.users USING btree (tenant_id)` |
| `users_pkey` | `CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id)` |
| `users_username_key` | `CREATE UNIQUE INDEX users_username_key ON public.users USING btree (username)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `users_dept_id_fkey` | `dept_id -> departments(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `users_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## webhook_deliveries

**当前行数:** 10

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `subscription_id` | `uuid` | NO |  |  |
| 3 | `event_type` | `varchar(50)` | NO |  |  |
| 4 | `payload` | `jsonb` | NO |  |  |
| 5 | `status_code` | `integer` | YES |  |  |
| 6 | `response_body` | `text` | YES |  |  |
| 7 | `delivered_at` | `timestamp with time zone` | YES | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_webhook_deliveries_sub_delivered` | `CREATE INDEX idx_webhook_deliveries_sub_delivered ON public.webhook_deliveries USING btree (subscription_id, delivered_at DESC)` |
| `webhook_deliveries_pkey` | `CREATE UNIQUE INDEX webhook_deliveries_pkey ON public.webhook_deliveries USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `webhook_deliveries_subscription_id_fkey` | `subscription_id -> webhook_subscriptions(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## webhook_subscriptions

**当前行数:** 4

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `name` | `varchar(100)` | NO |  |  |
| 4 | `url` | `varchar(500)` | NO |  |  |
| 5 | `secret` | `varchar(200)` | YES |  |  |
| 6 | `events` | `text[]` | NO |  |  |
| 7 | `enabled` | `boolean` | YES | `true` |  |
| 8 | `created_at` | `timestamp with time zone` | YES | `now()` |  |
| 9 | `filter_bia` | `text[]` | YES | `'{}'::text[]` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_webhooks_enabled` | `CREATE INDEX idx_webhooks_enabled ON public.webhook_subscriptions USING btree (enabled) WHERE (enabled = true)` |
| `webhook_subscriptions_pkey` | `CREATE UNIQUE INDEX webhook_subscriptions_pkey ON public.webhook_subscriptions USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `webhook_subscriptions_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

---

## work_order_comments

**当前行数:** 0

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `order_id` | `uuid` | NO |  |  |
| 3 | `author_id` | `uuid` | YES |  |  |
| 4 | `text` | `text` | NO |  |  |
| 5 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_wo_comments_order` | `CREATE INDEX idx_wo_comments_order ON public.work_order_comments USING btree (order_id)` |
| `work_order_comments_pkey` | `CREATE UNIQUE INDEX work_order_comments_pkey ON public.work_order_comments USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `work_order_comments_author_id_fkey` | `author_id -> users(id) ON DELETE SET NULL ON UPDATE NO ACTION` |
| `work_order_comments_order_id_fkey` | `order_id -> work_orders(id) ON DELETE CASCADE ON UPDATE NO ACTION` |

---

## work_order_logs

**当前行数:** 34

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `order_id` | `uuid` | NO |  |  |
| 3 | `action` | `varchar(50)` | NO |  |  |
| 4 | `from_status` | `varchar(30)` | YES |  |  |
| 5 | `to_status` | `varchar(30)` | YES |  |  |
| 6 | `operator_id` | `uuid` | YES |  |  |
| 7 | `comment` | `text` | YES |  |  |
| 8 | `created_at` | `timestamp with time zone` | NO | `now()` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_work_order_logs_operator_id` | `CREATE INDEX idx_work_order_logs_operator_id ON public.work_order_logs USING btree (operator_id)` |
| `idx_work_order_logs_order_id` | `CREATE INDEX idx_work_order_logs_order_id ON public.work_order_logs USING btree (order_id)` |
| `work_order_logs_pkey` | `CREATE UNIQUE INDEX work_order_logs_pkey ON public.work_order_logs USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `work_order_logs_operator_id_fkey` | `operator_id -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `work_order_logs_order_id_fkey` | `order_id -> work_orders(id) ON DELETE CASCADE ON UPDATE NO ACTION` |

---

## work_orders

**当前行数:** 44

### 列定义

| # | 列名 | 类型 | 可空 | 默认值 | 说明 |
|---|------|------|------|--------|------|
| 1 | `id` | `uuid` | NO | `gen_random_uuid()` |  |
| 2 | `tenant_id` | `uuid` | NO |  |  |
| 3 | `code` | `varchar(100)` | NO |  |  |
| 4 | `title` | `varchar(255)` | NO |  |  |
| 5 | `type` | `varchar(50)` | NO |  |  |
| 6 | `status` | `varchar(30)` | NO | `'draft'::character varying` |  |
| 7 | `priority` | `varchar(20)` | NO | `'medium'::character varying` |  |
| 8 | `location_id` | `uuid` | YES |  |  |
| 9 | `asset_id` | `uuid` | YES |  |  |
| 10 | `requestor_id` | `uuid` | YES |  |  |
| 11 | `assignee_id` | `uuid` | YES |  |  |
| 12 | `description` | `text` | YES |  |  |
| 13 | `reason` | `text` | YES |  |  |
| 14 | `prediction_id` | `uuid` | YES |  |  |
| 15 | `scheduled_start` | `timestamp with time zone` | YES |  |  |
| 16 | `scheduled_end` | `timestamp with time zone` | YES |  |  |
| 17 | `actual_start` | `timestamp with time zone` | YES |  |  |
| 18 | `actual_end` | `timestamp with time zone` | YES |  |  |
| 19 | `created_at` | `timestamp with time zone` | NO | `now()` |  |
| 20 | `updated_at` | `timestamp with time zone` | NO | `now()` |  |
| 21 | `deleted_at` | `timestamp with time zone` | YES |  |  |
| 22 | `approved_at` | `timestamp with time zone` | YES |  |  |
| 23 | `approved_by` | `uuid` | YES |  |  |
| 24 | `sla_deadline` | `timestamp with time zone` | YES |  |  |
| 25 | `sla_warning_sent` | `boolean` | NO | `false` |  |
| 26 | `sla_breached` | `boolean` | NO | `false` |  |
| 27 | `sync_version` | `bigint` | NO | `0` |  |
| 28 | `execution_status` | `varchar(20)` | NO | `'pending'::character varying` |  |
| 29 | `governance_status` | `varchar(20)` | NO | `'submitted'::character varying` |  |

### 索引

| 索引名 | 定义 |
|--------|------|
| `idx_work_orders_asset_id` | `CREATE INDEX idx_work_orders_asset_id ON public.work_orders USING btree (asset_id)` |
| `idx_work_orders_assignee_id` | `CREATE INDEX idx_work_orders_assignee_id ON public.work_orders USING btree (assignee_id)` |
| `idx_work_orders_deleted_at` | `CREATE INDEX idx_work_orders_deleted_at ON public.work_orders USING btree (deleted_at) WHERE (deleted_at IS NULL)` |
| `idx_work_orders_location_id` | `CREATE INDEX idx_work_orders_location_id ON public.work_orders USING btree (location_id)` |
| `idx_work_orders_requestor_id` | `CREATE INDEX idx_work_orders_requestor_id ON public.work_orders USING btree (requestor_id)` |
| `idx_work_orders_sync_version` | `CREATE INDEX idx_work_orders_sync_version ON public.work_orders USING btree (tenant_id, sync_version)` |
| `idx_work_orders_tenant_id` | `CREATE INDEX idx_work_orders_tenant_id ON public.work_orders USING btree (tenant_id)` |
| `idx_work_orders_tenant_status` | `CREATE INDEX idx_work_orders_tenant_status ON public.work_orders USING btree (tenant_id, status)` |
| `work_orders_code_key` | `CREATE UNIQUE INDEX work_orders_code_key ON public.work_orders USING btree (code)` |
| `work_orders_pkey` | `CREATE UNIQUE INDEX work_orders_pkey ON public.work_orders USING btree (id)` |

### 外键

| 约束名 | 引用 |
|--------|------|
| `work_orders_approved_by_fkey` | `approved_by -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `work_orders_asset_id_fkey` | `asset_id -> assets(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `work_orders_assignee_id_fkey` | `assignee_id -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `work_orders_location_id_fkey` | `location_id -> locations(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `work_orders_requestor_id_fkey` | `requestor_id -> users(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |
| `work_orders_tenant_id_fkey` | `tenant_id -> tenants(id) ON DELETE NO ACTION ON UPDATE NO ACTION` |

### CHECK 约束

| 约束名 | 定义 |
|--------|------|
| `chk_work_orders_status` | `CHECK (((status)::text = ANY ((ARRAY['draft'::character varying, 'submitted'::character varying, 'approved'::character varying, 'rejected'::character varying, 'in_progress'::character varying, 'completed'::character varying, 'verified'::character varying])::text[])))` |

---

