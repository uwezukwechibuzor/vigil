"use client";

import { useEffect, useState } from "react";
import { Badge } from "../../components/ui/badge";
import { apiFetch, getMe, type Incident } from "../../lib/api";

const severityVariant = (s: string): "danger" | "warning" | "success" =>
  s === "critical" ? "danger" : s === "high" ? "warning" : "success";

export default function IncidentsPage() {
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const me = await getMe();
        const params = me.project_id ? `?project_id=${me.project_id}` : "";
        const data = await apiFetch<{ data: Incident[] }>(`/v1/incidents${params}`);
        setIncidents(data.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Incidents</h1>
      <p className="text-slate-500">Correlated anomalies with AI root-cause summaries.</p>

      {loading && <p className="text-sm text-slate-500">Loading incidents…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      {!loading && !error && incidents.length === 0 && (
        <div className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
          No incidents detected.
        </div>
      )}

      {!loading && !error && incidents.length > 0 && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-800">
              <tr>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Title</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Severity</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Status</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Started</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Resolved</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
              {incidents.map((i) => (
                <tr key={i.id} className="hover:bg-slate-50 dark:hover:bg-slate-800">
                  <td className="px-4 py-3 font-medium">{i.title}</td>
                  <td className="px-4 py-3">
                    <Badge variant={severityVariant(i.severity)}>{i.severity}</Badge>
                  </td>
                  <td className="px-4 py-3">
                    <Badge variant={i.status === "resolved" ? "success" : i.status === "acknowledged" ? "warning" : "danger"}>
                      {i.status}
                    </Badge>
                  </td>
                  <td className="px-4 py-3 text-slate-500">{new Date(i.started_at).toLocaleString()}</td>
                  <td className="px-4 py-3 text-slate-500">
                    {i.resolved_at ? new Date(i.resolved_at).toLocaleString() : "—"}
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
