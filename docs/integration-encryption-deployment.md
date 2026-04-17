# 集成模块加密升级 — 部署 Checklist

> 适用于从 master `9a91826` 之前的版本升级到 `2a11996` 或更新的版本。
> 本次升级引入:
> - 强制环境变量 `CMDB_SECRET_KEY`(AES-256 at-rest 加密)
> - 两个数据库迁移 000038 / 000039
> - Adapter config / Webhook secret dual-write 加密
> - Adapter 失败状态持久化 + 指数退避

---

## 0. 升级前必读

### 0.1 为什么要做这些操作

| 变更 | 如果不做会怎样 |
|------|----------------|
| 设置 `CMDB_SECRET_KEY` | 服务启动会 `Fatal` 并立即退出 |
| 执行 migration 000038 | 启动时 `expected_version=38` 检查会 `Fatal`,服务拒绝启动 |
| 执行 migration 000039 | 同上 |
| 备份数据库 | 回滚失败时无法恢复集成配置数据 |

### 0.2 升级期间的行为说明

- **Dual-write 模式**:新代码同时写明文 + 密文。这是为了让首次上线安全 —— 即使密钥配错,读路径会自动回落到明文,不会丢数据。
- **一次性数据迁移(可选)**:老数据只有明文。如果需要把历史数据也加密,参见第 7 节。
- **密钥丢失**:如果 `CMDB_SECRET_KEY` 丢失或变更,之前加密的行无法解密;dual-write 期间读路径会回落到明文列(仍可读)。**dual-write 被移除之后密钥丢失等于数据丢失**。

### 0.3 前置条件

- Docker 24+ 和 Docker Compose v2(如果是 compose 部署)
- 可登录的 Postgres 账号,具备 ALTER TABLE 权限
- `openssl`(生成密钥用)或能跑 Go 的环境
- 能在 CI/CD 或秘钥管理系统里新增一个变量
- 最近 24 小时内的数据库备份

---

## 1. 生成 `CMDB_SECRET_KEY`

**方式 A(推荐,一行命令):**
```bash
openssl rand -hex 32
```

输出示例(**不要使用下面这个值,请自己生成**):
```
a1b2c3d4e5f60718293a4b5c6d7e8f9a0b1c2d3e4f5a6b7c8d9e0f1a2b3c4d5e
```

**方式 B(用 Go):**
```bash
cd cmdb-core
go run -tags=keygen ./cmd/keygen 2>/dev/null || \
  go run -exec 'go run' -gcflags= - <<'EOF'
package main
import (
  "fmt"
  "github.com/cmdb-platform/cmdb-core/internal/platform/crypto"
)
func main() { fmt.Println(crypto.GenerateKeyHex()) }
EOF
```

**校验要求:**
- 长度**必须**恰好 64 个字符
- 只能包含 `0-9` 和 `a-f`(小写)
- 不能复用 `JWT_SECRET`、`DB_PASS` 或任何其他密钥

**存储规范:**
- ❌ 不要写进 git
- ❌ 不要在 Slack / 邮件 / issue 里粘贴
- ✅ 存到密钥管理系统:AWS Secrets Manager / HashiCorp Vault / Kubernetes Secret / GCP Secret Manager / 阿里云 KMS
- ✅ 至少 2 个有权限的人知道它在哪里(避免 bus-factor)

---

## 2. 把密钥注入到运行环境

### 2.1 Docker Compose(单机 / 开发)

```bash
cd cmdb-core/deploy

# 如果 .env 还不存在,先从模板复制
[ -f .env ] || cp .env.example .env

# 编辑 .env,找到 CMDB_SECRET_KEY 这一行,替换右侧占位值
# 假设你刚生成了 KEY=...
export KEY=$(openssl rand -hex 32)
sed -i.bak "s|^CMDB_SECRET_KEY=.*|CMDB_SECRET_KEY=${KEY}|" .env
unset KEY
rm -f .env.bak

# 确认文件权限仅 owner 可读
chmod 600 .env
ls -l .env
# 应显示: -rw-------
```

