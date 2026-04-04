import { memo, useState, useMemo } from "react";
import { useTranslation } from "react-i18next";
import { useUsers, useRoles } from "../hooks/useIdentity";

/* ──────────────────────────────────────────────
   Types & constants
   ────────────────────────────────────────────── */

interface RoleDisplay {
  id: string;
  name: string;
  scope: string;
  users?: number;
  warning?: boolean;
  icon: string;
}

const ROLE_ICONS: Record<string, string> = {
  admin: "admin_panel_settings",
  operator: "engineering",
  auditor: "account_balance",
  viewer: "person_outline",
};

interface PermissionRow {
  key: string;
  label: string;
  icon: string;
  view: boolean;
  edit: boolean;
  delete: boolean;
  export: boolean;
}

const PERM_SCOPES = [
  { key: "asset", label: "Asset Management", icon: "dns" },
  { key: "stock", label: "Stock & Inventory", icon: "inventory_2" },
  { key: "monitor", label: "System Monitoring", icon: "monitor_heart" },
  { key: "config", label: "System Configuration", icon: "settings" },
  { key: "compliance", label: "Compliance Audit", icon: "verified_user" },
];

const PERM_COLS = ["View", "Edit", "Delete", "Export"] as const;

/* ──────────────────────────────────────────────
   Small reusable pieces
   ────────────────────────────────────────────── */

function Icon({ name, className = "" }: { name: string; className?: string }) {
  return (
    <span className={`material-symbols-outlined ${className}`}>{name}</span>
  );
}

function Toggle({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: () => void;
  label: string;
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={onChange}
      className={`relative h-6 w-11 shrink-0 rounded-full transition-colors ${
        checked ? "bg-primary" : "bg-surface-container-highest"
      }`}
    >
      <span
        className={`absolute top-0.5 left-0.5 h-5 w-5 rounded-full transition-transform ${
          checked
            ? "translate-x-5 bg-[#001b34]"
            : "translate-x-0 bg-on-surface-variant"
        }`}
      />
    </button>
  );
}

/* ──────────────────────────────────────────────
   Main Component
   ────────────────────────────────────────────── */

