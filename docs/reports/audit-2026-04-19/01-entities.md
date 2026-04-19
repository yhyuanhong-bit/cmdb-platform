# 01 — 核心实体域: Asset / Inventory / Location / Discovery / Topology

## 模块总览

这五个模块构成 CMDB 的"实体骨架"。`Topology` 管理 Location→Rack→RackSlot 的物理层级（ltree 树 + U 位槽位），是其它所有实体的空间坐标系。`Asset` 是 CMDB 的核心 CI 实体，通过 `location_id / rack_id` 挂在拓扑上。`Inventory` 是对 Asset 集合的"定期盘点任务"——生成 items、扫描匹配、标记差异。`Discovery` 是"自动发现暂存区"——网络扫描 / SNMP / IPMI 流入的候选资产，经人工 approve 后转成正式 Asset。`LocationDetect` 是 Discovery 的特殊子域：它不新增 CI，而是通过 SNMP MAC 表比对"实际位置 vs CMDB 记录的位置"，产出差异、自动纠正、告警。

调用链主方向：`Discovery` / `LocationDetect` → 产生候选或差异 → 流入 `Asset` → 被 `Topology` 空间约束 → 被 `Inventory` 周期性验证。所有模块都向 `assets / locations / racks / inventory_tasks` 等表写 `sync_version`，供边缘同步子系统消费。

---

## 1. Asset

### 职责
CI 主表的 CRUD、按 tenant/type/status/location/rack/serial/search 组合查询、分页。承载 BMC、保修、生命周期等扩展字段；对外发 `asset.created/updated/deleted` 事件；写 audit；软删除；sync_version 自增。

### 数据模型 (主要表 + 字段 + 关系)
- `assets` (迁移 `000004_assets_and_racks.up.sql:17-46` + 增量 `000035` BMC 字段 + `000036` warranty/lifecycle + `000017` ip_address)
  - 主键 `id`，租户 `tenant_id → tenants`
  - 自然键 `asset_tag UNIQUE`（全局，非 per-tenant），辅键 `property_number` / `control_number` / `serial_number`
  - 空间 FK: `location_id → locations`，`rack_id → racks`
  - 分类: `type / sub_type / status / bia_level`
  - 扩展: `attributes JSONB`（GIN 索引）、`tags TEXT[]`（GIN 索引）
  - BMC: `bmc_ip / bmc_type / bmc_firmware`
  - 保修/生命周期: `purchase_date / purchase_cost / warranty_start / warranty_end / warranty_vendor / warranty_contract / expected_lifespan_months / eol_date`
  - 网络: `ip_address`
  - 元字段: `deleted_at`（软删除）、`sync_version`（边缘同步）、`updated_at`（触发器）
- `asset_dependencies` (后述 Topology)
- `asset_field_authorities`（被 `impl_assets.go:246-276` 查询，未在本次扫描的迁移列表中；用于来源优先级）

### 运行逻辑 (请求/事件流向)
- Handler 层 `cmdb-core/internal/api/impl_assets.go`
  - `CreateAsset` (L50): 绑定 → 质量闸门 `qualitySvc.ValidateForCreation` (L94-116) → `assetSvc.Create` → audit → 发 `SubjectAssetCreated` → CIType 软校验返回 warnings
  - `UpdateAsset` (L161): 字段级"权威来源"保护，从 `asset_field_authorities` 查 priority > 50 的字段并清空对应 params (L240-276) → sqlc `UpdateAsset` → 单独一条 `UPDATE ... ip_address` 绕过 sqlc (L290-299) → 若 `bia_level=='critical'` 自动建 `change_audit` 工单 (L330-342)
  - `DeleteAsset` (L366): 软删除（sqlc 实际是 `DELETE`，见下风险）
- Service 层 `cmdb-core/internal/domain/asset/service.go`
  - `List/Get/Create/Update/Delete/FindBySerialOrTag`；写操作后同步 `incrementSyncVersion` (L140-147)
- 生成的 SQL `cmdb-core/db/queries/assets.sql`
  - `ListAssets` / `CountAssets` 都带 `deleted_at IS NULL`（软删除过滤生效）
  - 但 `DeleteAsset` (L80) 是 `DELETE FROM assets WHERE id=$1 AND tenant_id=$2`——**硬删除**，与模型里的 `deleted_at` 字段不一致

