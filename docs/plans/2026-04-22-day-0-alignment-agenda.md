# Day 0 对齐会议议程 — CMDB 业务修复 Wave 启动

> **时间**：90 分钟，硬性时间盒
> **目的**：让 A/B/C 三件事可以从下周一开始安全地并行执行
> **不是**：技术细节讨论会、需求澄清会、kickoff 庆祝会
> **是**：决策会 — 每个议题结束时必须落下答案，写入决策日志

---

## 与会者（必到）

| 角色 | 谁 | 为什么必须到 |
|---|---|---|
| 决策人 / Tech Lead | _填_ | 最终拍板权 |
| Product / 业务方 | _填_ | C-1 (Service 边界) 没你定不下来 |
| BE Lead | _填_ | 估资源、cap effort |
| FE Lead | _填_ | A-1 (删 phantom UI) 直接影响 |
| 待命 | _填_ | DBA 或 SRE 顾问 — 备询 |

**缺席任何"必到"角色 → 推迟会议**。不要在缺人时下决策然后回头被推翻。

---

## 会前准备（与会者必读，3 分钟）

每位与会者会前 24 小时 **读**：
- `docs/reviews/2026-04-22-business-fit-review.md` (4500 字，10 分钟)
- `docs/plans/2026-04-22-business-remediation-roadmap.md` 的 M1 小节（1500 字，5 分钟）

**没读的人不参加投票**。这是节省现场时间的硬规则。

---

## 议程结构

每个议题：5 分钟陈述 + 10 分钟讨论 + 5 分钟决策。决策结果当场记录在 `docs/decisions/2026-04-22-day-0.md`（会议主持人现场敲）。

| # | 议题 | 时间 | 决策必须出的产物 |
|---|---|---|---|
| 0 | Check-in + 议程确认 | 5min | 议程是否还有缺项 |
| 1 | Service 实体边界 | 20min | A / B / C 选一 |
| 2 | README 准确化范围 | 15min | 改 N 处文字 + 是否承诺 M4 |
| 3 | OpenAPI gate 严格度 | 10min | block PR / warn / advisory |
| 4 | C 的 spec 作者 + spec 模板 | 10min | 指名 + 模板路径 |
| 5 | 资源分配 + 谁做什么 | 15min | 每人下周 5 天的 commitment |
| 6 | 完成判据 + 下次同步 | 10min | 周五 standup 议程 |
| 7 | Buffer / overflow | 5min | — |

---

## 议题 1 — Service 实体的业务边界（20 分钟）

**为什么这是头号议题**：C 的所有 schema、API、UI 都从这个定义往下推。错了 3 周白做。

### 问题陈述

「Service」一词在 ITIL / ServiceNow / 客户日常语言里至少有三种含义。我们要选一种，并写下不选另外两种的代价。

### 选项

| 选项 | 定义 | 例子 | 适合谁 |
|---|---|---|---|
| **A. Business Service**（业务功能） | 用户可见、对外承诺 SLA 的功能单元 | 「在线下单」「支付」「客户登入」 | 给业务、CIO、合规审计看的视角 |
| **B. Technical Service**（技术组件） | IT 团队维护的技术能力 | 「订单 API」「Postgres 主集群」「Kafka 集群」 | 给 SRE / 开发看的视角 |
| **C. Application Service**（应用维度） | 一个可部署的应用 + 它的所有实例 | 「order-service v1.4 在 prod-cluster 跑 30 个 pod」 | 给 DevOps / SRE 看的视角 |

### 决策含义

| 选 | 含义 | 例子 |
|---|---|---|
| A | services 表关联到 BIA tier，service ↔ assets N:M | 「在线下单」由 5 台机 + 3 个 service 支撑 |
| B | services 之间也有依赖（service-to-service map） | 「订单 API depends on Postgres 主集群」 |
| C | 引入 deployment / instance 概念，跟 K8s 模型贴 | 跟既有 asset 模型有重叠风险 |

**推荐**：**A**（Business Service）。理由：
- 评审报告里 #5 service-centric 告警聚合需要 BIA tier 驱动 — 这是 A 的语义
- 客户问「订单系统挂了影响哪些客户」时问的是 A 不是 B
- B/C 可以后续作为 sub-entity 加上去（service.children）— A 反过来加不上

### 决策

- [ ] 选 A
- [ ] 选 B
- [ ] 选 C
- [ ] 选 A + 后续允许加 sub_type 区分 B/C

**还要回答的子问题（每个 1 句话）**：
- service 是否跨租户？（**默认否** — 多租户 SaaS 模型）
- service 能否互相依赖？（推荐**能**，N:M `service_dependencies` 表，留给 M2）
- 一个 asset 能否属于多个 service？（**能** — service_assets 是 N:M）
- service 状态字段含义？（建议：active / deprecated / decommissioned）

