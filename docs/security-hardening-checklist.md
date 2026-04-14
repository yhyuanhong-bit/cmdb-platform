# Security Hardening Checklist

Use this checklist before promoting any CMDB Platform deployment to production.
Items marked **CRITICAL** will cause data exposure or unauthorized access if skipped.

---

## Authentication

- [ ] **CRITICAL** — Change `JWT_SECRET` to a 64+ character random string.
  Generate with: `openssl rand -hex 32`
  The application refuses to start in `cloud` mode if this is still set to the default.

- [ ] **CRITICAL** — Change the default admin account password immediately after first login.
  The seed data creates a default admin; this account must be secured before exposing the API.

- [ ] **CRITICAL** — Enable HTTPS-only access. Redirect all HTTP (port 80) to HTTPS (port 443).
  No credentials or tokens should ever travel over plain HTTP.

- [ ] Set `Secure` and `HttpOnly` flags on all session cookies.
  Verify in browser DevTools → Application → Cookies.

- [ ] Set `SameSite=Strict` on session cookies to prevent CSRF via cookie.

- [ ] Set a short JWT expiry (e.g., 15 minutes) and enable refresh token rotation if supported.

- [ ] Disable or remove any test accounts and development API keys before go-live.

---

## Database

- [ ] **CRITICAL** — Change `DB_PASS` from the default `changeme` to a strong, unique password.
  The application refuses to start in `cloud` mode if `DATABASE_URL` contains `changeme`.

- [ ] Restrict PostgreSQL `listen_addresses` to `localhost` or the internal Docker network.
  The bundled compose exposes port 5432 — remove this port binding in production:
  ```yaml
  # In docker-compose.override.yml
  services:
    postgres:
      ports: []   # Remove public binding; use internal Docker DNS instead
  ```

- [ ] Enable SSL for database connections. Update `DATABASE_URL` to use `sslmode=require`:
  ```
  DATABASE_URL=postgres://cmdb:PASSWORD@postgres:5432/cmdb?sslmode=require
  ```

- [ ] Create a dedicated database user with least-privilege access (not the `postgres` superuser).
  The default `cmdb` user should own only the `cmdb` database.

- [ ] Verify no database port is reachable from outside the host firewall:
  ```bash
  nmap -p 5432 your-server-external-ip
  # Expected: filtered or closed
  ```

- [ ] Enable PostgreSQL audit logging (`log_connections`, `log_disconnections`, `log_statement=ddl`)
  to detect unauthorized schema changes.

---

## Redis

- [ ] Restrict Redis `bind` address to the internal Docker network. Remove the public port binding:
  ```yaml
  # In docker-compose.override.yml
  services:
    redis:
      ports: []   # Internal only
  ```

- [ ] Enable Redis `requirepass` authentication for production:
  ```yaml
  command: redis-server --requirepass STRONG_REDIS_PASSWORD --maxmemory 256mb --maxmemory-policy allkeys-lru
  ```
  Update `REDIS_URL` to include the password: `redis://:PASSWORD@redis:6379/0`

- [ ] Disable Redis `CONFIG` command if not needed (`rename-command CONFIG ""`).

---

## Network

- [ ] **CRITICAL** — Use a firewall to expose only ports 80 and 443 externally.
  All other ports (5432, 6379, 4222, 7422, 8080, 3001, 9090, 3000, 16686) must be
  blocked from external access.

  Example using ufw:
  ```bash
  ufw default deny incoming
  ufw allow 22/tcp    # SSH
  ufw allow 80/tcp    # HTTP (redirect to HTTPS)
  ufw allow 443/tcp   # HTTPS
  ufw enable
  ```

- [ ] Enable NATS TLS for leafnode connections (port 7422) and client connections (port 4222).
  Configure in `cmdb-core/deploy/nats/nats-central.conf`. See NATS docs for TLS block syntax.

- [ ] For Edge deployments: restrict Edge nodes to only outbound TCP to Central on port 7422.
  Inbound connections from Edge to Central API should not be required.

- [ ] Remove or restrict the NATS monitoring port (8222) from external access.
  It exposes server statistics and connection details without authentication by default.

- [ ] Remove Jaeger (16686), Prometheus (9090), Grafana (3000), and Loki (3100) port bindings
  from public interfaces. These dashboards should be accessed via SSH tunnel or VPN only.

---

## Application

- [ ] **CRITICAL** — Set `DEPLOY_MODE=cloud` in production. This mode enforces all credential
  validation checks at startup. The application will not start with insecure defaults.

- [ ] Set `MCP_API_KEY` to a strong random key if `MCP_ENABLED=true`.
  Without this, the MCP server (port 3001) accepts unauthenticated connections.
  Generate with: `openssl rand -hex 24`