### 业务规则
1. **质量闸门**：创建前必须通过 `qualitySvc.ValidateForCreation`，失败返回 HTTP 422 + `QUALITY_GATE_FAILED`。(`impl_assets.go:94-116`)
2. **字段权威来源**：当某字段（serial_number/vendor/model）在 `asset_field_authorities` 中有 priority>50 的登记记录时，API 来源（priority=50）的写入会被静默丢弃并返回 warnings。(`impl_assets.go:240-276`)
3. **Critical CI 变更审计**：`bia_level='critical'` 的资产被更新时自动创建 `change_audit` 工单。(`impl_assets.go:330-342`)
4. **asset_tag 全局唯一**：`UNIQUE` 约束在列定义上，无 `tenant_id` 前缀（迁移 `000004:20`）——跨租户冲突。
5. **sync_version 自增**：Create/Update/Delete 都在 pool 里 `UPDATE assets SET sync_version = sync_version + 1`（`service.go:140-147`）。
6. **serial/tag 查找非租户首查**：`FindAssetBySerialOrTag` 虽然带 `tenant_id = $1`，但 `(serial_number = $2 OR asset_tag = $3)` 无 `LIMIT 1` 稳定排序——存在多命中歧义。(`db/queries/assets.sql:83-87`)

### 外部依赖
- DB: `assets`, `asset_field_authorities`, `work_orders`（通过 maintenance 服务）
- NATS subjects publish: `asset.created`, `asset.updated`, `asset.deleted`, `asset.location_changed`
- 被调: `maintenance.Service` (Critical 变更时)、`quality.Service`（创建质量闸）、`audit` 服务
- 调用方: `impl_assets.go`、`impl_inventory.go` (FindBySerialOrTag during import)、`impl_discovery.go` (通过 `FindAssetByIP`)、workflows (auto work order 间接读 assets)

### 观察到的优点
- `assetService` 接口（`asset_service.go:14-21`）只暴露 handler 真正需要的 6 个方法，易于 mock。
- sqlc COALESCE+narg 的 UPDATE 模式保证 partial update 语义清晰。
- sync_version 集中在 Service 层一处，新增字段时不会漏同步。
- `ListAssets` 已带 `deleted_at IS NULL` 过滤。

### 观察到的缺口/风险
- **硬删除 vs 软删除不一致**：`Asset` 表有 `deleted_at` 列，所有 List/Count/Find 查询都带 `deleted_at IS NULL`，但 `DeleteAsset`（`db/queries/assets.sql:80-81`）执行真正的 `DELETE`。外键 `rack_slots.asset_id`（000037 改为 `ON DELETE SET NULL`）、`inventory_items.asset_id`、`mac_address_cache.asset_id`、`discovered_assets.matched_asset_id` 都会被级联清空——历史审计断链。
- **ip_address 绕过 sqlc**：`impl_assets.go:290-299` 手写 `UPDATE assets SET ip_address=...`，不走 Service 层，因此 **不触发 sync_version 自增**，也不触发 `updated_at` 以外的任何事件 / 审计。
- **asset_tag UNIQUE 无 tenant 维度**：迁移 `000004:20` 写的是 `asset_tag VARCHAR(100) NOT NULL UNIQUE`。多租户环境下两租户不能有同名 tag。
- **字段权威保护每次 Update 都查一次 DB**：`impl_assets.go:245-252` 每次 Update 都执行 `SELECT ... FROM asset_field_authorities GROUP BY field_name`。未缓存，未走 prepared statement。
- **搜索用 ILIKE 且无 trigram 索引**：`db/queries/assets.sql:10` `name ILIKE '%'||$search||'%'` 双端通配符，现有 GIN 索引都在 attributes/tags 上，未覆盖 name/asset_tag 的模糊搜索。
- **Create 事务不原子**：`assetSvc.Create` 成功后，audit + event publish 是分开执行——若 DB commit 后进程崩溃，事件和审计都会丢。`ip_address` 分两步写同理。
- **domain 包零测试**：`asset/service.go` 没有任何 `*_test.go`。Critical 逻辑（质量闸门、权威保护、Critical 变更工单）只在 `impl_assets_test.go` 有测试。

---

## 2. Inventory

### 职责
周期性盘点任务：创建任务 → 导入预期清单（expected）→ 现场扫描 / 录入 actual → 系统比对标记 scanned/pending/discrepancy/missing → 生成 per-rack / discrepancy 汇总 → 支持 resolve 动作闭环。含 scan history、notes 子资源。

### 数据模型
- `inventory_tasks` (`000007_inventory.up.sql:1-15` + `000029` 加 `deleted_at`, `sync_version`)
  - `code UNIQUE`、`status ∈ {planned,in_progress,completed}`、`method`、`planned_date`、`completed_date`、`scope_location_id`、`assigned_to`
- `inventory_items` (`000007:17-27` + `000031` 加 `sync_version`)
  - `task_id → inventory_tasks ON DELETE CASCADE`
  - `asset_id`（可空——missing 时为空）、`rack_id`
  - `expected JSONB` / `actual JSONB`
  - `status ∈ {pending,scanned,discrepancy,missing}`、`scanned_at`、`scanned_by`
- `inventory_scan_history` (`models.go:244-252`): 每次扫描一行，含 method / result / note
- `inventory_notes` (`models.go:235-242`): severity + text