`docker-compose.yml` 已经用 `${CMDB_SECRET_KEY:?...}` 强制要求该变量非空,如果 `.env` 没配好 `docker compose up` 会在 YAML 解析阶段直接拒绝。

### 2.2 Kubernetes

**创建 Secret:**
```bash
kubectl create secret generic cmdb-secret-key \
  --namespace cmdb \
  --from-literal=CMDB_SECRET_KEY=$(openssl rand -hex 32)
```

**在 Deployment 里引用:**
```yaml
# deployment.yaml 片段
spec:
  template:
    spec:
      containers:
        - name: cmdb-core
          env:
            - name: CMDB_SECRET_KEY
              valueFrom:
                secretKeyRef:
                  name: cmdb-secret-key
                  key: CMDB_SECRET_KEY
```

**校验 Secret 存在:**
```bash
kubectl -n cmdb get secret cmdb-secret-key -o jsonpath='{.data.CMDB_SECRET_KEY}' | base64 -d | wc -c
# 应输出: 64
```

### 2.3 Systemd

```ini
# /etc/systemd/system/cmdb-core.service
[Service]
EnvironmentFile=/etc/cmdb/secrets.env
# 不要用 Environment=CMDB_SECRET_KEY=... 明文写在 unit 文件
```

```bash
# /etc/cmdb/secrets.env
CMDB_SECRET_KEY=<你生成的 64 位 hex>
```

```bash
sudo chown root:cmdb /etc/cmdb/secrets.env
sudo chmod 640 /etc/cmdb/secrets.env
sudo systemctl daemon-reload
```

### 2.4 云厂商托管(AWS ECS / GCP Cloud Run / 阿里云 ASK)

以 AWS ECS 为例:

```bash
# 1. 存到 Secrets Manager
aws secretsmanager create-secret \
  --name cmdb/secret-key \
  --secret-string "$(openssl rand -hex 32)"

# 2. 在 Task Definition 里引用(片段)
{
  "containerDefinitions": [{
    "secrets": [{
      "name": "CMDB_SECRET_KEY",
      "valueFrom": "arn:aws:secretsmanager:region:account:secret:cmdb/secret-key"
    }]
  }]
}
```

---

## 3. 数据库备份(强制)

**Docker Compose 环境:**
```bash
# 设定备份目录
BACKUP_DIR=/var/backups/cmdb/$(date +%Y%m%d-%H%M%S)
mkdir -p "${BACKUP_DIR}"

# 执行备份
docker compose -f cmdb-core/deploy/docker-compose.yml exec -T postgres \
  pg_dump -U cmdb -Fc cmdb > "${BACKUP_DIR}/cmdb.dump"

# 校验备份不为 0 字节
ls -lh "${BACKUP_DIR}/cmdb.dump"
# 至少应该有几 MB
```

**独立 Postgres:**
```bash
PGHOST=<主机> PGUSER=cmdb PGPASSWORD=<密码> \
  pg_dump -Fc cmdb > "${BACKUP_DIR}/cmdb.dump"
```

**校验备份可还原(强烈推荐):**
```bash
# 临时建一个 staging DB,试着恢复,看有无报错
createdb -h localhost cmdb_restore_test
pg_restore -d cmdb_restore_test "${BACKUP_DIR}/cmdb.dump" 2>&1 | tail -10
dropdb cmdb_restore_test
```

---

## 4. 执行数据库迁移

### 4.1 自动执行(推荐)

`cmd/server/main.go` 在启动时会自动扫描 `migrations/` 并应用未执行过的 `.up.sql`。**你只需要启服务**,迁移会被自动跑。

如果你用的是 docker-compose,启动时 `MIGRATIONS_DIR` 默认指向容器内 `/app/migrations`,容器镜像已经打包好迁移文件。

### 4.2 手动执行(零停机升级场景)

如果你想在升级服务**之前**就把迁移跑掉(避免"旧代码 + 新 schema"和"新代码 + 旧 schema"的短暂窗口):

