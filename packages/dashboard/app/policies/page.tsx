"use client";

import { useEffect, useState } from "react";
import { Badge } from "../../components/ui/badge";
import { apiFetch, getMe, type Policy } from "../../lib/api";

export default function PoliciesPage() {
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const me = await getMe();
        if (!me.project_id) {
          setLoading(false);
          return;
        }
        const data = await apiFetch<{ data: Policy[] }>(`/v1/policies?project_id=${me.project_id}`);
        setPolicies(data.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Policies</h1>
      <p className="text-slate-500">Configure flight-check policies for your agents.</p>

      {loading && <p className="text-sm text-slate-500">Loading policies…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      {!loading && !error && policies.length === 0 && (
        <div className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
          No policies defined. Create one via the API.
        </div>
      )}

      {!loading && !error && policies.length > 0 && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-800">
              <tr>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Name</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Version</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Status</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
              {policies.map((p) => (
                <tr key={p.id} className="hover:bg-slate-50 dark:hover:bg-slate-800">
                  <td className="px-4 py-3 font-medium">{p.name}</td>
                  <td className="px-4 py-3">v{p.version}</td>
                  <td className="px-4 py-3">
                    <Badge variant={p.enabled ? "success" : "warning"}>
                      {p.enabled ? "active" : "disabled"}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-slate-500">{new Date(p.created_at).toLocaleDateString()}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