### 运行逻辑
- Handler 分三个文件：
  - `impl_inventory.go`：任务 CRUD、items 列表、`ScanInventoryItem`、`ImportInventoryItems`（批量导入+匹配）
  - `inventory_endpoints.go`：`GetInventoryRacksSummary`、`GetInventoryDiscrepancies`
  - `inventory_item_endpoints.go` + `inventory_resolve_endpoint.go`：scan history / notes / resolve
- `CreateInventoryTask` (`impl_inventory.go:67-106`): code 自动生成 `INV-YYYY-NNNN`
- `ImportInventoryItems` (`impl_inventory.go:211-325`):
  - 在一个 tx 内循环：先 `FindBySerialOrTag`，fallback `property_number`，再 fallback `control_number`；命中则 insert 为 `pending`，未命中为 `missing`
  - 结尾 `UPDATE inventory_tasks SET status='in_progress' WHERE status='planned'`
- `ScanInventoryItem` (`impl_inventory.go:172-206`): Service 写 item + 发 `inventory.item_updated` + 在 handler 层再 `UPDATE inventory_tasks SET status='in_progress' WHERE status='planned'`（Auto-activate）
- `ResolveInventoryDiscrepancy` (`inventory_resolve_endpoint.go:13-83`): action ∈ {verify,clear,add_findings,register} → 更新 item status + 写 note + 写 scan_history + Auto-activate task

### 业务规则
1. **状态机约束**：只有 `planned`/`in_progress` 可 Update，只有 `planned` 可 Delete（`service.go:127-129`、L156-160、sqlc `UpdateInventoryTask` WHERE `status != 'completed' AND deleted_at IS NULL`）。
2. **任务 auto-activate**：首次 Scan 或 Import 或 Resolve 会把 `planned → in_progress`（三处代码路径，见 `impl_inventory.go:198-200, 310-312`、`inventory_resolve_endpoint.go:73-75`）。
3. **Complete 不保护**：`CompleteInventoryTask` 直接 UPDATE 无条件，`db/queries/inventory_tasks.sql:32-35` 未检查状态——可把已 completed 再 complete（无害但违反状态机）。
4. **Item 匹配三级 fallback**：serial/asset_tag → property_number → control_number（`impl_inventory.go:242-272`），匹配不到则 status=`missing`。
5. **Discrepancy 解析**：`resolve.action='verify'|'clear'|'register'` → status=`scanned`；`add_findings` → `discrepancy`（`inventory_resolve_endpoint.go:27-37`）。

### 外部依赖
- DB: `inventory_tasks`, `inventory_items`, `inventory_scan_history`, `inventory_notes`, `assets`, `racks`, `locations`, `users`
- NATS publish: `inventory.item_updated`（Service 层）；`inventory.task_completed` 在 workflow 订阅里消费但本模块未找到发布点
- 被调: `asset.Service.FindBySerialOrTag`、`asset.Service.GetByID`（fallback 查询后二次读取）
- 调用方: 仅 API handler 直接调用；workflows 监听 `SubjectInventoryTaskCompleted` 做通知

### 观察到的优点
- `Import` 使用单个 tx 保证"item 批量+task 激活"原子性（`impl_inventory.go:224-317`）。
- `UpdateInventoryTask` 和 `SoftDeleteInventoryTask` 在 sqlc 层用 WHERE 约束状态机，不靠应用层检查。
- `inventory_items(task_id, sync_version)` 有组合索引（000031）。

### 观察到的缺口/风险
- **`SubjectInventoryTaskCompleted` 从未发布**：`CompleteInventoryTask` handler 只写 audit，无 `publishEvent`（`impl_inventory.go:110-120`），但 workflows `subscriber.go:41` 订阅了它 → 通知永远不会触发。
- **`inventory.task_completed` 订阅空转**：`onInventoryCompletedNotify` 在 workflows 里注册但没人 fire。
- **Auto-activate 重复分散**：3 个 handler 各自 `UPDATE inventory_tasks SET status='in_progress' WHERE status='planned'`；应下沉到 Service 的 `ActivateIfPlanned`（sqlc 已有 `ActivateInventoryTask` 但未被调用，见 `inventory_tasks.sql:68-71`）。
- **Import 用 `tx.Exec` 忽略 error**：`impl_inventory.go:296-306` 两处 `tx.Exec(...)` 不检查返回值，部分失败会被吞。
- **`CompleteInventoryTask` 不检查 task 归属租户**：sqlc `CompleteInventoryTask WHERE id=$1`（无 tenant_id），任何人拿到 task_id 就能 complete。
- **Import 不走 sqlc**：原始 INSERT 字符串（`impl_inventory.go:297、305`）造成 `inventory_items.sync_version` 不变（sqlc 生成的 INSERT 会默认 0，但手写 INSERT 字段顺序漂移风险）。
- **rack_summary / discrepancies 原生 SQL 在 handler 里**：两个自定义查询（`inventory_endpoints.go:43-57、112-122`）混在 handler，应下沉到 Service / sqlc。
- **item 查询未带 tenant 隔离**：`ListInventoryItems`（`inventory_tasks.sql:18-22`）仅 `WHERE task_id=$1`；依赖上游 `GetInventoryTask` 作 tenant 过滤——`ListInventoryItems` handler (`impl_inventory.go:54-63`) 没做二次校验。

