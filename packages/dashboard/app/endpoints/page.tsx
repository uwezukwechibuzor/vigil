"use client";

import { useEffect, useState } from "react";
import { Badge } from "../../components/ui/badge";
import { apiFetch, getMe, type Endpoint } from "../../lib/api";

export default function EndpointsPage() {
  const [endpoints, setEndpoints] = useState<Endpoint[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const me = await getMe();
        const params = me.project_id ? `?project_id=${me.project_id}` : "";
        const data = await apiFetch<{ data: Endpoint[] }>(`/v1/endpoints${params}`);
        setEndpoints(data.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Endpoints</h1>
      <p className="text-slate-500">Monitor RPC, indexer and oracle endpoints for your projects.</p>

      {loading && <p className="text-sm text-slate-500">Loading endpoints…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      {!loading && !error && endpoints.length === 0 && (
        <div className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
          No endpoints configured. Add one via the API or SDK.
        </div>
      )}

      {!loading && !error && endpoints.length > 0 && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-800">
              <tr>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Name</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Kind</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">URL</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
              {endpoints.map((ep) => (
                <tr key={ep.id} className="hover:bg-slate-50 dark:hover:bg-slate-800">
                  <td className="px-4 py-3 font-medium">{ep.name}</td>
                  <td className="px-4 py-3 font-mono text-xs">{ep.kind}</td>
                  <td className="px-4 py-3 font-mono text-xs truncate max-w-xs" title={ep.url}>
                    {ep.url.replace(/\/v2\/[^/]+/, "/v2/***")}
                  </td>
                  <td className="px-4 py-3">
                    <Badge variant={ep.enabled ? "success" : "warning"}>
                      {ep.enabled ? "enabled" : "disabled"}
                    </Badge>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