function RolesPermissions() {
  const { t } = useTranslation();
  const { data: usersResponse, isLoading: usersLoading } = useUsers();
  const { data: rolesResponse, isLoading: rolesLoading } = useRoles();
  const apiUsers = usersResponse?.data ?? [];
  const apiRoles = rolesResponse?.data ?? [];

  // Map API roles to display shape
  const ROLES: RoleDisplay[] = useMemo(() => {
    if (apiRoles.length === 0) {
      return [
        { id: "sysadmin", name: "System Administrator", scope: "GLOBAL ACCESS", users: 2, icon: "admin_panel_settings" },
        { id: "operator", name: "IT Operator", scope: "CLUSTER DJ-WEST-1", users: 5, icon: "engineering" },
        { id: "auditor", name: "Finance Auditor", scope: "BILLING ENTRIES ONLY", icon: "account_balance" },
        { id: "guest", name: "Read-Only Guest", scope: "TEMPORARY ACCESS", warning: true, icon: "person_outline" },
      ];
    }
    return apiRoles.map((r) => ({
      id: r.id,
      name: r.name,
      scope: r.description || 'CUSTOM SCOPE',
      icon: ROLE_ICONS[r.name?.toLowerCase()] ?? "shield_person",
      users: apiUsers.filter((u) => u.status === 'active').length, // approximate
    }));
  }, [apiRoles, apiUsers]);

  const [selectedRole, setSelectedRole] = useState<string>('');
  // Auto-select first role
  const effectiveSelectedRole = selectedRole || ROLES[0]?.id || '';

  // Build permissions from API role.permissions or fall back to defaults
  const buildPermsForRole = (roleId: string): PermissionRow[] => {
    const apiRole = apiRoles.find((r) => r.id === roleId);
    if (apiRole?.permissions && Object.keys(apiRole.permissions).length > 0) {
      return PERM_SCOPES.map((scope) => {
        const perms = apiRole.permissions[scope.key] ?? [];
        return {
          key: scope.key,
          label: scope.label,
          icon: scope.icon,
          view: perms.includes('read') || perms.includes('view') || perms.includes('*'),
          edit: perms.includes('write') || perms.includes('edit') || perms.includes('*'),
          delete: perms.includes('delete') || perms.includes('*'),
          export: perms.includes('export') || perms.includes('*'),
        };
      });
    }
    // Fallback: all view enabled
    return PERM_SCOPES.map((scope) => ({
      key: scope.key,
      label: scope.label,
      icon: scope.icon,
      view: true,
      edit: false,
      delete: false,
      export: false,
    }));
  };

  const [permOverrides, setPermOverrides] = useState<Record<string, PermissionRow[]>>({});
  const activePerms = permOverrides[effectiveSelectedRole] ?? buildPermsForRole(effectiveSelectedRole);
  const activeRole = ROLES.find((r) => r.id === effectiveSelectedRole) ?? ROLES[0];

  function togglePerm(key: string, col: "view" | "edit" | "delete" | "export") {
    const currentPerms = permOverrides[effectiveSelectedRole] ?? buildPermsForRole(effectiveSelectedRole);
    setPermOverrides((prev) => ({
      ...prev,
      [effectiveSelectedRole]: currentPerms.map((row) =>
        row.key === key ? { ...row, [col]: !row[col] } : row,
      ),
    }));
  }

  const isLoading = usersLoading || rolesLoading;

  if (isLoading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="animate-spin rounded-full h-8 w-8 border-2 border-primary border-t-transparent" />
      </div>
    );
  }

  return (
    <div className="min-h-screen space-y-6 bg-surface px-6 py-5">
      {/* ── Header Row ── */}
      <div className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <h1 className="font-headline text-2xl font-bold text-on-surface">
            {t('roles.title')}
          </h1>
          <p className="mt-1 text-xs uppercase tracking-widest text-on-surface-variant">
            {t('roles.subtitle')}
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            type="button"
            className="flex items-center gap-2 rounded-lg bg-error-container px-5 py-2.5 text-xs font-bold uppercase tracking-wider text-error transition-colors hover:brightness-125"
          >
            <Icon name="emergency" className="text-base" />
            {t('roles.emergency_stop')}
          </button>
          <button
            type="button"
            className="machined-gradient flex items-center gap-2 rounded-lg px-5 py-2.5 text-xs font-bold uppercase tracking-wider text-[#001b34] transition-all hover:brightness-110"
          >
            <Icon name="rocket_launch" className="text-base" />
            {t('common.deploy_changes')}
          </button>
        </div>
      </div>

      {/* ── Main Content: Roles List + Permissions Matrix ── */}
      <div className="grid grid-cols-1 gap-4 lg:grid-cols-[320px_1fr]">
        {/* Left Panel: Role List */}
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <h2 className="text-[11px] font-semibold uppercase tracking-wider text-on-surface-variant">
              {t('roles.defined_roles')}
            </h2>
            <button
              type="button"
              className="flex items-center gap-1.5 rounded-md bg-surface-container-high px-3 py-1.5 text-[11px] font-semibold uppercase tracking-wider text-primary transition-colors hover:bg-surface-container-highest"
            >
              <Icon name="add" className="text-sm" />
              {t('roles.add_new_role')}
            </button>
          </div>

          <div className="space-y-2">
            {ROLES.map((role) => (
              <button
                key={role.id}
                type="button"
                onClick={() => setSelectedRole(role.id)}
                className={`flex w-full items-start gap-3 rounded-lg p-4 text-left transition-colors ${
                  effectiveSelectedRole === role.id
                    ? "bg-surface-container-highest"
                    : "bg-surface-container hover:bg-surface-container-high"
                }`}
              >
                <div
                  className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-lg ${
                    effectiveSelectedRole === role.id
                      ? "bg-primary/15"
                      : "bg-surface-container-low"
                  }`}
                >
                  <Icon
                    name={role.icon}
                    className={`text-xl ${
                      effectiveSelectedRole === role.id ? "text-primary" : "text-on-surface-variant"
                    }`}
                  />
                </div>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-semibold text-on-surface">
                    {role.name}
                  </p>
                  <p className="mt-0.5 text-[10px] uppercase tracking-wider text-on-surface-variant">
                    {role.scope}
                  </p>
                  <div className="mt-1.5 flex items-center gap-2">
                    {role.users !== undefined && (
                      <span className="text-[10px] text-on-surface-variant">
                        {role.users} {role.users !== 1 ? t('roles.users') : t('roles.user')}
                      </span>
                    )}
                    {role.warning && (
                      <span className="inline-flex items-center gap-1 rounded bg-error/15 px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wide text-error">
                        <Icon name="warning" className="text-xs" />
                        {t('roles.expiring')}
                      </span>
                    )}
                  </div>
                </div>
                {effectiveSelectedRole === role.id && (
                  <Icon name="chevron_right" className="mt-2 text-lg text-primary" />
                )}
              </button>
            ))}
          </div>
        </div>

        {/* Right Panel: Permissions Matrix */}
        <div className="rounded-lg bg-surface-container p-5">
          {/* Matrix Header */}
          <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="font-headline text-lg font-bold text-on-surface">
                {t('roles.permissions_matrix')}
              </h2>
              <p className="mt-0.5 text-[11px] uppercase tracking-wider text-on-surface-variant">
                {t('roles.configuring')}: <span className="font-bold text-primary">{activeRole.name}</span>
              </p>
            </div>
            <div className="flex items-center gap-2">
              <button
                type="button"
                className="rounded-md bg-surface-container-high px-4 py-2 text-xs font-semibold uppercase tracking-wider text-on-surface-variant transition-colors hover:text-on-surface"
              >
                {t('common.cancel')}
              </button>
              <button
                type="button"
                className="rounded-md bg-primary px-4 py-2 text-xs font-bold uppercase tracking-wider text-[#001b34] transition-colors hover:brightness-110"
              >
                {t('common.save_changes')}
              </button>
            </div>
          </div>

          {/* Matrix Table */}
          <div className="overflow-x-auto">
            {/* Column Headers */}
            <div className="grid grid-cols-[1fr_80px_80px_80px_80px] items-center gap-px rounded-t-lg bg-surface-container-high px-4 py-3">
              <span className="text-[11px] font-semibold uppercase tracking-wider text-on-surface-variant">
                {t('roles.permission_scope')}
              </span>
              {PERM_COLS.map((col) => (
                <span
                  key={col}
                  className="text-center text-[11px] font-semibold uppercase tracking-wider text-on-surface-variant"
                >
                  {col}
                </span>
              ))}
            </div>

            {/* Permission Rows */}
            {activePerms.map((row, idx) => (
              <div
                key={row.key}
                className={`grid grid-cols-[1fr_80px_80px_80px_80px] items-center gap-px px-4 py-3.5 ${
                  idx % 2 === 0 ? "bg-surface-container-low" : "bg-surface-container"
                }`}
              >
                <div className="flex items-center gap-3">
                  <Icon name={row.icon} className="text-lg text-on-surface-variant" />
                  <span className="text-sm font-medium text-on-surface">{row.label}</span>
                </div>
                {(["view", "edit", "delete", "export"] as const).map((col) => (
                  <div key={col} className="flex justify-center">
                    <Toggle
                      checked={row[col]}
                      onChange={() => togglePerm(row.key, col)}
                      label={`${col} ${row.label}`}
                    />
                  </div>
                ))}
              </div>
            ))}
          </div>

          {/* Bottom Info Bar */}
          <div className="mt-5 grid grid-cols-1 gap-3 sm:grid-cols-3">
            <div className="rounded-lg bg-surface-container-low p-4">
              <span className="text-[10px] uppercase tracking-wider text-on-surface-variant">
                {t('roles.effective_scope')}
              </span>
              <p className="mt-1 flex items-center gap-2 text-sm font-semibold text-on-surface">
                <Icon name="public" className="text-base text-primary" />
                {t('roles.global_cluster')}
              </p>
            </div>
            <div className="rounded-lg bg-surface-container-low p-4">
              <span className="text-[10px] uppercase tracking-wider text-on-surface-variant">
                {t('roles.security_level')}
              </span>
              <p className="mt-1 flex items-center gap-2 text-sm font-semibold text-error">
                <Icon name="lock" className="text-base" />
                L5 CRITICAL
              </p>
            </div>
            <div className="rounded-lg bg-surface-container-low p-4">
              <span className="text-[10px] uppercase tracking-wider text-on-surface-variant">
                {t('roles.last_audit')}
              </span>
              <p className="mt-1 flex items-center gap-2 text-sm font-semibold text-on-surface">
                <Icon name="schedule" className="text-base text-on-surface-variant" />
                2023-10-18 09:14:22 UTC
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}

export default memo(RolesPermissions);