```bash
cd cmdb-core

# 连到目标 DB
export DATABASE_URL=postgres://cmdb:<密码>@<主机>:5432/cmdb?sslmode=require

# 查看当前版本
psql "${DATABASE_URL}" -c "SELECT MAX(version) FROM schema_migrations;"
# 期望: 37(升级前)

# 执行 000038
psql "${DATABASE_URL}" -f db/migrations/000038_encrypt_integration_secrets.up.sql
psql "${DATABASE_URL}" -c "INSERT INTO schema_migrations (version, dirty) VALUES (38, false);"

# 执行 000039
psql "${DATABASE_URL}" -f db/migrations/000039_adapter_failure_state.up.sql
psql "${DATABASE_URL}" -c "INSERT INTO schema_migrations (version, dirty) VALUES (39, false);"

# 再次确认
psql "${DATABASE_URL}" -c "SELECT MAX(version) FROM schema_migrations;"
# 期望: 39
```

### 4.3 校验迁移结果

```bash
# integration_adapters 新增了 5 个列
psql "${DATABASE_URL}" -c "\d integration_adapters" | grep -E "config_encrypted|consecutive_failures|last_failure_at|last_failure_reason|next_attempt_at"
# 应输出 5 行

# webhook_subscriptions 新增了 1 个列
psql "${DATABASE_URL}" -c "\d webhook_subscriptions" | grep secret_encrypted
# 应输出 1 行
```

---

## 5. 部署新版本服务

### 5.1 Docker Compose

```bash
cd cmdb-core/deploy

# 拉取最新代码
cd ../..
git fetch origin
git checkout master
git pull

# 重建镜像(包含新二进制 + 新迁移文件)
cd cmdb-core/deploy
docker compose build cmdb-core

# 滚动重启(先起新的,再停旧的)
docker compose up -d --no-deps cmdb-core

# 观察启动日志
docker compose logs -f cmdb-core
```

**预期日志应看到:**
```
migration: applied  file=000038_encrypt_integration_secrets.up.sql version=38
migration: applied  file=000039_adapter_failure_state.up.sql version=39
Metrics puller started (5m interval)
```

**如果看到下列任一,立即停止并查第 8 节:**
- `failed to load at-rest encryption key (set CMDB_SECRET_KEY)`
- `database schema is behind code`
- `panic: invalid hex key length`

### 5.2 Kubernetes(滚动更新)

```bash
# 推送新镜像到你的 registry 后
kubectl -n cmdb set image deployment/cmdb-core cmdb-core=<registry>/cmdb-core:<新 tag>

# 观察 rollout 状态
kubectl -n cmdb rollout status deployment/cmdb-core --timeout=5m

# 拉日志
kubectl -n cmdb logs -l app=cmdb-core --tail=100 -f
```

### 5.3 健康检查

```bash
# Liveness
curl -fsS http://<主机>:8080/healthz
# 期望: {"status":"ok"}

# Readiness
curl -fsS http://<主机>:8080/readyz
# 期望: {"status":"ok"}

# 新功能冒烟测试 — 创建一个测试 webhook,confirm 加密字段被写
TOKEN=<你的 admin token>
curl -X POST http://<主机>:8080/api/v1/integration/webhooks \
  -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name":"smoke-test","url":"https://example.com/hook","events":["asset.created"],"secret":"test-secret-1234","enabled":false}'

# 然后在 DB 里确认 secret_encrypted 非 NULL
psql "${DATABASE_URL}" -c "SELECT name, secret IS NOT NULL AS has_plain, secret_encrypted IS NOT NULL AS has_enc FROM webhook_subscriptions WHERE name='smoke-test';"
# 期望输出: has_plain=t, has_enc=t

# 清理测试数据
psql "${DATABASE_URL}" -c "DELETE FROM webhook_subscriptions WHERE name='smoke-test';"
```

---

## 6. 回滚方案

### 6.1 触发条件

- `CMDB_SECRET_KEY` 配错,所有新建 adapter/webhook 失败
- 迁移后应用无法启动(错误日志包含 "schema ahead of code")
- 加密/解密路径出现数据损坏报错

### 6.2 回滚步骤