---

## 3. Location Detect

### 职责
**不是 CRUD 域**。是个基于 SNMP 周期采集的"物理位置校验器"。把交换机 MAC 表（从 ingestion-engine 通过 NATS 进来）→ 映射到 rack → 和 CMDB 的 `assets.rack_id` 比对 → 产出 4 种 diff（consistent / relocated / missing / new_device）→ 有工单就自动确认变更，无工单就告警；再做异常模式检测（频繁迁移 / 非工时迁移 / 机柜集中消失）。

### 数据模型
- `mac_address_cache` (`000032_location_detect.up.sql:32-47`)
  - 唯一键 `(tenant_id, mac_address)`
  - `switch_asset_id + port_name → switch_port_mapping → connected_rack_id`
  - `asset_id`（尝试反查 assets.attributes->>'mac_address'）、`detected_rack_id`
  - `first_seen / last_seen`
- `switch_port_mapping` (`000032:2-14`)：交换机端口→rack/U 位的静态表
- `asset_location_history` (`000032:17-29`)：每次变更留痕，`detected_by ∈ {snmp_auto, manual, qr_scan, ...}`，含 `work_order_id`
- 读写 `assets.rack_id`、`work_orders`（用于判断是否"授权迁移"）、`alert_events`（产出告警）

### 运行逻辑
1. ingestion-engine 发 `mac_table.updated` NATS 事件
2. `cmd/server/main.go:240-278` 订阅并 `UpdateMACCache`（service.go:163-196）
3. 完成后 `go func() { locationDetectSvc.RunDetection(...) }()`（main.go:272-274）
4. `RunDetection` (`detector.go:34-95`)：
   - `CompareLocations` → 列出 diffs
   - 每个 diff 按 diff_type 处理：
     - `relocated` + 有工单 → `autoConfirmRelocation` → UPDATE assets.rack_id + history + 发 `asset.location_changed` + 关闭匹配的 `relocation` 工单 (`detector.go:97-153`)
     - `relocated` 无工单 → 告警 warning
     - `missing` → 告警 warning
     - `new_device` → 告警 info + insert 到 `discovered_assets` 供 Discovery 审批 (`detector.go:68-76`)
   - 最后 `DetectAnomalies` → 三类异常写告警
- API endpoints (`location_detect_endpoints.go`)：diffs / summary / anomalies / report 全部读取聚合

### 业务规则
1. **"授权迁移"判定**：`work_orders.type='relocation' AND status NOT IN ('completed','verified','rejected')` 存在则 `has_work_order=true`（`service.go:70-76`）。
2. **自动关闭工单**：`autoConfirmRelocation` 关闭所有 `type='relocation'` 且非终态的工单，写入 `work_order_logs` 动作 `auto_completed_by_location_detect`（`detector.go:127-146`）。
3. **频繁迁移异常**：30 天内同一 asset 迁移≥3 次 → warning（`anomaly.go:44-77`）。
4. **非工时异常**：`EXTRACT(HOUR)` ∈ `[22,24) ∪ [0,6)` 且 24h 内的迁移 → warning（`anomaly.go:80-113`）。
5. **集中消失异常**：同一 rack 1h 内 ≥3 个设备离开 → critical（`anomaly.go:116-150`）。
6. **new_device 自动登记到 Discovery**：`ON CONFLICT DO NOTHING`（`detector.go:69-75`）——注意这里的 ON CONFLICT 缺失显式约束，依赖于 `000034` 的 `(tenant_id, source, external_id)` 唯一索引，`external_id` 被手动拼接成 `MAC-<mac>`。

### 外部依赖
- DB: `mac_address_cache`, `switch_port_mapping`, `asset_location_history`, `assets`, `racks`, `work_orders`, `work_order_logs`, `alert_events`, `discovered_assets`
- NATS subscribe: `mac_table.updated`
- NATS publish: `asset.location_changed`, `alert.fired`
- 被调: `qr_endpoints.go:104-105`（扫码后 `RecordLocationChange`）
- 调用方: main.go 订阅器；API（只读端点）

### 观察到的优点
- MAC 表比对后**立即触发**一次 RunDetection，不必等 ticker（`main.go:271-274`）。
- "授权 vs 未授权迁移"把 ITSM 工单和网络扫描绑在一起，非常合理。
- 异常检测三种模式覆盖面合理（频繁/非工时/集中消失）。
- 自动关闭工单时写 `work_order_logs`，保留审计痕迹。