- [ ] Verify RBAC is enabled and all API endpoints require authentication.
  Test with an unauthenticated request:
  ```bash
  curl -s http://localhost:8080/api/v1/assets
  # Expected: 401 Unauthorized
  ```

- [ ] Review which Edge nodes are authorized to sync and which `TENANT_ID` values are registered.
  Unauthorized Edge nodes should not receive snapshot data.

- [ ] Enable rate limiting on authentication endpoints to prevent brute-force attacks.
  Verify the configured rate limit middleware is active in the router.

- [ ] Ensure error responses do not leak stack traces, internal paths, or SQL errors.
  In production, all errors should return generic messages; details go to logs only.

- [ ] Set `LOG_LEVEL=info` (not `debug`) in production to avoid logging sensitive request data.

---

## Docker

- [ ] Run containers as non-root users. Add `user` directives to service definitions:
  ```yaml
  services:
    cmdb-core:
      user: "1000:1000"
  ```
  Verify the application image supports non-root execution.

- [ ] Use read-only file systems for stateless containers where possible:
  ```yaml
  services:
    cmdb-core:
      read_only: true
      tmpfs:
        - /tmp
  ```

- [ ] **CRITICAL** — Pin all image versions. Never use `:latest` in production.
  Current pinned versions in `docker-compose.yml`:
  - `timescale/timescaledb:latest-pg17` → pin to a specific digest or tag
  - `redis:7.4-alpine`
  - `nats:2.10-alpine`
  - `nginx:1.27-alpine`
  - `otel/opentelemetry-collector-contrib:0.115.0`
  - `jaegertracing/all-in-one:1.64`
  - `prom/prometheus:v3.1.0`
  - `grafana/loki:3.3.2`
  - `grafana/grafana:11.4.0`

- [ ] Scan all images for known CVEs before deploying to production:
  ```bash
  docker scout cves timescale/timescaledb:latest-pg17
  docker scout cves redis:7.4-alpine
  # Repeat for each image
  ```

- [ ] Limit container capabilities using `cap_drop` and `cap_add`:
  ```yaml
  services:
    cmdb-core:
      cap_drop:
        - ALL
      cap_add:
        - NET_BIND_SERVICE   # Only if binding to port < 1024
  ```

- [ ] Set `no-new-privileges` security option to prevent privilege escalation:
  ```yaml
  services:
    cmdb-core:
      security_opt:
        - no-new-privileges:true
  ```

---

## Monitoring and Audit

- [ ] Confirm audit logging is active. The application logs all authentication events,
  RBAC decisions, and data mutations by default at `info` level.

- [ ] Set up alerting for:
  - Repeated failed login attempts (> 10 in 5 minutes from same IP)
  - Unexpected JWT validation failures
  - Sync conflicts above normal baseline
  - Any `ERROR` or `FATAL` log entries

- [ ] Monitor the `/metrics` endpoint via Prometheus for anomalies.
  Sudden spikes in request rate or error rate can indicate an attack.

- [ ] Review `sync_conflicts` table weekly for unexpected patterns:
  ```sql
  SELECT entity_type, COUNT(*) AS conflicts, MAX(created_at) AS last_seen
  FROM sync_conflicts
  WHERE created_at > NOW() - INTERVAL '7 days'
  GROUP BY entity_type
  ORDER BY conflicts DESC;
  ```

- [ ] Enable Grafana authentication (the default `admin/admin` must be changed via `GRAFANA_PASS`).
  Consider enabling Grafana OAuth or LDAP integration for team access.

---

## Secrets Management

- [ ] **CRITICAL** — Never commit `.env` files to version control.
  Confirm `.env` is listed in `.gitignore`.

- [ ] Use Docker Secrets or a secrets manager (HashiCorp Vault, AWS Secrets Manager) for
  credentials in production rather than plain `.env` files:
  ```yaml
  services:
    cmdb-core:
      secrets:
        - jwt_secret
        - db_password
  secrets:
    jwt_secret:
      external: true
    db_password:
      external: true
  ```

- [ ] Rotate `JWT_SECRET` on a schedule (quarterly minimum). After rotation, all active
  sessions will be invalidated — plan for user re-authentication.

- [ ] Rotate database passwords on a schedule (quarterly minimum). Update `DATABASE_URL`
  and `DB_PASS` simultaneously, then restart affected services.

- [ ] Rotate `MCP_API_KEY` if it may have been exposed in logs or shared externally.

- [ ] Audit who has access to the host filesystem where `.env` is stored.
  File permissions should restrict read access to the deployment user only:
  ```bash
  chmod 600 .env
  ```

---

## Sign-Off

| Check | Completed By | Date |
|-------|-------------|------|
| Authentication hardening | | |
| Database hardening | | |
| Network firewall rules | | |
| Application configuration | | |
| Docker security options | | |
| Monitoring and alerting | | |
| Secrets management | | |