```bash
# 1. 切回旧镜像
docker compose -f cmdb-core/deploy/docker-compose.yml up -d --no-deps cmdb-core:<旧 tag>
# 或者:
kubectl -n cmdb rollout undo deployment/cmdb-core

# 2. 回退 schema(仅当确认旧代码跑不动新 schema 时执行)
psql "${DATABASE_URL}" -f cmdb-core/db/migrations/000039_adapter_failure_state.down.sql
psql "${DATABASE_URL}" -c "DELETE FROM schema_migrations WHERE version = 39;"

psql "${DATABASE_URL}" -f cmdb-core/db/migrations/000038_encrypt_integration_secrets.down.sql
psql "${DATABASE_URL}" -c "DELETE FROM schema_migrations WHERE version = 38;"

# 3. 如果数据已损坏,从备份恢复
docker compose -f cmdb-core/deploy/docker-compose.yml stop cmdb-core
psql "${DATABASE_URL}" -c "DROP DATABASE cmdb; CREATE DATABASE cmdb OWNER cmdb;"
pg_restore -d "${DATABASE_URL}" "${BACKUP_DIR}/cmdb.dump"
docker compose -f cmdb-core/deploy/docker-compose.yml start cmdb-core
```

**注意**:`000038.down.sql` 只 DROP 加密列,明文列保留。所以即便回滚,dual-write 期间写入的明文数据仍然可读。

---

## 7. 老数据回填加密(可选,第二阶段)

升级后新数据会 dual-write,但老数据(升级前已存在的行)的加密列是 NULL。
只要应用层读路径仍然兜底到明文(`DecryptConfigWithFallback` /
`DecryptSecretWithFallback`),这些老行照常工作。如果想让它们也加密并最终把明文列清空,走下面两步:

### 7.1 回填加密列

需要应用层回填——密钥只在应用进程里,数据库无法自己加密。推荐写一次性 admin 任务:

```go
// 伪代码:迭代 config 非空、config_encrypted 为空的行
rows, _ := pool.Query(ctx, `
    SELECT id, tenant_id, config
    FROM integration_adapters
    WHERE config IS NOT NULL
      AND config::text <> '{}'
      AND config_encrypted IS NULL
`)
for rows.Next() {
    var id, tenantID uuid.UUID
    var cfg []byte
    _ = rows.Scan(&id, &tenantID, &cfg)
    enc, err := cipher.Encrypt(cfg)
    if err != nil { /* log + skip */ continue }
    _, _ = pool.Exec(ctx, `
        UPDATE integration_adapters
        SET config_encrypted = $1
        WHERE id = $2 AND tenant_id = $3
    `, enc, id, tenantID)
}
// 对 webhook_subscriptions.secret / secret_encrypted 同样处理一次。
```

webhook 的 `secret` 是 TEXT(HMAC 共享密钥,原本就是 ASCII),回填时记得
`[]byte(secret)` 再 `cipher.Encrypt`。

> **不推荐** 用 `pgcrypto` 在数据库里加密——那会把密钥带到数据库层,打破"密钥只
> 存在应用侧"的前提,也让后续的 key rotation 变复杂。

### 7.2 清空明文列

回填完且观察期(建议 ≥ 1 个完整发布周期)无人报错后,运行仓库自带脚本把明文列清空。
脚本带事务 + pre-flight 守卫,只要任何非空明文行缺少对应密文就会 RAISE EXCEPTION 回滚。

```bash
# 强制要求:提前做一次 pg_dump 备份(§3 同款命令)
psql "$DATABASE_URL" -v ON_ERROR_STOP=1 \
  -f cmdb-core/db/scripts/cleanup/clear_integration_plaintext.sql
```

脚本最后会 `\echo` 清理的行数,并往 `audit_events` 写一条
`integration_plaintext_cleared` 记录。具体说明见
[`cmdb-core/db/scripts/cleanup/README.md`](../cmdb-core/db/scripts/cleanup/README.md)。

> 脚本**刻意**放在 `db/scripts/cleanup/` 而不是 `db/migrations/`,
> 因此 `main.go` 启动时的迁移扫描不会自动跑它。它需要人工触发。

---

## 8. 故障排查

### 8.1 启动报 `failed to load at-rest encryption key`

**原因**:`CMDB_SECRET_KEY` 没设,或者值不是 64 位 hex。

**检查**:
```bash
docker compose exec cmdb-core sh -c 'echo ${#CMDB_SECRET_KEY}'
# 应输出: 64
```