### 观察到的缺口/风险
- **`StartPeriodicDetection` 死代码**：`detector.go:15-29` 声明了 5 分钟 ticker，但全仓库搜不到调用点（`grep StartPeriodicDetection` 只匹配定义自身）。意味着只有 `mac_table.updated` 到来时才触发比对，MAC 表一直没更新就永远不做周期扫描。
- **`createLocationAlert` 对 new_device 丢 asset_id**：`detector.go:155-162` 的 `INSERT INTO alert_events (asset_id, ...)` 会写入空的 asset_id（`uuid.Nil`）——`alert_events.asset_id` 若非空约束会报错；若可空则大量 NULL asset_id 告警。
- **手拼 JSON 字符串**：`detector.go:74` 用 `fmt.Sprintf(`{"mac_address":"%s",...}`)` 拼 JSON，如果 MAC 或 rack_name 含特殊字符会炸（应该 `json.Marshal`）。
- **`anomaly.go:88` 的 EXTRACT HOUR**：未指定时区，服务器所在 TZ 与租户业务时区可能不一致——导致"非工时"定义漂移。
- **异常告警不带 asset_id**：`detector.go:90-94` 调 `createLocationAlert(..., LocationDiff{DiffType: string(a.Type)})` 丢掉了 `Anomaly.AssetID` 和 `Anomaly.RackID`——告警中 asset_id 永远 nil。
- **rows.Scan 错误被吞**：`service.go:102-104、142-144`、`detector.go:136-138` 多处 `rows.Scan(...) == nil` 或 continue，单行解析失败全部静默跳过。
- **每租户无并发保护**：RunDetection 可被 ticker + mac_table.updated 同时触发（`main.go:272` 又是 `go func()`），两个 goroutine 在同一 tenant 上并发修改 `assets.rack_id`——存在最后写入者赢。
- **SQL 在 service 层直接 `s.pool.Exec`**：所有 location_detect 查询都是原生 SQL，未经 sqlc，schema 变更无编译期检查。
- **`asset_location_history.tenant_id` 无 FK**：迁移 `000032:19` `tenant_id UUID NOT NULL`，没有 `REFERENCES tenants(id)` ——孤立数据风险。
- **`mac_address_cache.tenant_id` 同上**（`000032:34`）。

---

## 4. Discovery

### 职责
"自动发现暂存区"：外部扫描（integration adapter、SNMP、IPMI、location_detect 的 new_device）把候选设备写入 `discovered_assets`，人工审批 approve / ignore。命中已有 asset 的 IP 则标记为 `conflict`。

### 数据模型
- `discovered_assets` (`000016:1-17` + `000034` 唯一索引)
  - `source`（自由字符串），`external_id`（外部系统 ID），`hostname`, `ip_address`
  - `raw_data JSONB`
  - `status ∈ {pending, conflict, approved, ignored}`
  - `matched_asset_id → assets`
  - `diff_details JSONB`（未在当前流程写入，为未来预留）
  - `reviewed_by / reviewed_at`
  - 唯一键 `(tenant_id, source, external_id)`（000034）
- 间接依赖：`credentials` (`000017:10-20`)、`scan_targets` (`000017:23-34`)——但这些表的 CRUD 不在 `discovery/service.go`，归属 integration / adapter 域

### 运行逻辑
- `impl_discovery.go`
  - `IngestDiscoveredAsset` (L33-81): bind → 如带 IP，先 `FindAssetByIP` 命中则置 `matched_asset_id` 并把 status 改成 `conflict`（L64-73）→ insert
  - `ApproveDiscoveredAsset` / `IgnoreDiscoveredAsset`：仅改 status + reviewer
  - `GetDiscoveryStats`：过去 24h 的计数聚合
- Service (`discovery/service.go`) 是极薄包装：`List/Ingest/Approve/Ignore/GetStats`，外加 `Queries()` 逃逸方法（L21-23）让 handler 直接调底层查询（IP 匹配）

### 业务规则
1. **IP 冲突检测**：Ingest 时若 `assets.ip_address = $2 OR assets.bmc_ip = $2` 命中（`db/queries/discovery.sql:41-42`），status=`conflict`。
2. **状态机**：`pending → approved` 或 `pending → ignored`（两条 UPDATE 无状态前置校验，可重复 approve）。
3. **24h 统计窗口**：`GetDiscoveryStats` 硬编码 `discovered_at > now() - interval '24 hours'`（`db/queries/discovery.sql:39`）——超过 24h 的数据统计里看不到。
4. **唯一性兜底**：(tenant_id, source, external_id) 唯一（000034:10-11），但 `external_id` 可以为 NULL——NULL 在 PG 中不参与 UNIQUE 冲突，允许无 external_id 的记录重复插入。

### 外部依赖
- DB: `discovered_assets`, `assets` (IP 匹配)
- NATS: 本模块不发不订（**缺口**——approve 后应该有 "asset upgraded from discovered" 事件）
- 被调: 无（纯数据管道）
- 调用方:
  - `location_detect/detector.go:69-76`（SNMP new_device 自动登记）
  - 外部 collector（通过 HTTP API 直接 POST）