---

## 议题 2 — README 准确化范围（15 分钟）

### 问题陈述

README 当前对外承诺的能力 vs 评审揭示的真实状态有 **3 处明显漂移**：

| README 措辞 | 真实情况 | 损害 |
|---|---|---|
| "**Offline-capable** Edge nodes with NATS leafnode federation" | SyncGate 在 edge 未同步时返回 503 拒所有写。无写队列。 | 客户合同期望失真 |
| "**Auto-recovery after 14 days offline**" | 实际是 NATS JetStream MaxAge=14d，过了会丢 envelope。不是真的"recovery" | 同上 |
| "**Predictive AI with RCA**" | prediction_models 表是 global（无 tenant_id），handler 抽象到 ML service 但没看到实际模型 | 客户以为有 AI |

### 决策

**Q1: 改 README 还是改产品？**

- [ ] 改 README — 收回承诺。立刻可做，承认现状。
- [ ] 改产品 — 把 M4 (Edge 离线) 提前。Edge 离线 4 周成本。
- [ ] 都做 — 先改 README 不丢面子，同时把 M4 列入 commitment

**Q2: 修改后的措辞是什么？**

候选措辞（投票选一个）：
- 候选 1: ~~"Offline-capable"~~ → "Resilient to brief central outages with auto-resume sync"
- 候选 2: ~~"Offline-capable"~~ → "Edge nodes with bidirectional sync and central failover"
- 候选 3: 改产品 → 不改 README

**Q3: Predictive AI 措辞**

- [ ] 删除 "Predictive AI with RCA" — 还没真做
- [ ] 改成 "Asset failure prediction (beta)" — 软化
- [ ] 不改 — 接受指责

---

## 议题 3 — OpenAPI gate 严格度（10 分钟）

### 问题陈述

Foundation Wave (B) 要把 OpenAPI 提升为真相源。CI 检查发现 spec 和 impl 不一致时怎么办？

### 选项

| 选项 | 行为 | 副作用 |
|---|---|---|
| **Block** | 不一致 → CI fail → PR 不能 merge | 阻止漂移；初期会让 PR 速度下降一周 |
| **Warn** | 不一致 → CI 显示 warning，但 merge 不阻塞 | 妥协，但漂移持续累积 |
| **Advisory** | 不一致 → 只在 nightly job 报告 | 实际等于没做 |

### 决策含义

- 选 Block：要先把现有 spec 漂移修干净（评审里 `docs/OPENAPI_DRIFT.md` 列了 N 处），否则所有 PR 立刻挂掉
- 选 Warn：B 的工作是表面功夫，6 个月后还是漂移
- 选 Advisory：浪费 B 这个礼拜

### 决策

- [ ] Block，但有 1 周 grace period 修现有漂移
- [ ] Warn，3 个月后 review 是否升级到 Block
- [ ] Advisory（**不推荐**）

---

## 议题 4 — Spec 作者 + 模板（10 分钟）

### 问题陈述

Foundation Wave 要立 `db/specs/` 流程。第一个用例是 C 的 service 实体 spec。**谁写第一个 spec 决定模板长什么样**。

### 决策

**Q1: spec 作者**

- [ ] BE Lead 写 — 最熟全局，但贵
- [ ] BE-2 写 + Tech Lead review — 培养
- [ ] Product 写业务部分 + BE-2 写技术部分 — 推荐

**Q2: spec 模板长什么样**

候选模板（投票选一个）：
- 候选 1: 短模板（5 段：背景 / 实体 ER / 边界问题 / 迁移策略 / 测试计划）— 推荐
- 候选 2: ADR 风格（Decision / Context / Consequences）
- 候选 3: Design Doc 风格（10+ 段，类 Google）

**推荐**：候选 1。理由：足以承载关键决策，又不至于变成永远写不完的产物。

**Q3: spec 必须 review 的人数**

- [ ] 1 个 reviewer 够（BE Lead）
- [ ] 2 个（BE Lead + Product）— 推荐
- [ ] 全员 review — 太重

---

## 议题 5 — 资源分配 + 周一开始的 commitment（15 分钟）

### 团队规模假设

**先回答**：本周可全职投入的人数？

- 后端: ___ 人
- 前端: ___ 人
- Product: ___ 人

### 分配建议（按团队规模）

#### 如果 BE >= 3：并行模式

| 人 | 周一开始做什么 | 周五交付 |
|---|---|---|
| BE-1 | Foundation Wave (B) 全部 | OpenAPI gate 上 CI |
| BE-2 + Product | C 的 spec | spec 第一稿 review |
| BE-3 | A 的 5 个 quick win + alert rule 脏数据修 | 全做完 |
| FE-1 | A-1 删 phantom UI + 准备 C 的前端骨架 | 11 个占位下线 |

#### 如果 BE = 2：减并行