**修复**:按第 1 节重新生成并按第 2 节注入,然后重启服务。

### 8.2 启动报 `database schema is behind code, migrations_behind=2`

**原因**:代码期望 `expectedMigration=39`,但 DB 还在 37。

**修复**:
- 如果是 docker-compose 部署 → 查 migration 自动执行日志,通常是目录挂载错了,`MIGRATIONS_DIR` 找不到文件
- 如果是独立 DB → 按第 4.2 节手动执行

### 8.3 启动报 `database schema is ahead of code`(Warn,不致命)

**原因**:DB 已经跑到 39,但你起的是旧二进制(version=37)。

**处理**:
- 如果你在故意回滚旧版本,且旧版本能容忍多出的列(一般能,因为老代码不读新列)→ 忽略
- 如果是部署了错误的镜像 → 部署正确版本

### 8.4 创建 webhook 成功但投递失败,日志显示 `decrypt secret failed`

**原因**:`CMDB_SECRET_KEY` 在升级过程中被替换过。

**诊断**:
```sql
SELECT name, secret IS NOT NULL AS has_plain, secret_encrypted IS NOT NULL AS has_enc
FROM webhook_subscriptions;
```

- 如果 `has_plain=true`:代码会自动回落到明文,投递应当仍然成功。如果仍失败,看具体错误栈。
- 如果 `has_plain=false` 且 `has_enc=true`:只能重新录入该 webhook,或者从备份恢复 DB 再用老密钥。

### 8.5 Adapter 反复进 backoff 窗口,似乎不同步

**原因**:新逻辑按 `next_attempt_at` 过滤;adapter 需要过了 backoff 窗口才会被再次拉取。

**诊断**:
```sql
SELECT name, consecutive_failures, last_failure_at, next_attempt_at, last_failure_reason
FROM integration_adapters
WHERE enabled = true
ORDER BY consecutive_failures DESC;
```

**强制立即重试**:
```sql
UPDATE integration_adapters
SET consecutive_failures = 0, next_attempt_at = NULL
WHERE id = '<adapter id>';
```

---

## 9. 完成后动作(Post-Deploy)

- [ ] 在密钥管理系统里给 `CMDB_SECRET_KEY` 打标签(owner、创建时间、应用范围)
- [ ] 加监控告警:`CMDB_SECRET_KEY` 被访问时触发(审计日志)
- [ ] 更新运维 runbook,加上本文档链接
- [ ] 在内部 wiki 登记本次升级的日期、版本号、执行人
- [ ] 预定密钥轮换时间表(建议 12 个月,轮换方案参见未来的 `docs/key-rotation-runbook.md`)

---

## 10. 快速 Checklist(一页纸)

升级执行人从上到下逐项打勾:

- [ ] 阅读完本文档第 0 节
- [ ] 生成 `CMDB_SECRET_KEY`(`openssl rand -hex 32`,64 位 hex)
- [ ] 密钥已存入秘钥管理系统,**不在** git / 聊天工具 / 邮件里
- [ ] 至少 2 人知道密钥的存放位置
- [ ] 数据库已备份到 `${BACKUP_DIR}/cmdb.dump`
- [ ] 备份已通过 `pg_restore` 验证可还原
- [ ] 目标环境 `.env` 或 Secret 已注入 `CMDB_SECRET_KEY`
- [ ] 校验 `echo ${#CMDB_SECRET_KEY}` 输出 `64`
- [ ] 拉取代码到 `master` 最新(commit 不早于 `2a11996`)
- [ ] 服务已重启,日志看到 `migration: applied version=38` 和 `version=39`
- [ ] `/healthz` 和 `/readyz` 都返回 200
- [ ] 冒烟测试:创建测试 webhook 后 DB 里 `secret_encrypted IS NOT NULL`
- [ ] 冒烟测试清理完毕
- [ ] 回滚步骤已演练过(或至少已在 staging 验证过)
- [ ] 本次升级已登记到运维 wiki

---

**维护人**:集成模块 owner
**最后更新**:2026-04-18
**关联 PR / Commit**:master `2a11996`
