# CMDB Platform 权限管理完整分析报告

> 日期: 2026-04-10
> 范围: 数据库 → 后端中间件 → API Handler → 前端 UI 全链路

---

## 一、数据模型

### 核心表结构

```
tenants          → 租户隔离
  ├── users      → 用户（username, password_hash, status, source）
  ├── roles      → 角色（permissions JSONB, is_system BOOLEAN）
  ├── user_roles → 用户-角色关联（user_id + role_id 联合主键）
  ├── departments → 部门（permissions JSONB — 未启用）
  └── user_sessions → 登录会话（IP, user_agent, device_type）
```

### 权限 JSON 结构

```json
{
  "resource_name": ["action1", "action2"],
  "assets": ["read", "write", "delete"],
  "topology": ["read"],
  "*": ["*"]  // 超级管理员通配符
}
```

**资源名 → URL 路径映射** (rbac.go `resourceMap`):
| 资源名 | 匹配路径 |
|--------|---------|
| assets | /assets, /discovery, /upgrade-rules |
| topology | /locations, /racks, /topology |
| maintenance | /maintenance |
| monitoring | /monitoring, /energy, /sensors |
| inventory | /inventory |
| audit | /audit, /activity-feed |
| dashboard | /dashboard |
| identity | /users, /roles, /auth |
| prediction | /prediction |
| integration | /integration |
| system | /system, /quality, /bia |

**动作映射**:
| HTTP 方法 | RBAC 动作 |
|-----------|----------|
| GET | read |
| POST / PUT / PATCH | write |
| DELETE | delete |

---

## 二、默认角色与权限

### 种子数据 (seed.sql)

| 角色 | 类型 | 权限 | 分配用户 |
|------|------|------|---------|
| **super-admin** | 系统级 (`is_system=true`) | `{"*": ["*"]}` 全部权限 | admin |
| **ops-admin** | 租户级 | assets(rwd), maintenance(rw), monitoring(rw), topology(r), inventory(rw), audit(r), dashboard(r), prediction(r), system(r) | sarah.jenkins |
| **viewer** | 租户级 | assets(r), topology(r), maintenance(r), monitoring(r), inventory(r), audit(r), dashboard(r) | mike.chen |

默认密码: `admin123`（bcrypt 哈希）

---

## 三、权限校验流程

```
HTTP 请求 (Bearer Token)
    │
    ▼
┌─────────────────────────────────────────────┐
│  Auth Middleware (auth.go)                   │
│  ├─ 跳过: /auth/login, /auth/refresh,       │
│  │        /healthz, /metrics                 │
│  ├─ 解析 JWT (HMAC-SHA256)                   │
│  ├─ 验证签名 + 过期时间 (15 分钟)              │
│  └─ 设置 context: user_id, tenant_id         │
└─────────────────┬───────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────┐
│  RBAC Middleware (rbac.go)                   │
│  ├─ 提取 URL → resource (via resourceMap)    │
│  ├─ HTTP method → action (read/write/delete) │
│  ├─ 加载用户权限:                              │
│  │   ├─ 1. Redis 缓存 (perms:{user_id}, 5min)│
│  │   └─ 2. 回退: DB 查 user_roles → 合并权限   │
│  ├─ 权限检查:                                  │
│  │   ├─ perms["*"]["*"] → 超管放行             │
│  │   ├─ perms[resource][action] → 直接匹配     │
│  │   ├─ write 隐含 read → 有 write 可读       │
│  │   └─ 未知资源路径 → 默认拒绝 (v1.4.0 修复)  │
│  └─ 403 Forbidden 或放行                      │
└─────────────────┬───────────────────────────┘
                  │
                  ▼
            Route Handler
```

---

## 四、Token 体系

| Token | 有效期 | 存储 | 用途 |
|-------|--------|------|------|
| Access Token | 15 分钟 | sessionStorage (前端) | API 认证 |
| Refresh Token | 7 天 | Redis (后端) + sessionStorage (前端) | 刷新 Access Token |

**刷新流程**:
1. Access Token 过期 → API 返回 401
2. 前端 apiClient 自动用 Refresh Token 调用 `/auth/refresh`
3. 后端验证 Refresh Token（Redis）→ 签发新 Access + Refresh
4. 旧 Refresh Token 从 Redis 删除（轮换机制）

---

## 五、前端权限 UI 分析

### A. 角色权限管理 (RolesPermissions.tsx)

| 功能 | 状态 | 说明 |
|------|------|------|
| 查看所有角色 | ✅ 可用 | 左侧角色列表面板 |
| 创建自定义角色 | ✅ 可用 | "Add New Role" → 弹窗表单 (名称 + 描述) |
| 编辑角色权限 | ✅ 可用 | 权限矩阵 toggle (view/edit/delete/export × 5 个作用域) |
| 保存权限变更 | ✅ 可用 | "Save Changes" → PUT /roles/{id} |
| 删除自定义角色 | ✅ 可用 | 系统角色受 `is_system` 保护 |
| Emergency Stop | ❌ Coming Soon | 无实现 |
| Deploy Changes | ❌ Coming Soon | 无实现（暗示审批工作流未实现） |

### B. 用户管理 (SystemSettings.tsx)

