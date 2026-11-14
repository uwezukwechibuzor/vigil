"use client";

import { useEffect, useState } from "react";
import { apiFetch, getMe, type UsageCounter } from "../../lib/api";

export default function UsagePage() {
  const [counters, setCounters] = useState<UsageCounter[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const me = await getMe();
        const to = new Date();
        const from = new Date();
        from.setDate(from.getDate() - 30);
        const fmt = (d: Date) => d.toISOString().split("T")[0];
        const data = await apiFetch<{ data: UsageCounter[] }>(
          `/v1/usage?org_id=${me.org_id}&from=${fmt(from)}&to=${fmt(to)}`
        );
        setCounters(data.data);
      } catch (e) {
        setError(e instanceof Error ? e.message : "Failed to load");
      } finally {
        setLoading(false);
      }
    })();
  }, []);

  const totals = counters.reduce(
    (acc, c) => ({
      traces: acc.traces + c.traces,
      flight_checks: acc.flight_checks + c.flight_checks,
      api_calls: acc.api_calls + c.api_calls,
    }),
    { traces: 0, flight_checks: 0, api_calls: 0 }
  );

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-semibold">Usage &amp; Billing</h1>
      <p className="text-slate-500">Track API calls, flight checks, and spans ingested — last 30 days.</p>

      {loading && <p className="text-sm text-slate-500">Loading usage…</p>}
      {error && <p className="text-sm text-red-500">{error}</p>}

      {!loading && !error && (
        <>
          <div className="grid grid-cols-3 gap-4">
            {[
              { label: "Traces", value: totals.traces },
              { label: "Flight checks", value: totals.flight_checks },
              { label: "API calls", value: totals.api_calls },
            ].map((s) => (
              <div key={s.label} className="rounded-lg border border-slate-200 p-4 dark:border-slate-700">
                <p className="text-sm text-slate-500">{s.label}</p>
                <p className="mt-1 text-2xl font-semibold">{s.value.toLocaleString()}</p>
              </div>
            ))}
          </div>

          {counters.length === 0 ? (
            <div className="rounded-lg border border-slate-200 p-8 text-center text-slate-400 dark:border-slate-700">
              No usage data yet.
            </div>
          ) : (
            <div className="rounded-lg border border-slate-200 dark:border-slate-700 overflow-hidden">
              <table className="w-full text-sm">
                <thead className="bg-slate-50 dark:bg-slate-800">
                  <tr>
                    <th className="px-4 py-3 text-left font-medium text-slate-600 dark:text-slate-300">Day</th>
                    <th className="px-4 py-3 text-right font-medium text-slate-600 dark:text-slate-300">Traces</th>
                    <th className="px-4 py-3 text-right font-medium text-slate-600 dark:text-slate-300">Flight checks</th>
                    <th className="px-4 py-3 text-right font-medium text-slate-600 dark:text-slate-300">API calls</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-200 dark:divide-slate-700">
                  {counters.map((c) => (
                    <tr key={c.day} className="hover:bg-slate-50 dark:hover:bg-slate-800">
                      <td className="px-4 py-3">{c.day}</td>
                      <td className="px-4 py-3 text-right">{c.traces}</td>
                      <td className="px-4 py-3 text-right">{c.flight_checks}</td>
                      <td className="px-4 py-3 text-right">{c.api_calls}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  );
}