| 人 | 周一开始做什么 |
|---|---|
| BE-1 | Foundation (B) + 周中开始 C 的 spec |
| BE-2 | A 的 quick wins + alert rule 脏数据修，周中起协助 C |
| FE-1 | A-1 删 phantom UI |

#### 如果 BE = 1：串行模式（不并行）

| 周 | 做什么 |
|---|---|
| W1 | 全部 quick wins (A) — 4 天 + 1 天 buffer |
| W2 | Foundation (B) |
| W3 | C 的 spec 撰写 + review |
| W4-5 | C 实现 |

### 决策

- [ ] 走 3-BE 并行模式
- [ ] 走 2-BE 减并行模式
- [ ] 走 1-BE 串行模式
- [ ] 其他配置：______

---

## 议题 6 — 完成判据 + 周五同步（10 分钟）

### Wave 1 周完成的判据

每条都要 yes 才算 wave 启动成功：

- [ ] **A 完成**：11 个 phantom UI 下线、README 改完、ROADMAP 文件存在、5 条 alert rule 脏数据清掉
- [ ] **B 完成**：OpenAPI gate 在 CI 上跑 + block/warn 模式确认 + db/specs/README.md 存在
- [ ] **C 完成**：service 实体 spec 第一稿 + 1 轮 review + sign-off
- [ ] 决策日志 `docs/decisions/2026-04-22-day-0.md` 含本会全部 7 项决策

### 周五 30 分钟同步会

议程模板（每周固定）：
1. 上周 commitment 完成度（红/黄/绿，每人 30 秒）
2. blocker（每个 blocker 有 owner 和下次更新时间）
3. 决策需要 escalate 的问题（如有）
4. 下周 commitment

**不**做：技术 deep dive、设计讨论 — 那些另开会

---

## 议题 7 — Overflow / Buffer（5 分钟）

留 5 分钟处理：
- 议程外的紧急议题（限 1 个，否则下次开会）
- 与会者补充关注点
- 行动项分配确认

---

## 决策日志模板

会议主持人现场（不是会后）写入 `docs/decisions/2026-04-22-day-0.md`：

```markdown
# Day 0 对齐会决策日志 — 2026-04-22

## 与会者
- _名字_ (Tech Lead)
- _名字_ (Product)
- ...

## 缺席
- _名字_（原因）— 决策不为其保留意见

## 决策

### D1: Service 实体边界
- 选项: A (Business Service)
- 投票: X-Y-Z
- 反对意见 + 谁记录: _名字_
- 后续可推翻条件: 第一个客户面谈反馈说错了

### D2: README 改写
...

### D3: OpenAPI gate
...

### D4: Spec 作者
- _名字_（业务部分）+ _名字_（技术部分）
- 模板: 候选 1，路径 db/specs/_template.md

### D5: 资源
- 模式: 3-BE 并行
- 人员: BE-1=_名字_, BE-2=_名字_, ...

### D6: 周完成判据 + 周五同步
- 判据: 上述 4 条
- 同步会: 每周五 16:00, 30 分钟

## 行动项

| # | 行动 | Owner | Due |
|---|---|---|---|
| 1 | 起草 db/specs/_template.md | _名字_ | 周二 EOD |
| 2 | 起草 service 实体 spec | _名字_+_名字_ | 周四 EOD |
| 3 | 改 README 3 处措辞 | _名字_ | 周一 EOD |
| 4 | 删 11 个 phantom UI | _名字_ | 周三 EOD |
| 5 | 在 CI 加 OpenAPI gate (warn 模式 / block 模式) | _名字_ | 周四 EOD |
| 6 | 修 alert rule 脏数据 | _名字_ | 周二 EOD |
| ... |

## 下次会议
- 周五 16:00, 30 分钟 wave 周同步
```

---

## 主持人注意事项

1. **时间盒严格**：每个议题到点不结束 → 当场决定（a）pick 推荐选项（b）记入 parking lot 限时 24h 内 async 解决
2. **没读会前材料的人不投票** — 这是规则，第一次违反明确警告
3. **争议无法收敛 → Tech Lead 拍板** — 不要为 unanimous 浪费时间
4. **决策不出门** — 当场写决策日志，会后追加无效（避免「我以为我们说的是...」）
5. **不要进入"怎么做"** — 议题是"做什么 + 谁做"，"怎么做"是后续 spec 阶段的事

---

## 准备工作 checklist

会前 24 小时主持人确认：

- [ ] 会议邀请发出，明示「90 分钟硬上限」
- [ ] 与会者收到会前材料链接
- [ ] 投票工具准备好（Slack reaction / 现场口头都行）
- [ ] 决策日志文件预创建空白模板
- [ ] 会议室 / Zoom 链接 OK
- [ ] 确认所有"必到"角色 RSVP yes