| 功能 | 状态 | 说明 |
|------|------|------|
| 查看用户列表 | ✅ 可用 | 分页表格 |
| 创建用户 | ✅ 可用 | 弹窗表单 (username, display_name, email, password) |
| 编辑用户 | ❌ Coming Soon | 后端 API 已实现，前端 UI 禁用 |
| 删除用户 | ❌ Coming Soon | 后端无 DELETE 端点，前端无实现 |
| 分配角色给用户 | ❌ 未实现 | DB 有 AssignRole 查询但从未调用 |
| 移除用户角色 | ❌ 未实现 | 无端点、无 UI |
| 查看密码修改 | ⚠️ 仅后端 | API 存在但 SystemSettings 无入口 |
| 查看用户会话 | ⚠️ 仅后端 | API 存在但无 UI |

### C. 前端权限检查

| 检查点 | 状态 | 说明 |
|--------|------|------|
| AuthGuard (路由保护) | ✅ | 未登录 → 重定向 /login |
| usePermission hook | ✅ 存在 | 但**没有任何页面使用它** |
| 按钮/菜单权限门控 | ❌ | 所有 UI 元素对所有用户可见 |
| 页面级权限门控 | ❌ | viewer 可看到 Admin 页面所有按钮 |

**后果**: viewer 用户可以看到"创建角色"按钮，点击后提交表单，API 返回 403。用户体验差。

---

## 六、严重问题清单

### P0 — 功能缺失

| # | 问题 | 影响 |
|---|------|------|
| 1 | **无法给用户分配角色** | DB 有 `user_roles` 表和 `AssignRole` 查询，但无 API 端点、无 Service 方法、无 UI。新创建的用户没有任何角色，无法访问任何资源 |
| 2 | **无法删除用户** | 无 DELETE /users/{id} 端点，无后端实现 |
| 3 | **前端无权限门控** | `usePermission` hook 存在但 0 个组件使用。所有页面/按钮对所有登录用户可见 |

### P1 — 逻辑错误

| # | 问题 | 影响 |
|---|------|------|
| 4 | **UI 权限动作名不匹配** | 前端: view/edit/delete/export → 后端: read/write/delete。"view"≠"read"，"edit"≠"write"，保存的权限 JSON 与后端校验不一致 |
| 5 | **UI 权限作用域不完整** | 前端仅 5 个作用域 (asset, stock, monitor, config, compliance) → 后端有 11 个资源。缺少: inventory, prediction, integration, dashboard, audit, identity |
| 6 | **Department 权限未启用** | `departments.permissions` 列存在但从未使用，`tenant.go` 中间件是空操作 |

### P2 — 体验问题

| # | 问题 | 影响 |
|---|------|------|
| 7 | 编辑用户 Coming Soon | 后端 API 可用但前端禁用 |
| 8 | Emergency Stop / Deploy Changes | 两个按钮无实现 |
| 9 | 密码修改无 UI 入口 | 后端有 ChangePassword，SystemSettings 无表单 |
| 10 | 用户会话无 UI | 后端记录会话，无管理界面 |
| 11 | 2FA / LDAP 未实现 | UI 有切换开关但是装饰性的 |

---

## 七、完整 API 端点状态

| 端点 | 后端 | 前端 | 状态 |
|------|------|------|------|
| POST /auth/login | ✅ | ✅ | 完整 |
| POST /auth/refresh | ✅ | ✅ | 完整 |
| GET /auth/me | ✅ | ✅ | 完整 |
| POST /auth/change-password | ✅ | ✅ (UserProfile) | 可用 |
| GET /users | ✅ | ✅ | 完整 |
| POST /users | ✅ | ✅ | 完整 |
| GET /users/{id} | ✅ | ✅ | 完整 |
| PUT /users/{id} | ✅ | ❌ Coming Soon | 后端可用 |
| DELETE /users/{id} | ❌ | ❌ | **未实现** |
| GET /users/{id}/sessions | ✅ | ❌ 无 UI | 后端可用 |
| POST /users/{id}/roles | ❌ | ❌ | **未实现** |
| DELETE /users/{id}/roles/{roleId} | ❌ | ❌ | **未实现** |
| GET /roles | ✅ | ✅ | 完整 |
| POST /roles | ✅ | ✅ | 完整 |
| PUT /roles/{id} | ✅ | ✅ | 完整 |
| DELETE /roles/{id} | ✅ | ✅ | 完整 (系统角色保护) |

---

## 八、修复建议路线图

### 第 1 批: 核心功能补齐

1. **角色分配端点**
   - 后端: POST /users/{id}/roles + DELETE /users/{id}/roles/{roleId}
   - Service: `AssignRole()`, `RemoveRole()`
   - 前端: SystemSettings 用户行添加角色下拉选择

2. **用户删除**
   - 后端: DELETE /users/{id} (软删除建议)
   - 前端: 启用删除按钮 + 确认对话框

3. **启用编辑用户**
   - 前端: 将 "Coming Soon" 替换为编辑表单（复用 CreateUser Modal）

### 第 2 批: 权限对齐

4. **修复前端权限动作名**
   - view → read, edit → write，与后端一致

5. **扩展权限矩阵作用域**
   - 添加缺失的 6 个资源: inventory, prediction, integration, dashboard, audit, identity

6. **启用前端权限门控**
   - 在关键页面使用 `usePermission()` 隐藏/禁用无权操作

### 第 3 批: 增强功能

7. Emergency Stop / Deploy Changes 实现或移除
8. 用户会话管理 UI
9. Department 级权限继承
10. LDAP/AD 集成