### 观察到的优点
- Service 层很薄，易读。
- 冲突检测 `ip_address OR bmc_ip`（`discovery.sql:42`）同时覆盖 BMC 管理口和业务口，合理。
- sqlc 完整覆盖此模块。

### 观察到的缺口/风险
- **Approve 不创建实体资产**：`ApproveDiscoveredAsset` (`service.go:51-57`) 只改 status，**没有**从 `discovered_assets` 生成 `assets` 行。也就是说"审批"目前没有任何实际后果——纯标记。关键业务逻辑缺失。
- **没有从 Discovered→Asset 的 promotion 流程**：`raw_data` 含 hostname/IP/source，但 approve 后的后续动作在代码里找不到（没有对应事件订阅者）。
- **IP 冲突处理半吊子**：standalone `Ingest` 命中时 status=`conflict`，但 `matched_asset_id` 之后的 diff_details 始终不填——操作员无法知道"哪些字段不一样"。
- **Approve/Ignore 无状态校验**：`db/queries/discovery.sql:21-27` 无 `WHERE status='pending'`，已 approve 的可重复 approve，reviewed_at 会被覆盖。
- **`Service.Queries()` 反模式**：`discovery/service.go:21-23` 暴露底层 `*dbgen.Queries`，破坏了服务边界——handler 直接 `s.discoverySvc.Queries().FindAssetByIP(...)`（`impl_discovery.go:65-68`），使得 mock 困难。
- **source 字段无枚举**：数据库 `VARCHAR(50)`，代码里看到的值五花八门（`snmp_mac_detect`, `integration`, 外部 id…），无常量文件定义合法值。
- **无清理策略**：`discovered_assets` 只增不减，长期会涨；没有 retention。
- **GetStats 用 FILTER 在 approved/ignored 上**：24h 窗口导致审批历史记录超过 24h 就不再出现在 dashboard，但"pending"状态的也被同样窗口截断——**未处理的旧 pending 会从统计里消失**（`discovery.sql:29-39`）。

---

## 5. Topology

### 职责
物理拓扑树：Location（ltree，任意深度）→ Rack（每 Location N 个）→ RackSlot（每 Rack 42 U + side）。提供层级查询、祖先/后代遍历、rack 占用计算、location 统计（资产、机柜、告警、平均占用率）、资产依赖图（asset_dependencies）+ 拓扑可视化图数据。

### 数据模型
- `locations` (`000003_locations.up.sql:1-21` + `000033` 加经纬度 + `000029` 加 `deleted_at`/`sync_version`)
  - `path LTREE`（GIST 索引）、`parent_id` 自引用
  - 唯一 `(tenant_id, slug)` 在 `000003:20` 建立
  - `level ∈ {country, region, city, site, ...}`（自由字符串）
- `racks` (`000004:1-15` + `000037` 加 `updated_at` + `000029` 加 deleted_at/sync_version + `000034` 加 `(tenant_id, location_id, name) WHERE deleted_at IS NULL` 唯一索引)
- `rack_slots` (`000004:48-57`)：`UNIQUE(rack_id, start_u, side)`，`CHECK (end_u >= start_u)`；`000037:1-4` 把 asset_id FK 改成 `ON DELETE SET NULL`
- `asset_dependencies` (`topology_endpoints.go:31-39` 引用；迁移位于其它文件，含 source/target/type/description)

### 运行逻辑
- `topology/service.go` 大部分是 sqlc 包装器；两处内联 SQL：
  - `GetLocationStats` (L62-118) 对平均占用率用 `s.pool.QueryRow`（sqlc 写不下这个聚合）
  - `DeleteRackSlot` / `CreateRackSlot` 的 `assets.rack_id` 同步（Fix #20，L294-297、L321-332）
- Handler 层 `impl_locations.go` + `topology_endpoints.go` + `location_stats_endpoints.go`
  - `CreateLocation` 手工拼 ltree path：root 用 slug，子节点拼 `parent.path + "." + slug`（`impl_locations.go:160-170`）
  - `DeleteLocation` 支持 `?preflight=true` 先返回依赖计数，`?recursive=true` 才允许级联（`impl_locations.go:284-318`、`service.go:198-229`）
  - `GetTopologyGraph` (L153-350)：大 SQL 查询 location 下所有资产（`path <@`）+ asset_dependencies + metrics 批量拉取，返回 nodes/edges

