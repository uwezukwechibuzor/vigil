"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { apiFetch, getMe, type RunSummary } from "../../lib/api";

export default function RunsPage() {
  const [runs, setRuns] = useState<RunSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const me = await getMe();
        const params = me.project_id ? `?project_id=${me.project_id}&limit=50` : "?limit=50";
        const data = await apiFetch<{ data: RunSummary[] }>(`/v1/runs${params}`);
        setRuns(data.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Agent Runs</h1>
      <p className="text-slate-500">End-to-end traces from LLM reasoning to onchain result.</p>

      {loading && <p className="text-sm text-slate-500">Loading runs…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      {!loading && !error && runs.length === 0 && (
        <div className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
          No runs yet. Send traces via the SDK or OTLP endpoint.
        </div>
      )}

      {!loading && !error && runs.length > 0 && (
        <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
          <table className="w-full text-sm">
            <thead className="bg-slate-50 dark:bg-slate-800">
              <tr>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Run ID</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Agent</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Spans</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Cost (USD)</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Tx</th>
                <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Time</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
              {runs.map((r) => (
                <tr key={r.run_id} className="hover:bg-slate-50 dark:hover:bg-slate-800">
                  <td className="px-4 py-3">
                    <Link href={`/runs/${r.run_id}`} className="font-mono text-xs text-indigo-600 hover:underline">
                      {r.run_id.slice(0, 16)}…
                    </Link>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs">{r.agent_id.slice(0, 16)}…</td>
                  <td className="px-4 py-3">{r.span_count}</td>
                  <td className="px-4 py-3">{r.total_cost_usd.toFixed(4)}</td>
                  <td className="px-4 py-3 font-mono text-xs">
                    {r.tx_hash ? `${r.tx_hash.slice(0, 10)}…` : "—"}
                  </td>
                  <td className="px-4 py-3 text-slate-500">
                    {new Date(r.created_at).toLocaleString()}
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
