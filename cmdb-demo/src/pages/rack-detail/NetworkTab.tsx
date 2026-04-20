import { useTranslation } from "react-i18next";
import { useDeleteNetworkConnection } from "../../hooks/useTopology";
import type { NetworkConnection } from "../../lib/api/topology";

/** NetworkConnection as returned by the API — includes legacy field aliases */
export type NetworkConnectionExt = NetworkConnection & {
  port?: string;
  device?: string;
  vlan?: number | string;
};

export function NetworkTab({ networkConnections, rackId, onAddConnection }: { networkConnections: NetworkConnectionExt[]; rackId: string; onAddConnection: () => void }) {
  const { t } = useTranslation();
  const deleteConn = useDeleteNetworkConnection();

  function handleDelete(connId: string) {
    if (confirm(t("rack_detail.confirm_delete_connection"))) {
      deleteConn.mutate({ rackId, connId });
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-2">
          <span className="material-symbols-outlined text-primary">lan</span>
          <h2 className="font-headline font-bold text-sm tracking-widest uppercase text-on-surface">
            {t("rack_detail.network_connectivity")}
          </h2>
        </div>
        <button
          onClick={onAddConnection}
          className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary/20 text-primary text-sm font-medium hover:bg-primary/30 transition-colors"
        >
          <span className="material-symbols-outlined text-[18px]">add</span>
          {t("rack_detail.btn_add_connection")}
        </button>
      </div>
      <div className="bg-surface-container rounded overflow-hidden">
        <table className="w-full text-sm">
          <thead>
            <tr className="bg-surface-container-high text-on-surface-variant text-[11px] uppercase tracking-widest">
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_port")}</th>
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_connected_device")}</th>
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_speed")}</th>
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_status")}</th>
              <th className="text-left px-5 py-3 font-medium">{t("rack_detail.table_vlan")}</th>
              <th className="px-5 py-3" />
            </tr>
          </thead>
          <tbody>
            {networkConnections.map((conn) => (
              <tr key={conn.id ?? conn.source_port ?? conn.port} className="bg-surface-container hover:bg-surface-container-high transition-colors">
                <td className="px-5 py-3 font-label font-semibold text-on-surface">{conn.source_port ?? conn.port}</td>
                <td className="px-5 py-3 text-on-surface-variant">
                  <div className="flex items-center gap-1.5">
                    <span className="material-symbols-outlined text-[16px]">router</span>
                    {conn.external_device ?? conn.connected_asset_id ?? conn.device}
                  </div>
                </td>
                <td className="px-5 py-3 text-on-surface-variant">{conn.speed}</td>
                <td className="px-5 py-3">
                  <span
                    className={`inline-flex items-center gap-1 px-2.5 py-0.5 rounded text-[11px] font-semibold tracking-wider ${
                      conn.status === "UP"
                        ? "bg-on-primary-container/20 text-primary"
                        : "bg-error-container/40 text-error"
                    }`}
                  >
                    <span className={`w-1.5 h-1.5 rounded-full ${conn.status === "UP" ? "bg-primary" : "bg-error"}`} />
                    {conn.status}
                  </span>
                </td>
                <td className="px-5 py-3 text-on-surface-variant font-mono text-xs">
                  {Array.isArray(conn.vlans) ? conn.vlans.join(', ') : (conn.vlan ?? '')}
                </td>
                <td className="px-3 py-3">
                  <button
                    onClick={() => handleDelete(conn.id)}
                    disabled={deleteConn.isPending}
                    className="p-1 rounded text-on-surface-variant hover:text-error hover:bg-error-container/20 transition-colors disabled:opacity-50"
                    title={t("rack_detail.confirm_delete_connection")}
                  >
                    <span className="material-symbols-outlined text-[18px]">delete</span>
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