### 业务规则
1. **ltree path 全应用层维护**：`impl_locations.go:160-170` 在 handler 里拼 path 字符串，没有 DB 触发器；**更新 parent_id 不会自动重建 path**（sqlc 的 `UpdateLocation` 根本不更新 path——`db/queries/locations.sql:40-51`）。
2. **删除保护**：非 recursive 时，若 `child_locations + racks + assets > 0` 返回 409 + `HAS_DEPENDENCIES`（`service.go:209-212`）。
3. **槽位冲突检测**：`CheckSlotConflict` (`rack_slots.sql:19-23`) `NOT (end_u < $start_u OR start_u > $end_u)`——区间重叠。
4. **RackSlot 双写 assets.rack_id**：创建 slot 时自动把 asset 的 rack_id 同步（Fix #20, `service.go:285-299`），删除时若无其它 slot 引用则清空（L301-334）。
5. **统计范围包含后代**：`CountAssetsUnderLocation`, `CountRacksUnderLocation`, `GetRackOccupanciesByLocation` 全部 `path <@ $loc.path`（`db/queries/assets.sql:89-94`、`racks.sql:60-66、50-58`）。
6. **rack 唯一性**：`(tenant_id, location_id, name) WHERE deleted_at IS NULL`（000034:2-3）——同一 location 下 rack 名唯一，软删除不占名字。

### 外部依赖
- DB: `locations`, `racks`, `rack_slots`, `asset_dependencies`, `assets`, `alert_events`, `metrics`
- NATS publish: `location.created/updated/deleted`
- 被调: `asset` (经 `rack_id` / `location_id`)、`location_detect`、`inventory` (scope_location_id)、`discovery`（无直接，但 approve 后应挂位置）
- 调用方: 所有需要空间定位的模块

### 观察到的优点
- ltree 路径 + GIST 索引：任意深度的祖先/后代查询 O(log n)。
- RackSlot ↔ Asset 双向同步是正确的（之前可能是缺口，现在 Fix #20 合上了）。
- Delete preflight + recursive 设计对用户友好。
- `GetRackOccupanciesByLocation` 做了批量去 N+1（`impl_locations.go:107-113` 显式注释 "Fix #3"）。

### 观察到的缺口/风险
- **Update parent/slug 不重建 path**：`db/queries/locations.sql:40-51` 的 `UpdateLocation` 只改 name/name_en/slug/level/status/metadata/sort_order，**不更新 path**，也不更新子孙的 path。一旦把 site 改 slug，整棵子树 path 停在旧值——父子 `path <@` 查询会错。
- **parent_id 不在 Update 范围**：同上，无法通过 UpdateLocation 移动子树；只能 Delete+Recreate。
- **latitude/longitude 通过 metadata 绕路**：`impl_locations.go:184-197` 从 `req.Metadata["latitude"]` 转写到 `params.Latitude`，但 OpenAPI Create 请求体没有显式经纬度字段——这是 undocumented API 契约。
- **`GetLocationStats` 内联 SQL 算平均占用率**：`service.go:96-110` 12 行手写 SQL，未用 sqlc 也未加缓存；大租户下 dashboard 每刷新一次执行一遍。
- **`GetTopologyGraph` 单查询返回全部 asset+dep+metrics**：`topology_endpoints.go:162-321` 一次查 200 assets、然后 `ANY($1)` 查依赖、再 `ANY($1)` 查外部节点、再 `ANY($1)` 查 metrics——4 次 round-trip，metrics 聚合在 handler 里做。Location 很大时（>200 assets）截断返回，前端拿不到完整图。
- **raw SQL 在 handler**：`topology_endpoints.go:25-39、90-93、118-120、162-188、229-233、262-269、292-299` 全部手写 SQL。schema drift 零保护。
- **asset_dependencies 无租户过滤（在 delete 时）**：`DeleteAssetDependency`（L118-120）`DELETE FROM asset_dependencies WHERE id=$1`——**没有 tenant_id 校验**。跨租户删除风险。
- **asset_dependencies 无 type 枚举约束**：`dependency_type` 自由字符串，应用层 default `depends_on`（L86-87）。
- **DeleteLocation 实际是硬删除**：`db/queries/locations.sql:53-54` `DELETE FROM locations ...`，而 schema 里有 `deleted_at`（000029）——与 Asset 一样的"有字段但硬删除"问题。
- **`DeleteDescendantLocations` 同上**（`locations.sql:68-70`）——级联删除也是硬删。
- **sync_version 自增不在事务内**：Create/Update/Delete 后独立 `UPDATE ... SET sync_version=sync_version+1`，若两步之间失败就不一致；相比直接把 sync_version 放进 sqlc 的 UPDATE 字段内更稳。
- **`ListRootLocations` 不过滤 deleted_at**：`db/queries/locations.sql:1-4` 未带 `deleted_at IS NULL`（而 `ListAllLocations` 带了）。软删除的根 location 仍返回。
- **`rack_slots` 表无 `tenant_id`**：(`000004:48-57`) 依赖 `racks.tenant_id` 间接追溯；`DeleteRackSlot` 写了 `JOIN racks` 做 tenant 校验（`rack_slots.sql:14-17`），但 `ListRackSlots`（`rack_slots.sql:1-7`）没有——已知 rack_id 就能读任何 slot。

---

## 跨模块交互

