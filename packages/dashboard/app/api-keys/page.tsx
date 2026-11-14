"use client";

import { useEffect, useState } from "react";
import { Badge } from "../../components/ui/badge";
import { apiFetch, getMe, type APIKey } from "../../lib/api";

export default function ApiKeysPage() {
  const [keys, setKeys] = useState<APIKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const me = await getMe();
        const data = await apiFetch<{ data: APIKey[] }>(`/v1/api-keys?org_id=${me.org_id}`);
        setKeys(data.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">API Keys</h1>
      <p className="text-slate-500">Manage API keys for SDK and direct API access.</p>

      {loading && <p className="text-sm text-slate-500">Loading keys…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      {!loading && !error && keys.length === 0 && (
        <div className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
          No API keys yet.
        </div>
      )}

      {!loading && !error && keys.length > 0 && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-800">
              <tr>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Name</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Prefix</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Scopes</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Last used</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Status</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
              {keys.map((k) => (
                <tr key={k.id} className="hover:bg-slate-50 dark:hover:bg-slate-800">
                  <td className="px-4 py-3 font-medium">{k.name}</td>
                  <td className="px-4 py-3 font-mono text-xs">{k.key_prefix}…</td>
                  <td className="px-4 py-3 font-mono text-xs">{k.scopes.join(", ")}</td>
                  <td className="px-4 py-3 text-slate-500">
                    {k.last_used_at ? new Date(k.last_used_at).toLocaleDateString() : "never"}
                  </td>
                  <td className="px-4 py-3">
                    <Badge variant={k.revoked_at ? "danger" : "success"}>
                      {k.revoked_at ? "revoked" : "active"}
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