```
                       NATS (mac_table.updated)
                                |
                                v
                       +--------+-----------+
                       |  LocationDetect    |
                       |  Service           |
                       +--------+-----------+
                                |
               +----------------+---------------+
               |                |               |
               v                v               v
       discovered_assets  alert_events    assets.rack_id
         (Discovery 域)   (Monitoring)    (UPDATE +
               |                             history)
               v
       +-------+--------+      audit + event
       |  Discovery     |<-----------------+
       |  Service       | approve (当前空实现)
       +-------+--------+
               |  (缺失: 生成 Asset)
               v
       +-------+--------+         event publish
       |  Asset Service |--- asset.created/updated/deleted -->
       +-------+--------+                                   |
               | rack_id / location_id                      |
               v                                            v
       +-------+--------+                            workflows
       |  Topology      |                            订阅器
       |  Service       |<-- location.*/ asset.*     (auto WO)
       +-------+--------+
               | scope_location_id
               v
       +-------+--------+  FindBySerialOrTag -> Asset
       |  Inventory     |  scan item
       |  Service       |
       +----------------+
```

关键耦合点:
1. **LocationDetect → Discovery**：new_device 自动 insert `discovered_assets`（`detector.go:69-76`），但两边对接点只用源码字符串 `'snmp_mac_detect'` 约定，无常量。
2. **LocationDetect → Asset**：`autoConfirmRelocation` 直接 `UPDATE assets` 不走 Asset Service（`detector.go:99-104`）——绕过 sync_version 自增吗？不，SQL 里显式带了 `sync_version=sync_version+1`；但**绕过了 Service 层的 audit / event**。
3. **LocationDetect → Maintenance**：直接 UPDATE `work_orders` 关闭单据（`detector.go:138-140`），绕过 maintenance.Service 状态机。风险：maintenance 里定义的状态转换约束（如必须经过 `in_progress`）被绕过。
4. **Asset → Maintenance**：Critical 变更自动建工单（`impl_assets.go:330-342`），这是反向耦合——Asset 主动调 maintenance。
5. **Inventory → Asset**：仅读，通过 `FindBySerialOrTag` + `GetByID`，单向。
6. **Topology ← Asset/Rack**：被动提供空间坐标；RackSlot 的创建会反向同步 `assets.rack_id`（service.go:285-299）。
7. **Discovery → Asset**：目前**只读**（IP 匹配），approve 后无写入路径。
8. **QR 扫描 → LocationDetect**：`qr_endpoints.go:104-105` 扫码确认位置后调 `RecordLocationChange`，绕过 Asset Service 直接写 `assets.rack_id` + history。

---

## 整体评估

**域模型主干合理，但职责边界有渗漏。** Location/Rack/Asset 的 ltree + 槽位设计是标准 DCIM 做法，Inventory 扫描对账 + Discovery 暂存区也都是业界通用模式，LocationDetect 用 SNMP MAC 表做"物理一致性"是亮点。主要结构性问题集中在四处：

1. **Discovery Approve 没闭环**：批准后 `discovered_assets.status='approved'` 仅仅是个标记，没有任何代码把它转成 `assets` 行。这是整个"自动发现"叙事的关键漏洞——等于审批按钮按下去什么也不会发生。
2. **软删除/硬删除混乱**：`assets`、`inventory_tasks`、`locations`、`racks` 都有 `deleted_at` 列，查询端也都 `WHERE deleted_at IS NULL`，但 **`DeleteAsset` / `DeleteLocation` / `DeleteDescendantLocations` 的 sqlc 是硬 DELETE**。只有 `DeleteRack` 和 `SoftDeleteInventoryTask` 真正软删。审计链可能断裂。
3. **LocationDetect 绕过 Service 层**：`autoConfirmRelocation` 直写 `assets` 表、关闭 `work_orders`——把三个域的变更耦合到一个函数里，没有通过事件/服务抽象。这让未来给 Asset 加字段、改状态机、加 validation 很危险（例如 BIA=critical 的 asset 被 SNMP 挪位置不会触发 change_audit 工单，而 API 更新会）。应该重构为：LocationDetect 发事件 → AssetService 订阅 → 走标准 Update 流程。
4. **ltree path 不可维护**：没有触发器，没有 Update 支持 path 重建，parent_id 甚至不能改。结果是租户建错层级就只能删重来。

次一级问题：周期性任务未启动（`StartPeriodicDetection` 死代码）、`SubjectInventoryTaskCompleted` 从未发布但有订阅者（事件死信）、`asset_tag` 全局唯一与多租户矛盾、`GetTopologyGraph` 200 条硬截断且 SQL 全在 handler、`asset_dependencies` 删除未做 tenant 隔离、`location_detect` 全部原生 SQL 无 sqlc。

测试覆盖方面，这五个域里只有 `asset` 有 handler 层 `impl_assets_test.go`；Service 层无任何单测，location_detect 的 4 类 diff + 3 类异常检测、inventory 的状态机、discovery 的 IP 冲突检测——全部依赖人工回归。考虑到这些是 CMDB 的心脏逻辑，覆盖缺口优先级应该高于许多其它 refactoring。
